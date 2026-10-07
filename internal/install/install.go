// Package install jalanin script install egg di container sementara,
// mirip proses install di Pterodactyl Wings.
package install

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	Idle      = "idle"
	Running   = "running"
	Completed = "completed"
	Failed    = "failed"

	maxScript      = 1 << 20
	maxTail        = 16 << 10
	maxLogResponse = 4 << 10
	defaultTimeout = 30 * time.Minute
	tmpPrefix      = ".install-"
	stateFile      = ".installstate.json"

	interruptedMsg = "daemon sempat mati atau restart saat install berjalan; hasil install nggak pasti. Cek isi server, lalu Reinstall kalau perlu."

	// Pakai bash kalau ada (image installer Debian), kalau nggak sh (Alpine/ash).
	entrypoint = `if [ -x /bin/bash ]; then exec /bin/bash /mnt/install/install.sh; else exec /bin/sh /mnt/install/install.sh; fi`
)

var (
	ErrBusy    = errors.New("install sedang berjalan untuk server ini")
	ErrInvalid = errors.New("permintaan install nggak valid")

	idRe     = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
	imageRe  = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._/:@-]{0,254}$`)
	envKeyRe = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]{0,127}$`)
)

// Spec = permintaan install dari Panel.
type Spec struct {
	Script string
	Image  string
	Env    map[string]string
}

// Job = satu eksekusi install yang diserahkan ke Runner.
type Job struct {
	Name    string
	Image   string
	DataDir string
	Script  string
	Env     map[string]string
}

// Runner jalanin job dan balikin ekor output. Dibikin tipe sendiri biar bisa
// diganti di test tanpa Docker.
type Runner func(ctx context.Context, job Job) (string, error)

// State = status install terakhir satu server (disimpan di memori).
type State struct {
	Status   string `json:"status"`
	Error    string `json:"error,omitempty"`
	Log      string `json:"log,omitempty"`
	ExitCode *int   `json:"exit_code,omitempty"`
}

// Installer ngatur install yang jalan di background per server.
type Installer struct {
	dataRoot string
	run      Runner
	timeout  time.Duration

	statePath string
	kill      func(name string)

	mu    sync.Mutex
	state map[string]State
}

// New bikin Installer yang pakai Docker.
func New(dataRoot string) *Installer {
	i := NewWithRunner(dataRoot, DockerRunner(dataRoot))
	i.kill = func(name string) { _ = exec.Command("docker", "rm", "-f", name).Run() }
	return i
}

// NewWithRunner sama kayak New tapi runner-nya bisa diganti (buat test).
func NewWithRunner(dataRoot string, run Runner) *Installer {
	return &Installer{
		dataRoot:  dataRoot,
		run:       run,
		timeout:   defaultTimeout,
		state:     map[string]State{},
		statePath: filepath.Join(dataRoot, stateFile),
		kill:      func(string) {},
	}
}

// Recover bersihin folder sementara sisa install yang terputus, lalu muat status
// install dari disk. Install yang masih "running" di file berarti daemon mati
// di tengah jalan: ditandai failed dan container install-nya dimatikan.
func (i *Installer) Recover() {
	left, _ := filepath.Glob(filepath.Join(i.dataRoot, tmpPrefix+"*"))
	for _, p := range left {
		_ = os.RemoveAll(p)
	}
	i.loadState()
}

func (i *Installer) loadState() {
	data, err := os.ReadFile(i.statePath)
	if err != nil {
		return
	}
	var loaded map[string]State
	if err := json.Unmarshal(data, &loaded); err != nil {
		log.Printf("status install rusak, diabaikan: %v", err)
		return
	}

	var killNames []string
	changed := false
	i.mu.Lock()
	for uuid, st := range loaded {
		if !idRe.MatchString(uuid) {
			continue
		}
		if st.Status == Running {
			killNames = append(killNames, "dockpanel-install-"+uuid)
			st = State{Status: Failed, Error: interruptedMsg, Log: st.Log}
			changed = true
		}
		i.state[uuid] = st
	}
	if changed {
		i.persistLocked()
	}
	i.mu.Unlock()

	for _, name := range killNames {
		i.kill(name)
	}
}

// persistLocked nulis status install ke disk (atomik). Pemanggil harus megang i.mu.
func (i *Installer) persistLocked() {
	data, err := json.Marshal(i.state)
	if err != nil {
		log.Printf("gagal simpan status install: %v", err)
		return
	}
	if err := os.MkdirAll(i.dataRoot, 0o755); err != nil {
		log.Printf("gagal simpan status install: %v", err)
		return
	}
	tmp := i.statePath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		log.Printf("gagal simpan status install: %v", err)
		return
	}
	if err := os.Rename(tmp, i.statePath); err != nil {
		log.Printf("gagal simpan status install: %v", err)
		_ = os.Remove(tmp)
	}
}

// State balikin status install terakhir server ini ("idle" kalau belum pernah).
func (i *Installer) State(uuid string) State {
	i.mu.Lock()
	defer i.mu.Unlock()
	if st, ok := i.state[uuid]; ok {
		return st
	}
	return State{Status: Idle}
}

func (i *Installer) IsInstalling(uuid string) bool {
	return i.State(uuid).Status == Running
}

