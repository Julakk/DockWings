package files

import "os"

// SetExecutable set izin file jadi 0755 (exec=true) atau 0644 (exec=false).
// Cuma buat file biasa, folder ditolak.
func (s *Store) SetExecutable(rel string, exec bool) error {
	if isRoot(rel) {
		return ErrRoot
	}
	p, err := s.resolve(rel)
	if err != nil {
		return err
	}
	info, err := os.Stat(p)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return ErrIsDir
	}
	mode := os.FileMode(0o644)
	if exec {
		mode = 0o755
	}
	return os.Chmod(p, mode)
}
