package files

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var (
	ErrForbidden = errors.New("path di luar folder server")
	ErrRoot      = errors.New("nggak boleh operasi ke root folder server")
	ErrIsDir     = errors.New("path ini folder, bukan file")
	ErrNotDir    = errors.New("path ini bukan folder")
	ErrExists    = errors.New("tujuan sudah ada")
)

// Store ngasih akses file terbatas ke SATU folder server: <root>/<uuid>.
// uuid harus sudah divalidasi pemanggil (nggak boleh ada "/" atau "..").
type Store struct {
	base string
}

func New(root, uuid string) *Store {
	return &Store{base: filepath.Join(root, uuid)}
}

type Entry struct {
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	Modified time.Time `json:"modified"`
	Mode     string    `json:"mode"`
}

func isRoot(rel string) bool {
	return path.Clean("/"+filepath.ToSlash(rel)) == "/"
}

// resolve ngubah path relatif (dari user) jadi path absolut di host.
// ".." dinetralkan lewat path.Clean, dan symlink dicek supaya hasil akhirnya
// nggak keluar dari folder server. Path yang belum ada tetap boleh (buat
// write/mkdir), asal ancestor terdekat yang ada masih di dalam folder server.
func (s *Store) resolve(rel string) (string, error) {
	realBase, err := filepath.EvalSymlinks(s.base)
	if err != nil {
		return "", err
	}

	clean := path.Clean("/" + filepath.ToSlash(rel))
	p := filepath.Join(realBase, filepath.FromSlash(clean))

	var tail []string
	for {
		real, err := filepath.EvalSymlinks(p)
		if err == nil {
			if real != realBase && !strings.HasPrefix(real, realBase+string(filepath.Separator)) {
				return "", ErrForbidden
			}
			for i := len(tail) - 1; i >= 0; i-- {
				real = filepath.Join(real, tail[i])
			}
			return real, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		// Symlink menggantung (target belum ada) juga kena ErrNotExist.
		// Tolak, biar nggak bisa dipakai nulis ke luar folder server.
		if _, lerr := os.Lstat(p); lerr == nil {
			return "", ErrForbidden
		}
		parent := filepath.Dir(p)
		if parent == p || len(p) <= len(realBase) {
			return "", err
		}
		tail = append(tail, filepath.Base(p))
		p = parent
	}
}

// resolveNoFollow sama kayak resolve, tapi elemen terakhir TIDAK di-follow.
// Dipakai buat delete/rename supaya yang kena itu symlink-nya, bukan targetnya.
func (s *Store) resolveNoFollow(rel string) (string, error) {
	clean := path.Clean("/" + filepath.ToSlash(rel))
	dir, name := path.Split(clean)
	parent, err := s.resolve(dir)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, name), nil
}

func (s *Store) List(rel string) ([]Entry, error) {
	p, err := s.resolve(rel)
	if err != nil {
		return nil, err
	}
	info, err := os.Stat(p)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() {
		return nil, ErrNotDir
	}
	des, err := os.ReadDir(p)
	if err != nil {
		return nil, err
	}
	out := make([]Entry, 0, len(des))
	for _, de := range des {
		fi, err := de.Info()
		if err != nil {
			continue
		}
		out = append(out, Entry{
			Name:     de.Name(),
			Size:     fi.Size(),
			IsDir:    de.IsDir(),
			Modified: fi.ModTime().UTC(),
			Mode:     fi.Mode().String(),
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].IsDir != out[j].IsDir {
			return out[i].IsDir
		}
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out, nil
}

// Open buka file buat dibaca. Pemanggil wajib Close().
func (s *Store) Open(rel string) (*os.File, fs.FileInfo, error) {
	p, err := s.resolve(rel)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, nil, ErrIsDir
	}
	return f, info, nil
}

// Write nulis file lewat temp file + rename, jadi kalau upload putus
// nggak ada file setengah jadi. Folder induk dibikin otomatis.
func (s *Store) Write(rel string, r io.Reader) (int64, error) {
	if isRoot(rel) {
		return 0, ErrRoot
	}
	p, err := s.resolve(rel)
	if err != nil {
		return 0, err
	}
	if info, err := os.Stat(p); err == nil && info.IsDir() {
		return 0, ErrIsDir
	}
	dir := filepath.Dir(p)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(dir, ".dockwings-upload-*")
	if err != nil {
		return 0, err
	}
	n, err := io.Copy(tmp, r)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Chmod(tmp.Name(), 0o644)
	}
	if err == nil {
		err = os.Rename(tmp.Name(), p)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return 0, err
	}
	return n, nil
}

func (s *Store) Mkdir(rel string) error {
	p, err := s.resolve(rel)
	if err != nil {
		return err
	}
	return os.MkdirAll(p, 0o755)
}

func (s *Store) Delete(rel string) error {
	if isRoot(rel) {
		return ErrRoot
	}
	p, err := s.resolveNoFollow(rel)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(p); err != nil {
		return err
	}
	return os.RemoveAll(p)
}

func (s *Store) Rename(from, to string) error {
	if isRoot(from) || isRoot(to) {
		return ErrRoot
	}
	src, err := s.resolveNoFollow(from)
	if err != nil {
		return err
	}
	dst, err := s.resolveNoFollow(to)
	if err != nil {
		return err
	}
	if _, err := os.Lstat(src); err != nil {
		return err
	}
	if _, err := os.Lstat(dst); err == nil {
		return ErrExists
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.Rename(src, dst)
}
