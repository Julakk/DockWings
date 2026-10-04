package backup

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func writeFile(t *testing.T, p, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func waitBackup(t *testing.T, s *Store, server, id string) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		info, err := s.Get(server, id)
		if err != nil {
			t.Fatal(err)
		}
		if info.Status != StatusCreating {
			if info.Status != StatusCompleted {
				t.Fatalf("backup gagal: %+v", info)
			}
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("backup nggak selesai tepat waktu")
}

func waitRestore(t *testing.T, r *Restorer, server string) RestoreState {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if st := r.State(server); st.Status != RestoreRunning {
			return st
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("restore nggak selesai tepat waktu")
	return RestoreState{}
}

func TestRestoreReplacesDirectory(t *testing.T) {
	data, bak := t.TempDir(), t.TempDir()
	src := filepath.Join(data, "srv1")
	writeFile(t, filepath.Join(src, "server.cfg"), "asli")
	writeFile(t, filepath.Join(src, "plugins", "a.so"), "plugin-asli")

	s := New(data, bak)
	if err := s.Start("srv1", "b1"); err != nil {
		t.Fatal(err)
	}
	waitBackup(t, s, "srv1", "b1")

	// Server berubah setelah backup: file diubah, ditambah, dihapus.
	writeFile(t, filepath.Join(src, "server.cfg"), "rusak")
	writeFile(t, filepath.Join(src, "sampah.txt"), "x")
	if err := os.Remove(filepath.Join(src, "plugins", "a.so")); err != nil {
		t.Fatal(err)
	}

	r := NewRestorer(s)
	if got := r.State("srv1").Status; got != RestoreIdle {
		t.Fatalf("status awal = %q", got)
	}
	if err := r.Start("srv1", "b1"); err != nil {
		t.Fatal(err)
	}
	st := waitRestore(t, r, "srv1")
	if st.Status != RestoreCompleted {
		t.Fatalf("restore gagal: %+v", st)
	}

	if b, _ := os.ReadFile(filepath.Join(src, "server.cfg")); string(b) != "asli" {
		t.Errorf("server.cfg = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(src, "plugins", "a.so")); string(b) != "plugin-asli" {
		t.Errorf("a.so = %q", b)
	}
	if _, err := os.Stat(filepath.Join(src, "sampah.txt")); !os.IsNotExist(err) {
		t.Error("file baru harusnya hilang setelah restore")
	}
	for _, suf := range []string{tmpSuffix, oldSuffix} {
		if _, err := os.Stat(src + suf); !os.IsNotExist(err) {
			t.Errorf("sisa %s seharusnya sudah dibersihkan", suf)
		}
	}
}

// bikinArsip nulis arsip tar.gz palsu + meta lengkap dengan checksum yang benar.
func bikinArsip(t *testing.T, s *Store, server, id string, build func(*tar.Writer)) {
	t.Helper()
	if err := os.MkdirAll(s.dir(server), 0o750); err != nil {
		t.Fatal(err)
	}
	f, err := os.Create(s.archive(server, id))
	if err != nil {
		t.Fatal(err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)
	build(tw)
	_ = tw.Close()
	_ = gz.Close()
	_ = f.Close()

	raw, _ := os.ReadFile(s.archive(server, id))
	sum := sha256.Sum256(raw)
	meta, _ := json.Marshal(Info{UUID: id, Status: StatusCompleted, Size: int64(len(raw)), Checksum: hex.EncodeToString(sum[:])})
	if err := os.WriteFile(s.metaFile(server, id), meta, 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreRejectsPathTraversal(t *testing.T) {
	data, bak := t.TempDir(), t.TempDir()
	src := filepath.Join(data, "srv1")
	writeFile(t, filepath.Join(src, "server.cfg"), "asli")

	s := New(data, bak)
	bikinArsip(t, s, "srv1", "jahat", func(tw *tar.Writer) {
		_ = tw.WriteHeader(&tar.Header{Name: "../../kabur.txt", Mode: 0o644, Size: 3, Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte("hai"))
	})

	r := NewRestorer(s)
	if err := r.Start("srv1", "jahat"); err != nil {
		t.Fatal(err)
	}
	st := waitRestore(t, r, "srv1")
	if st.Status != RestoreFailed {
		t.Fatalf("harusnya failed, dapat %+v", st)
	}
	if _, err := os.Stat(filepath.Join(filepath.Dir(data), "kabur.txt")); !os.IsNotExist(err) {
		t.Fatal("file berhasil keluar dari folder tujuan")
	}
	if b, _ := os.ReadFile(filepath.Join(src, "server.cfg")); string(b) != "asli" {
		t.Error("folder asli harus tetap utuh kalau restore gagal")
	}
	if _, err := os.Stat(src + tmpSuffix); !os.IsNotExist(err) {
		t.Error("folder sementara harus dibersihkan")
	}
}

func TestRestoreSkipsSymlinks(t *testing.T) {
	data, bak := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(data, "srv1", "a.txt"), "a")

	s := New(data, bak)
	bikinArsip(t, s, "srv1", "b2", func(tw *tar.Writer) {
		_ = tw.WriteHeader(&tar.Header{Name: "link", Linkname: "/etc/passwd", Typeflag: tar.TypeSymlink})
		_ = tw.WriteHeader(&tar.Header{Name: "ok.txt", Mode: 0o644, Size: 2, Typeflag: tar.TypeReg})
		_, _ = tw.Write([]byte("ok"))
	})

	r := NewRestorer(s)
	if err := r.Start("srv1", "b2"); err != nil {
		t.Fatal(err)
	}
	if st := waitRestore(t, r, "srv1"); st.Status != RestoreCompleted {
		t.Fatalf("restore gagal: %+v", st)
	}
	if _, err := os.Lstat(filepath.Join(data, "srv1", "link")); !os.IsNotExist(err) {
		t.Error("symlink seharusnya dilewati")
	}
	if b, _ := os.ReadFile(filepath.Join(data, "srv1", "ok.txt")); string(b) != "ok" {
		t.Error("file biasa harusnya ikut ke-restore")
	}
}

func TestRestoreRejectsTamperedArchive(t *testing.T) {
	data, bak := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(data, "srv1", "a.txt"), "a")

	s := New(data, bak)
	if err := s.Start("srv1", "b1"); err != nil {
		t.Fatal(err)
	}
	waitBackup(t, s, "srv1", "b1")

	f, _ := os.OpenFile(s.archive("srv1", "b1"), os.O_APPEND|os.O_WRONLY, 0)
	_, _ = f.Write([]byte("sampah"))
	_ = f.Close()

	r := NewRestorer(s)
	if err := r.Start("srv1", "b1"); err != nil {
		t.Fatal(err)
	}
	st := waitRestore(t, r, "srv1")
	if st.Status != RestoreFailed || st.Error == "" {
		t.Fatalf("harusnya failed karena checksum, dapat %+v", st)
	}
	if b, _ := os.ReadFile(filepath.Join(data, "srv1", "a.txt")); string(b) != "a" {
		t.Error("folder asli harus tetap utuh")
	}
}

func TestRestoreRejectsUnknownBackup(t *testing.T) {
	r := NewRestorer(New(t.TempDir(), t.TempDir()))
	if err := r.Start("srv1", "nggak-ada"); !errors.Is(err, ErrNotFound) {
		t.Errorf("dapat %v", err)
	}
	if err := r.Start("../x", "b"); !errors.Is(err, ErrInvalidID) {
		t.Errorf("dapat %v", err)
	}
}

func TestRecoverRestoresInterruptedSwap(t *testing.T) {
	data := t.TempDir()
	// Daemon mati setelah folder asli di-rename tapi sebelum folder baru dipasang.
	writeFile(t, filepath.Join(data, "srv1"+oldSuffix, "a.txt"), "data-asli")
	writeFile(t, filepath.Join(data, "srv1"+tmpSuffix, "b.txt"), "setengah")
	// Swap selesai tapi folder lama belum sempat dihapus.
	writeFile(t, filepath.Join(data, "srv2", "a.txt"), "baru")
	writeFile(t, filepath.Join(data, "srv2"+oldSuffix, "a.txt"), "lama")

	NewRestorer(New(data, t.TempDir())).Recover()

	if b, _ := os.ReadFile(filepath.Join(data, "srv1", "a.txt")); string(b) != "data-asli" {
		t.Errorf("srv1 harusnya dikembalikan, a.txt = %q", b)
	}
	if b, _ := os.ReadFile(filepath.Join(data, "srv2", "a.txt")); string(b) != "baru" {
		t.Errorf("srv2 harusnya tetap yang baru, a.txt = %q", b)
	}
	for _, n := range []string{"srv1" + tmpSuffix, "srv1" + oldSuffix, "srv2" + oldSuffix} {
		if _, err := os.Stat(filepath.Join(data, n)); !os.IsNotExist(err) {
			t.Errorf("%s harusnya sudah dibersihkan", n)
		}
	}
}
