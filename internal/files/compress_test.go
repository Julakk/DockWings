package files

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func siapkan(t *testing.T) (string, *Store) {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "u1", "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	os.WriteFile(filepath.Join(root, "u1", "plugins", "a.txt"), []byte("halo"), 0o644)
	os.WriteFile(filepath.Join(root, "u1", "server.cfg"), []byte("port 7777"), 0o644)
	return root, New(root, "u1")
}

func TestCompressRoundtrip(t *testing.T) {
	root, st := siapkan(t)
	n, err := st.Compress([]string{"/plugins", "/server.cfg"}, "/out.zip")
	if err != nil || n != 2 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if err := st.Rename("/out.zip", "/x/out.zip"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Extract("/x/out.zip"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "u1", "x", "plugins", "a.txt"))
	if err != nil || string(b) != "halo" {
		t.Fatalf("isi salah: %q %v", b, err)
	}
	b, err = os.ReadFile(filepath.Join(root, "u1", "x", "server.cfg"))
	if err != nil || string(b) != "port 7777" {
		t.Fatalf("isi salah: %q %v", b, err)
	}
}

func TestCompressNamaDanTimpa(t *testing.T) {
	_, st := siapkan(t)
	if _, err := st.Compress([]string{"/plugins"}, "/out.rar"); !errors.Is(err, ErrNeedZip) {
		t.Fatalf("harusnya ErrNeedZip, dapat %v", err)
	}
	if _, err := st.Compress([]string{"/plugins"}, "/out.zip"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.Compress([]string{"/plugins"}, "/out.zip"); !errors.Is(err, ErrExists) {
		t.Fatalf("harusnya ErrExists, dapat %v", err)
	}
	if _, err := st.Compress(nil, "/o2.zip"); !errors.Is(err, ErrNoInput) {
		t.Fatalf("harusnya ErrNoInput, dapat %v", err)
	}
}

func TestSetExecutable(t *testing.T) {
	root, st := siapkan(t)
	p := filepath.Join(root, "u1", "server.cfg")
	if err := st.SetExecutable("/server.cfg", true); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode()&0o111 == 0 {
		t.Fatal("harusnya executable")
	}
	if err := st.SetExecutable("/server.cfg", false); err != nil {
		t.Fatal(err)
	}
	if fi, _ := os.Stat(p); fi.Mode()&0o111 != 0 {
		t.Fatal("harusnya nggak executable")
	}
	if err := st.SetExecutable("/plugins", true); !errors.Is(err, ErrIsDir) {
		t.Fatalf("folder harusnya ErrIsDir, dapat %v", err)
	}
}
