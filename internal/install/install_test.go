package install

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func waitStatus(t *testing.T, i *Installer, uuid, want string) State {
	t.Helper()
	for n := 0; n < 300; n++ {
		if st := i.State(uuid); st.Status == want {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("status %s nggak jadi %q (sekarang %+v)", uuid, want, i.State(uuid))
	return State{}
}

func TestStartCompleted(t *testing.T) {
	root := t.TempDir()
	var got Job
	i := NewWithRunner(root, func(_ context.Context, job Job) (string, error) {
		got = job
		return "selesai", nil
	})

	if st := i.State("srv1"); st.Status != Idle {
		t.Fatalf("status awal = %q", st.Status)
	}
	err := i.Start("srv1", Spec{Script: "echo hi\r\nexit 0", Image: "alpine:3", Env: map[string]string{"A": "1"}})
	if err != nil {
		t.Fatal(err)
	}
	st := waitStatus(t, i, "srv1", Completed)
	if st.Log != "selesai" {
		t.Errorf("log = %q", st.Log)
	}
	if got.Script != "echo hi\nexit 0" {
		t.Errorf("script = %q", got.Script)
	}
	if got.Image != "alpine:3" || got.Env["A"] != "1" || got.Name != "dockpanel-install-srv1" {
		t.Errorf("job = %+v", got)
	}
	if got.DataDir != filepath.Join(root, "srv1") {
		t.Errorf("datadir = %q", got.DataDir)
	}
	if st, err := os.Stat(got.DataDir); err != nil || !st.IsDir() {
		t.Error("folder data server harusnya dibuat")
	}
}

func TestStartFailedCleansHostPath(t *testing.T) {
	root := t.TempDir()
	i := NewWithRunner(root, func(context.Context, Job) (string, error) {
		return "boom di " + root + "/x", errors.New("gagal baca " + root + "/srv1")
	})
	if err := i.Start("srv1", Spec{Script: "exit 1", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	st := waitStatus(t, i, "srv1", Failed)
	if strings.Contains(st.Error, root) || strings.Contains(st.Log, root) {
		t.Errorf("path host bocor: %+v", st)
	}
	if !strings.Contains(st.Error, "gagal baca") {
		t.Errorf("error = %q", st.Error)
	}
}

func TestStartBusyThenReusable(t *testing.T) {
	release := make(chan struct{})
	i := NewWithRunner(t.TempDir(), func(context.Context, Job) (string, error) {
		<-release
		return "", nil
	})
	spec := Spec{Script: "true", Image: "alpine"}
	if err := i.Start("srv1", spec); err != nil {
		t.Fatal(err)
	}
	if !i.IsInstalling("srv1") {
		t.Error("harusnya lagi installing")
	}
	if err := i.Start("srv1", spec); !errors.Is(err, ErrBusy) {
		t.Fatalf("install kedua harusnya ErrBusy, dapat %v", err)
	}
	if err := i.Start("srv2", spec); err != nil {
		t.Fatalf("server lain harusnya boleh: %v", err)
	}
	close(release)
	waitStatus(t, i, "srv1", Completed)
	waitStatus(t, i, "srv2", Completed)
	if err := i.Start("srv1", spec); err != nil {
		t.Fatalf("setelah selesai harusnya boleh lagi: %v", err)
	}
	waitStatus(t, i, "srv1", Completed)
}

func TestStartValidation(t *testing.T) {
	i := NewWithRunner(t.TempDir(), func(context.Context, Job) (string, error) { return "", nil })
	cases := map[string]struct {
		uuid string
		spec Spec
	}{
		"uuid":          {"../x", Spec{Script: "true", Image: "alpine"}},
		"image":         {"srv1", Spec{Script: "true", Image: "bad image;rm"}},
		"script kosong": {"srv1", Spec{Script: "  \n", Image: "alpine"}},
		"env key":       {"srv1", Spec{Script: "true", Image: "alpine", Env: map[string]string{"A=B": "x"}}},
		"script besar":  {"srv1", Spec{Script: strings.Repeat("a", maxScript+1), Image: "alpine"}},
	}
	for name, c := range cases {
		if err := i.Start(c.uuid, c.spec); !errors.Is(err, ErrInvalid) {
			t.Errorf("%s: harusnya ErrInvalid, dapat %v", name, err)
		}
	}
}

func TestTimeout(t *testing.T) {
	i := NewWithRunner(t.TempDir(), func(ctx context.Context, _ Job) (string, error) {
		<-ctx.Done()
		return "", ctx.Err()
	})
	i.timeout = 30 * time.Millisecond
	if err := i.Start("srv1", Spec{Script: "sleep 99", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	st := waitStatus(t, i, "srv1", Failed)
	if !strings.Contains(st.Error, "batas waktu") {
		t.Errorf("error = %q", st.Error)
	}
}

func TestRecoverRemovesLeftovers(t *testing.T) {
	root := t.TempDir()
	left := filepath.Join(root, tmpPrefix+"abc")
	keep := filepath.Join(root, "srv1")
	for _, p := range []string{left, keep} {
		if err := os.MkdirAll(p, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	NewWithRunner(root, nil).Recover()
	if _, err := os.Stat(left); !os.IsNotExist(err) {
		t.Error("sisa install harusnya dihapus")
	}
	if _, err := os.Stat(keep); err != nil {
		t.Error("folder server nggak boleh disentuh")
	}
}

func TestExitCodeReported(t *testing.T) {
	i := NewWithRunner(t.TempDir(), func(context.Context, Job) (string, error) {
		return "boom", fmt.Errorf("script install gagal: %w", exec.Command("sh", "-c", "exit 7").Run())
	})
	if err := i.Start("srv1", Spec{Script: "exit 7", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	st := waitStatus(t, i, "srv1", Failed)
	if st.ExitCode == nil || *st.ExitCode != 7 {
		t.Fatalf("exit_code = %v, mau 7", st.ExitCode)
	}

	ok := NewWithRunner(t.TempDir(), func(context.Context, Job) (string, error) { return "", nil })
	if err := ok.Start("srv1", Spec{Script: "true", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	st = waitStatus(t, ok, "srv1", Completed)
	if st.ExitCode == nil || *st.ExitCode != 0 {
		t.Fatalf("exit_code sukses = %v, mau 0", st.ExitCode)
	}

	other := NewWithRunner(t.TempDir(), func(context.Context, Job) (string, error) {
		return "", errors.New("pull image gagal")
	})
	if err := other.Start("srv1", Spec{Script: "true", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	if st = waitStatus(t, other, "srv1", Failed); st.ExitCode != nil {
		t.Errorf("gagal bukan karena script harusnya tanpa exit_code, dapat %d", *st.ExitCode)
	}
}

func TestStatePersistsAcrossRestart(t *testing.T) {
	root := t.TempDir()
	first := NewWithRunner(root, func(context.Context, Job) (string, error) { return "log install", nil })
	if err := first.Start("srv1", Spec{Script: "true", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, first, "srv1", Completed)

	second := NewWithRunner(root, nil)
	if st := second.State("srv1"); st.Status != Idle {
		t.Fatalf("sebelum Recover harusnya idle, dapat %q", st.Status)
	}
	second.Recover()
	st := second.State("srv1")
	if st.Status != Completed || st.Log != "log install" || st.ExitCode == nil || *st.ExitCode != 0 {
		t.Errorf("status setelah restart = %+v", st)
	}

	info, err := os.Stat(filepath.Join(root, stateFile))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode file status = %v, mau 0600", info.Mode().Perm())
	}
}

func TestRunningStateIsPersistedAtStart(t *testing.T) {
	root := t.TempDir()
	release := make(chan struct{})
	i := NewWithRunner(root, func(context.Context, Job) (string, error) {
		<-release
		return "", nil
	})
	if err := i.Start("srv1", Spec{Script: "true", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, stateFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), `"running"`) {
		t.Errorf("file status = %s", data)
	}
	close(release)
	waitStatus(t, i, "srv1", Completed)
}

func TestInterruptedInstallBecomesFailed(t *testing.T) {
	root := t.TempDir()
	raw := `{"srv1":{"status":"running"},"srv2":{"status":"completed","log":"ok"},"../x":{"status":"running"}}`
	if err := os.WriteFile(filepath.Join(root, stateFile), []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}

	var killed []string
	i := NewWithRunner(root, nil)
	i.kill = func(name string) { killed = append(killed, name) }
	i.Recover()

	st := i.State("srv1")
	if st.Status != Failed || !strings.Contains(st.Error, "restart") {
		t.Errorf("srv1 = %+v", st)
	}
	if i.IsInstalling("srv1") {
		t.Error("install yang terputus nggak boleh dianggap masih berjalan")
	}
	if got := i.State("srv2"); got.Status != Completed || got.Log != "ok" {
		t.Errorf("srv2 = %+v", got)
	}
	if len(killed) != 1 || killed[0] != "dockpanel-install-srv1" {
		t.Errorf("container yang dimatikan = %v", killed)
	}

	// Perubahan ke "failed" harus ikut tersimpan.
	again := NewWithRunner(root, nil)
	again.Recover()
	if got := again.State("srv1"); got.Status != Failed {
		t.Errorf("setelah Recover kedua srv1 = %+v", got)
	}
}

func TestCorruptStateFileIsIgnored(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, stateFile), []byte("{bukan json"), 0o600); err != nil {
		t.Fatal(err)
	}
	i := NewWithRunner(root, nil)
	i.Recover()
	if st := i.State("srv1"); st.Status != Idle {
		t.Errorf("status = %+v", st)
	}
}

func TestRecoverKeepsStateFile(t *testing.T) {
	root := t.TempDir()
	i := NewWithRunner(root, func(context.Context, Job) (string, error) { return "", nil })
	if err := i.Start("srv1", Spec{Script: "true", Image: "alpine"}); err != nil {
		t.Fatal(err)
	}
	waitStatus(t, i, "srv1", Completed)
	i.Recover()
	if _, err := os.Stat(filepath.Join(root, stateFile)); err != nil {
		t.Errorf("Recover nggak boleh menghapus file status: %v", err)
	}
}
