package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
)

const (
	tmpSuffix = ".restore-tmp"
	oldSuffix = ".restore-old"

	RestoreIdle      = "idle"
	RestoreRunning   = "restoring"
	RestoreCompleted = "completed"
	RestoreFailed    = "failed"
)

var (
	ErrRestoring = errors.New("restore sedang berjalan untuk server ini")
	ErrChecksum  = errors.New("checksum arsip nggak cocok, backup rusak atau diubah")
	ErrUnsafe    = errors.New("arsip berisi path yang nggak aman")
)

// RestoreState = status restore terakhir untuk satu server (disimpan di memori).
type RestoreState struct {
	Status string `json:"status"`
	Backup string `json:"backup,omitempty"`
	Error  string `json:"error,omitempty"`
}

// Restorer ngembaliin isi folder data server dari backup. Arsip diekstrak ke
// folder sementara dulu, diverifikasi checksum-nya, baru ditukar dengan folder
// asli. Kalau ada yang gagal, folder asli nggak disentuh.
type Restorer struct {
	store *Store

	mu    sync.Mutex
	state map[string]RestoreState
}

func NewRestorer(store *Store) *Restorer {
	return &Restorer{store: store, state: map[string]RestoreState{}}
}

// State balikin status restore terakhir server ini ("idle" kalau belum pernah).
func (r *Restorer) State(server string) RestoreState {
	r.mu.Lock()
	defer r.mu.Unlock()
	if st, ok := r.state[server]; ok {
		return st
	}
	return RestoreState{Status: RestoreIdle}
}

func (r *Restorer) IsRestoring(server string) bool {
	return r.State(server).Status == RestoreRunning
}

// Start mulai restore di background. Pemanggil wajib memastikan server mati.
func (r *Restorer) Start(server, id string) error {
	if !validIDs(server, id) {
		return ErrInvalidID
	}
	archive, err := r.store.Path(server, id)
	if err != nil {
		return err
	}
	info, err := r.store.Get(server, id)
	if err != nil {
		return err
	}

	r.store.mu.Lock()
	for key := range r.store.running {
		if strings.HasPrefix(key, server+"/") {
			r.store.mu.Unlock()
			return ErrBusy
		}
	}
	r.store.mu.Unlock()

	r.mu.Lock()
	if r.state[server].Status == RestoreRunning {
		r.mu.Unlock()
		return ErrRestoring
	}
	r.state[server] = RestoreState{Status: RestoreRunning, Backup: id}
	r.mu.Unlock()

	go func() {
		err := r.run(server, archive, info.Checksum)
		st := RestoreState{Status: RestoreCompleted, Backup: id}
		if err != nil {
			st = RestoreState{Status: RestoreFailed, Backup: id, Error: r.clean(err)}
			log.Printf("restore %s server %s gagal: %v", id, server, err)
		} else {
			log.Printf("restore %s server %s selesai", id, server)
		}
		r.mu.Lock()
		r.state[server] = st
		r.mu.Unlock()
	}()

	return nil
}

// clean buang path host dari pesan error sebelum dikirim ke Panel.
func (r *Restorer) clean(err error) string {
	msg := strings.ReplaceAll(err.Error(), r.store.dataRoot, "")
	return strings.ReplaceAll(msg, r.store.backupRoot, "")
}

func (r *Restorer) run(server, archive, checksum string) error {
	dir := filepath.Join(r.store.dataRoot, server)
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return ErrNoData
	}

	if err := verifyChecksum(archive, checksum); err != nil {
		return err
	}

	tmp, old := dir+tmpSuffix, dir+oldSuffix
	_ = os.RemoveAll(tmp)
	_ = os.RemoveAll(old)

	if err := os.MkdirAll(tmp, 0o755); err != nil {
		return err
	}
	if err := extract(archive, tmp); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	_ = os.Chmod(tmp, st.Mode().Perm())

	if err := os.Rename(dir, old); err != nil {
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.Rename(tmp, dir); err != nil {
		_ = os.Rename(old, dir) // balikin folder asli
		_ = os.RemoveAll(tmp)
		return err
	}
	if err := os.RemoveAll(old); err != nil {
		log.Printf("restore %s: gagal hapus folder lama: %v", server, err)
	}
	return nil
}

func verifyChecksum(file, want string) error {
	if want == "" {
		return ErrChecksum
	}
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != want {
		return ErrChecksum
	}
	return nil
}

// safeJoin gabung nama entri arsip ke dest dan nolak path yang keluar dari dest.
func safeJoin(dest, name string) (string, error) {
	n := path.Clean(strings.TrimPrefix(filepath.ToSlash(name), "./"))
	if n == "." {
		return "", nil
	}
	if path.IsAbs(n) || n == ".." || strings.HasPrefix(n, "../") {
		return "", ErrUnsafe
	}
	target := filepath.Join(dest, filepath.FromSlash(n))
	if !strings.HasPrefix(target, dest+string(os.PathSeparator)) {
		return "", ErrUnsafe
	}
	return target, nil
}

type dirMeta struct {
	path     string
	perm     os.FileMode
	uid, gid int
}

// extract ngeluarin arsip tar.gz ke dest. Cuma file biasa dan folder; symlink,
// hardlink, dan file spesial dilewati (backup juga nggak memuatnya).
func extract(archive, dest string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return fmt.Errorf("arsip bukan gzip yang valid: %w", err)
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	var dirs []dirMeta

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("arsip rusak: %w", err)
		}

		target, err := safeJoin(dest, hdr.Name)
		if err != nil {
			return err
		}
		if target == "" {
			continue
		}

		switch hdr.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			dirs = append(dirs, dirMeta{target, hdr.FileInfo().Mode().Perm(), hdr.Uid, hdr.Gid})

		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
				return err
			}
			out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode().Perm())
			if err != nil {
				return err
			}
			if _, err := io.Copy(out, io.LimitReader(tr, hdr.Size)); err != nil {
				_ = out.Close()
				return err
			}
			if err := out.Close(); err != nil {
				return err
			}
			_ = os.Lchown(target, hdr.Uid, hdr.Gid)
			_ = os.Chtimes(target, hdr.ModTime, hdr.ModTime)
		}
	}

	// Izin dan pemilik folder dipasang terakhir, dari yang terdalam.
	for i := len(dirs) - 1; i >= 0; i-- {
		d := dirs[i]
		_ = os.Chmod(d.path, d.perm)
		_ = os.Lchown(d.path, d.uid, d.gid)
	}
	return nil
}

// Recover beresin sisa restore yang terputus (daemon mati di tengah jalan).
// Dipanggil sekali pas daemon start.
func (r *Restorer) Recover() {
	root := r.store.dataRoot
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		full := filepath.Join(root, name)
		switch {
		case strings.HasSuffix(name, tmpSuffix):
			_ = os.RemoveAll(full)
		case strings.HasSuffix(name, oldSuffix):
			orig := filepath.Join(root, strings.TrimSuffix(name, oldSuffix))
			if _, err := os.Stat(orig); os.IsNotExist(err) {
				if err := os.Rename(full, orig); err == nil {
					log.Printf("restore terputus: folder %s dikembalikan", filepath.Base(orig))
				}
			} else {
				_ = os.RemoveAll(full)
			}
		}
	}
}
