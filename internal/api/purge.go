package api

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
)

// canonicalUUIDRe sengaja lebih ketat dari validUUID: operasi hapus data hanya
// boleh menyentuh folder yang namanya berbentuk UUID kanonik (8-4-4-4-12 hex),
// bukan nama sembarang di dalam data directory.
var canonicalUUIDRe = regexp.MustCompile(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`)

// purgeServerData menghapus folder data satu server di bawah root.
// Mengembalikan true kalau ada folder yang dihapus, false kalau memang tidak ada.
// Pengaman: uuid harus UUID kanonik, folder harus langsung di bawah root, dan
// folder itu sendiri tidak boleh symlink atau file biasa. os.RemoveAll tidak
// mengikuti symlink di dalam folder, jadi isi di luarnya tidak tersentuh.
func purgeServerData(root, uuid string) (bool, error) {
	if root == "" {
		return false, fmt.Errorf("data directory belum dikonfigurasi")
	}
	if !canonicalUUIDRe.MatchString(uuid) {
		return false, fmt.Errorf("uuid nggak valid")
	}

	rootAbs, err := filepath.Abs(root)
	if err != nil {
		return false, err
	}
	dir := filepath.Join(rootAbs, uuid)
	if filepath.Dir(dir) != rootAbs {
		return false, fmt.Errorf("path di luar data directory ditolak")
	}

	fi, err := os.Lstat(dir)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if fi.Mode()&os.ModeSymlink != 0 || !fi.IsDir() {
		return false, fmt.Errorf("%s bukan folder biasa, ditolak", dir)
	}

	if err := os.RemoveAll(dir); err != nil {
		return false, err
	}
	return true, nil
}
