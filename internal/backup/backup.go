// Package backup bikin, cek, ambil, dan hapus backup server (arsip tar.gz
// dari folder data). Pembuatan jalan di background; status dibaca lewat Get.
package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

var (
	ErrInvalidID = errors.New("id nggak valid")
	ErrNotFound  = errors.New("backup nggak ketemu")
	ErrBusy      = errors.New("backup ini sudah ada atau masih diproses")
	ErrNoData    = errors.New("folder data server nggak ketemu")

	idRe = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,63}$`)
)

const (
	StatusCreating  = "creating"
	StatusCompleted = "completed"
	StatusFailed    = "failed"
)

// Info = status satu backup, juga dikirim apa adanya ke Panel.
type Info struct {
	UUID        string `json:"uuid"`
	Status      string `json:"status"`
	Size        int64  `json:"size"`
	Checksum    string `json:"checksum"`
	Error       string `json:"error,omitempty"`
	CompletedAt string `json:"completed_at,omitempty"`
}

// Store ngatur backup di disk: <backupRoot>/<server uuid>/<id>.tar.gz (+ .json).
type Store struct {
	dataRoot   string
	backupRoot string

	mu      sync.Mutex
	running map[string]bool
}

func New(dataRoot, backupRoot string) *Store {
	return &Store{dataRoot: dataRoot, backupRoot: backupRoot, running: map[string]bool{}}
}

func (s *Store) dir(server string) string          { return filepath.Join(s.backupRoot, server) }
func (s *Store) archive(server, id string) string  { return filepath.Join(s.dir(server), id+".tar.gz") }
func (s *Store) partial(server, id string) string  { return s.archive(server, id) + ".part" }
func (s *Store) metaFile(server, id string) string { return filepath.Join(s.dir(server), id+".json") }
func validIDs(ids ...string) bool {
	for _, id := range ids {
		if !idRe.MatchString(id) {
			return false
		}
	}
	return true
}

// Start mulai bikin backup di background. Langsung balik; pantau lewat Get.
func (s *Store) Start(server, id string) error {
	if !validIDs(server, id) {
		return ErrInvalidID
	}
	src := filepath.Join(s.dataRoot, server)
	if st, err := os.Stat(src); err != nil || !st.IsDir() {
		return ErrNoData
	}

	key := server + "/" + id
	s.mu.Lock()
	if s.running[key] {
		s.mu.Unlock()
		return ErrBusy
	}
	if _, err := os.Stat(s.metaFile(server, id)); err == nil {
		s.mu.Unlock()
		return ErrBusy
	}
	s.running[key] = true
	s.mu.Unlock()

	go func() {
		info := s.build(server, id, src)
		if err := s.writeMeta(server, id, info); err != nil {
			log.Printf("gagal tulis meta backup %s: %v", id, err)
		}
		s.mu.Lock()
		delete(s.running, key)
		s.mu.Unlock()
		log.Printf("backup %s server %s: %s", id, server, info.Status)
	}()

	return nil
}

func (s *Store) build(server, id, src string) Info {
	info := Info{UUID: id, Status: StatusFailed}
	fail := func(err error) Info {
		_ = os.Remove(s.partial(server, id))
		msg := strings.ReplaceAll(err.Error(), s.dataRoot, "")
		msg = strings.ReplaceAll(msg, s.backupRoot, "")
		info.Error = msg
		return info
	}

	if err := os.MkdirAll(s.dir(server), 0o750); err != nil {
		return fail(err)
	}
	f, err := os.OpenFile(s.partial(server, id), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fail(err)
	}

	h := sha256.New()
	gz := gzip.NewWriter(io.MultiWriter(f, h))
	tw := tar.NewWriter(gz)

	if err := addTree(tw, src); err != nil {
		_ = f.Close()
		return fail(err)
	}
	if err := tw.Close(); err != nil {
		_ = f.Close()
		return fail(err)
	}
	if err := gz.Close(); err != nil {
		_ = f.Close()
		return fail(err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fail(err)
	}
	if err := f.Close(); err != nil {
		return fail(err)
	}
	if err := os.Rename(s.partial(server, id), s.archive(server, id)); err != nil {
		return fail(err)
	}

	st, err := os.Stat(s.archive(server, id))
	if err != nil {
		return fail(err)
	}
	info.Status = StatusCompleted
	info.Size = st.Size()
	info.Checksum = hex.EncodeToString(h.Sum(nil))
	info.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	return info
}

// addTree masukin isi folder src ke arsip. Symlink dan file spesial dilewati.
func addTree(tw *tar.Writer, src string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		if !d.IsDir() && !d.Type().IsRegular() {
			return nil
		}

		fi, err := d.Info()
		if err != nil {
			return err
		}
		hdr, err := tar.FileInfoHeader(fi, "")
		if err != nil {
			return err
		}
		hdr.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			hdr.Name += "/"
			return tw.WriteHeader(hdr)
		}
		if err := tw.WriteHeader(hdr); err != nil {
			return err
		}

		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		// File bisa berubah selama dibaca (server lagi jalan): ukuran di header
		// harus pas, jadi dipotong kalau membesar dan diisi nol kalau mengecil.
		n, err := io.CopyN(tw, in, hdr.Size)
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if n < hdr.Size {
			_, err = tw.Write(make([]byte, hdr.Size-n))
			return err
		}
		return nil
	})
}

func (s *Store) writeMeta(server, id string, info Info) error {
	data, err := json.Marshal(info)
	if err != nil {
		return err
	}
	tmp := s.metaFile(server, id) + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, s.metaFile(server, id))
}

// Get balikin status backup.
func (s *Store) Get(server, id string) (Info, error) {
	if !validIDs(server, id) {
		return Info{}, ErrInvalidID
	}

	s.mu.Lock()
	busy := s.running[server+"/"+id]
	s.mu.Unlock()
	if busy {
		return Info{UUID: id, Status: StatusCreating}, nil
	}

	data, err := os.ReadFile(s.metaFile(server, id))
	if err == nil {
		var info Info
		if err := json.Unmarshal(data, &info); err != nil {
			return Info{}, fmt.Errorf("meta backup rusak: %w", err)
		}
		return info, nil
	}
	if !os.IsNotExist(err) {
		return Info{}, err
	}
	if _, err := os.Stat(s.partial(server, id)); err == nil {
		// Ada file sisa tapi nggak ada proses: daemon sempat berhenti di tengah jalan.
		_ = os.Remove(s.partial(server, id))
		return Info{UUID: id, Status: StatusFailed, Error: "proses backup terputus (daemon restart)"}, nil
	}
	return Info{}, ErrNotFound
}

// Path balikin lokasi arsip backup yang sudah selesai.
func (s *Store) Path(server, id string) (string, error) {
	info, err := s.Get(server, id)
	if err != nil {
		return "", err
	}
	if info.Status != StatusCompleted {
		return "", ErrNotFound
	}
	return s.archive(server, id), nil
}

// Delete hapus backup (arsip + meta). Ditolak kalau masih diproses.
func (s *Store) Delete(server, id string) error {
	if !validIDs(server, id) {
		return ErrInvalidID
	}
	s.mu.Lock()
	busy := s.running[server+"/"+id]
	s.mu.Unlock()
	if busy {
		return ErrBusy
	}

	found := false
	for _, p := range []string{s.archive(server, id), s.partial(server, id), s.metaFile(server, id)} {
		err := os.Remove(p)
		if err == nil {
			found = true
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	if !found {
		return ErrNotFound
	}
	return nil
}
