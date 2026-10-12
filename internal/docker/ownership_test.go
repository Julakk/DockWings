package docker

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestUserSpec(t *testing.T) {
	e := NewDockerEnvironment(t.TempDir())
	if got := e.userSpec(); got != "" {
		t.Fatalf("default harus kosong, dapat %q", got)
	}
	e.SetUser(988, 0)
	if got := e.userSpec(); got != "988:988" {
		t.Fatalf("gid kosong harus ikut uid, dapat %q", got)
	}
	e.SetUser(0, 0)
	if got := e.userSpec(); got != "" {
		t.Fatalf("uid 0 harus mematikan fitur, dapat %q", got)
	}
}

func TestChownTree(t *testing.T) {
	if os.Geteuid() != 0 {
		t.Skip("butuh root buat chown")
	}
	base := t.TempDir()
	dir := filepath.Join(base, "server")
	outside := filepath.Join(base, "outside.txt")
	if err := os.MkdirAll(filepath.Join(dir, "plugins"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "plugins", "a.yml"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(outside, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(dir, "link")); err != nil {
		t.Fatal(err)
	}

	if err := ChownTree(dir, 12345, 12345); err != nil {
		t.Fatal(err)
	}

	owner := func(p string) (int, int) {
		fi, err := os.Lstat(p)
		if err != nil {
			t.Fatal(err)
		}
		st := fi.Sys().(*syscall.Stat_t)
		return int(st.Uid), int(st.Gid)
	}
	for _, p := range []string{dir, filepath.Join(dir, "plugins"), filepath.Join(dir, "plugins", "a.yml"), filepath.Join(dir, "link")} {
		if u, g := owner(p); u != 12345 || g != 12345 {
			t.Errorf("%s owner %d:%d, mau 12345:12345", p, u, g)
		}
	}
	if u, g := owner(outside); u != 0 || g != 0 {
		t.Errorf("file di luar server ikut ke-chown (%d:%d), symlink diikuti!", u, g)
	}
}
