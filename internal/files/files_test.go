package files

import (
	"errors"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newStore(t *testing.T) (*Store, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "srv1"), 0o755); err != nil {
		t.Fatal(err)
	}
	return New(root, "srv1"), root
}

func TestWriteReadList(t *testing.T) {
	st, _ := newStore(t)

	if _, err := st.Write("/a/b.txt", strings.NewReader("halo")); err != nil {
		t.Fatalf("Write: %v", err)
	}

	f, info, err := st.Open("/a/b.txt")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer f.Close()
	data, _ := io.ReadAll(f)
	if string(data) != "halo" || info.Size() != 4 {
		t.Errorf("isi = %q size = %d", data, info.Size())
	}

	root, err := st.List("/")
	if err != nil || len(root) != 1 || root[0].Name != "a" || !root[0].IsDir {
		t.Fatalf("List / = %+v err=%v", root, err)
	}
	sub, err := st.List("/a")
	if err != nil || len(sub) != 1 || sub[0].Name != "b.txt" {
		t.Fatalf("List /a = %+v err=%v (temp file nyisa?)", sub, err)
	}
}

func TestTraversalDinetralkan(t *testing.T) {
	st, root := newStore(t)
	if err := os.WriteFile(filepath.Join(root, "secret.txt"), []byte("rahasia"), 0o644); err != nil {
		t.Fatal(err)
	}

	for _, p := range []string{"../secret.txt", "/../../secret.txt", "a/../../secret.txt"} {
		if _, _, err := st.Open(p); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("Open(%q) err = %v, mau ErrNotExist (bukan baca file di luar)", p, err)
		}
	}
}

func TestSymlinkEscape(t *testing.T) {
	st, root := newStore(t)
	base := filepath.Join(root, "srv1")
	outside := filepath.Join(root, "outside")
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	secret := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(secret, []byte("rahasia"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(base, "link")); err != nil {
		t.Skipf("symlink nggak didukung di sini: %v", err)
	}

	if _, _, err := st.Open("link/secret.txt"); !errors.Is(err, ErrForbidden) {
		t.Errorf("Open lewat symlink err = %v, mau ErrForbidden", err)
	}
	if _, err := st.Write("link/baru.txt", strings.NewReader("x")); !errors.Is(err, ErrForbidden) {
		t.Errorf("Write lewat symlink err = %v, mau ErrForbidden", err)
	}

	// Symlink menggantung ke luar folder server nggak boleh dipakai nulis.
	target := filepath.Join(outside, "belum-ada.txt")
	if err := os.Symlink(target, filepath.Join(base, "dangling")); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Write("dangling", strings.NewReader("x")); !errors.Is(err, ErrForbidden) {
		t.Errorf("Write ke dangling symlink err = %v, mau ErrForbidden", err)
	}
	if _, err := os.Stat(target); err == nil {
		t.Error("file di luar folder server kebuat lewat dangling symlink")
	}

	// Hapus symlink = cuma link-nya yang hilang, target tetap aman.
	if err := st.Delete("link"); err != nil {
		t.Fatalf("Delete symlink: %v", err)
	}
	if _, err := os.Stat(secret); err != nil {
		t.Errorf("target symlink ikut kehapus: %v", err)
	}
}

func TestRootDilindungi(t *testing.T) {
	st, _ := newStore(t)
	if err := st.Delete("/"); !errors.Is(err, ErrRoot) {
		t.Errorf("Delete / err = %v", err)
	}
	if err := st.Rename("/", "x"); !errors.Is(err, ErrRoot) {
		t.Errorf("Rename / err = %v", err)
	}
	if _, err := st.Write("/", strings.NewReader("x")); !errors.Is(err, ErrRoot) {
		t.Errorf("Write / err = %v", err)
	}
}

func TestMkdirRenameDelete(t *testing.T) {
	st, _ := newStore(t)

	if err := st.Mkdir("/plugins/sub"); err != nil {
		t.Fatalf("Mkdir: %v", err)
	}
	if _, err := st.Write("/plugins/a.txt", strings.NewReader("a")); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Write("/b.txt", strings.NewReader("b")); err != nil {
		t.Fatal(err)
	}

	if err := st.Rename("/plugins/a.txt", "/plugins/sub/a2.txt"); err != nil {
		t.Fatalf("Rename: %v", err)
	}
	if _, _, err := st.Open("/plugins/a.txt"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("file lama masih ada: %v", err)
	}
	if err := st.Rename("/b.txt", "/plugins/sub/a2.txt"); !errors.Is(err, ErrExists) {
		t.Errorf("Rename ke tujuan yang ada err = %v, mau ErrExists", err)
	}

	if err := st.Delete("/plugins"); err != nil {
		t.Fatalf("Delete folder: %v", err)
	}
	if err := st.Delete("/plugins"); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("Delete kedua err = %v, mau ErrNotExist", err)
	}
	if _, err := st.List("/b.txt"); !errors.Is(err, ErrNotDir) {
		t.Errorf("List file err = %v, mau ErrNotDir", err)
	}
}