// Start mulai install di background. Pemanggil wajib memastikan server mati.
func (i *Installer) Start(uuid string, spec Spec) error {
	if !idRe.MatchString(uuid) {
		return fmt.Errorf("%w: uuid", ErrInvalid)
	}
	if !imageRe.MatchString(spec.Image) {
		return fmt.Errorf("%w: image container", ErrInvalid)
	}
	script := strings.ReplaceAll(spec.Script, "\r\n", "\n")
	if strings.TrimSpace(script) == "" {
		return fmt.Errorf("%w: script kosong", ErrInvalid)
	}
	if len(script) > maxScript {
		return fmt.Errorf("%w: script kebesaran", ErrInvalid)
	}
	for k := range spec.Env {
		if !envKeyRe.MatchString(k) {
			return fmt.Errorf("%w: nama variable %q", ErrInvalid, k)
		}
	}

	dir := filepath.Join(i.dataRoot, uuid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}

	i.mu.Lock()
	if i.state[uuid].Status == Running {
		i.mu.Unlock()
		return ErrBusy
	}
	i.state[uuid] = State{Status: Running}
	i.persistLocked()
	i.mu.Unlock()

	job := Job{Name: "dockpanel-install-" + uuid, Image: spec.Image, DataDir: dir, Script: script, Env: spec.Env}
	go i.finish(uuid, job)
	return nil
}

func (i *Installer) finish(uuid string, job Job) {
	ctx, cancel := context.WithTimeout(context.Background(), i.timeout)
	defer cancel()

	out, err := i.run(ctx, job)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		err = fmt.Errorf("install melebihi batas waktu %s", i.timeout)
	}

	zero := 0
	st := State{Status: Completed, Log: i.clean(tailBytes(out, maxLogResponse)), ExitCode: &zero}
	if err != nil {
		st = State{
			Status:   Failed,
			Error:    i.clean(err.Error()),
			Log:      i.clean(tailBytes(out, maxLogResponse)),
			ExitCode: exitCode(err),
		}
		log.Printf("install server %s gagal: %v", uuid, err)
	} else {
		log.Printf("install server %s selesai", uuid)
	}

	i.mu.Lock()
	i.state[uuid] = st
	i.persistLocked()
	i.mu.Unlock()
}

// exitCode ambil exit code script dari error docker run (nil kalau gagalnya bukan
// karena script, mis. pull image gagal atau timeout).
func exitCode(err error) *int {
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() >= 0 {
		c := ee.ExitCode()
		return &c
	}
	return nil
}

// clean buang path host dari teks sebelum dikirim ke Panel.
func (i *Installer) clean(s string) string {
	return strings.ReplaceAll(s, i.dataRoot, "")
}

func tailBytes(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

type tailBuffer struct{ buf []byte }

func (t *tailBuffer) Write(p []byte) (int, error) {
	t.buf = append(t.buf, p...)
	if len(t.buf) > maxTail {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-maxTail:]...)
	}
	return len(p), nil
}

// DockerRunner jalanin script di container sementara: folder data server
// di-mount ke /mnt/server, script ke /mnt/install/install.sh
// (bisa ditulis, karena script egg Pterodactyl sering chown folder ini; foldernya sementara).
func DockerRunner(dataRoot string) Runner {
	return func(ctx context.Context, job Job) (string, error) {
		tmp, err := os.MkdirTemp(dataRoot, tmpPrefix)
		if err != nil {
			return "", err
		}
		defer os.RemoveAll(tmp)
		if err := os.Chmod(tmp, 0o755); err != nil {
			return "", err
		}
		if err := os.WriteFile(filepath.Join(tmp, "install.sh"), []byte(job.Script), 0o755); err != nil {
			return "", err
		}

		dataDir, err := filepath.Abs(job.DataDir)
		if err != nil {
			return "", err
		}
		scriptDir, err := filepath.Abs(tmp)
		if err != nil {
			return "", err
		}

		// Sisa container install dari daemon yang sempat mati; dan bersihin pas selesai
		// (matiin proses docker CLI nggak otomatis matiin container-nya).
		_ = exec.Command("docker", "rm", "-f", job.Name).Run()
		defer func() { _ = exec.Command("docker", "rm", "-f", job.Name).Run() }()

		if out, err := exec.CommandContext(ctx, "docker", "pull", job.Image).CombinedOutput(); err != nil {
			// Offline tapi image sudah ada lokal: lanjut aja.
			if exec.CommandContext(ctx, "docker", "image", "inspect", job.Image).Run() != nil {
				return tailBytes(string(out), maxTail), fmt.Errorf("pull image %s gagal: %w", job.Image, err)
			}
		}

		args := []string{
			"run", "--rm", "--name", job.Name,
			"-v", dataDir + ":/mnt/server",
			"-v", scriptDir + ":/mnt/install",
			"-w", "/mnt/server",
			"--security-opt=no-new-privileges",
			"--pids-limit=512",
		}
		keys := make([]string, 0, len(job.Env))
		for k := range job.Env {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			args = append(args, "-e", k+"="+job.Env[k])
		}
		args = append(args, "--entrypoint", "/bin/sh", job.Image, "-c", entrypoint)

		var tb tailBuffer
		cmd := exec.CommandContext(ctx, "docker", args...)
		cmd.Stdout = &tb
		cmd.Stderr = &tb
		if err := cmd.Run(); err != nil {
			return string(tb.buf), fmt.Errorf("script install gagal: %w", err)
		}
		return string(tb.buf), nil
	}
}
