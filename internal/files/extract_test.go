package files

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func bikinZip(t *testing.T, root string, entries map[string]string) *Store {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, "u1"), 0o755); err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, isi := range entries {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		w.Write([]byte(isi))
	}
	zw.Close()
	if err := os.WriteFile(filepath.Join(root, "u1", "t.zip"), buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	return New(root, "u1")
}

func TestExtractZip(t *testing.T) {
	root := t.TempDir()
	st := bikinZip(t, root, map[string]string{"a/b.txt": "halo"})
	n, err := st.Extract("/t.zip")
	if err != nil || n != 1 {
		t.Fatalf("n=%d err=%v", n, err)
	}
	b, err := os.ReadFile(filepath.Join(root, "u1", "a", "b.txt"))
	if err != nil || string(b) != "halo" {
		t.Fatalf("isi salah: %q %v", b, err)
	}
}

func TestExtractZipSlip(t *testing.T) {
	root := t.TempDir()
	st := bikinZip(t, root, map[string]string{"../evil.txt": "x"})
	_, err := st.Extract("/t.zip")
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("harusnya ErrForbidden, dapat %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "evil.txt")); err == nil {
		t.Fatal("file keluar dari folder server")
	}
}

func TestExtractUnsupported(t *testing.T) {
	root := t.TempDir()
	st := bikinZip(t, root, map[string]string{"a.txt": "x"})
	os.Rename(filepath.Join(root, "u1", "t.zip"), filepath.Join(root, "u1", "t.rar"))
	if _, err := st.Extract("/t.rar"); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("harusnya ErrUnsupported, dapat %v", err)
	}
}
