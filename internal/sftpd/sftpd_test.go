package sftpd

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"io"
	"net"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
)

const testUUID = "11111111-2222-3333-4444-555555555555"

func fakeAuth(_ context.Context, user, pw, _ string) (*AuthResult, error) {
	switch {
	case user == "rw" && pw == "ok":
		return &AuthResult{ServerUUID: testUUID}, nil
	case user == "ro" && pw == "ok":
		return &AuthResult{ServerUUID: testUUID, ReadOnly: true}, nil
	}
	return nil, ErrAuthDenied
}

func setup(t *testing.T) (addr, root string) {
	t.Helper()
	root = t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, testUUID), 0o755); err != nil {
		t.Fatal(err)
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	signer, err := ssh.NewSignerFromKey(priv)
	if err != nil {
		t.Fatal(err)
	}
	srv := New(Config{DataRoot: root, HostKey: signer, Auth: fakeAuth})
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	go func() { _ = srv.Serve(ln) }()
	t.Cleanup(srv.Close)
	return ln.Addr().String(), root
}

func dial(t *testing.T, addr, user, pass string) (*sftp.Client, error) {
	t.Helper()
	cfg := &ssh.ClientConfig{
		User:            user,
		Auth:            []ssh.AuthMethod{ssh.Password(pass)},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		Timeout:         5 * time.Second,
	}
	c, err := ssh.Dial("tcp", addr, cfg)
	if err != nil {
		return nil, err
	}
	cl, err := sftp.NewClient(c)
	if err != nil {
		c.Close()
		return nil, err
	}
	t.Cleanup(func() { cl.Close(); c.Close() })
	return cl, nil
}

func TestLoginSalahDitolak(t *testing.T) {
	addr, _ := setup(t)
	if _, err := dial(t, addr, "rw", "salah"); err == nil {
		t.Fatal("password salah harusnya ditolak")
	}
	if _, err := dial(t, addr, "orang", "ok"); err == nil {
		t.Fatal("user nggak dikenal harusnya ditolak")
	}
}

func TestTulisBacaListRename(t *testing.T) {
	addr, root := setup(t)
	cl, err := dial(t, addr, "rw", "ok")
	if err != nil {
		t.Fatal(err)
	}
	f, err := cl.Create("/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write([]byte("halo")); err != nil {
		t.Fatal(err)
	}
	f.Close()

	rf, err := cl.Open("/a.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rf)
	rf.Close()
	if string(data) != "halo" {
		t.Fatalf("isi salah: %q", data)
	}

	infos, err := cl.ReadDir("/")
	if err != nil {
		t.Fatal(err)
	}
	ada := false
	for _, fi := range infos {
		if fi.Name() == "a.txt" {
			ada = true
		}
	}
	if !ada {
		t.Fatal("a.txt nggak muncul di listing")
	}

	if err := cl.Mkdir("/d"); err != nil {
		t.Fatal(err)
	}
	if err := cl.Rename("/a.txt", "/d/b.txt"); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, testUUID, "d", "b.txt"))
	if err != nil || string(b) != "halo" {
		t.Fatalf("file hasil rename salah: %q %v", b, err)
	}
	if err := cl.Remove("/d/b.txt"); err != nil {
		t.Fatal(err)
	}
	if err := cl.RemoveDirectory("/d"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, testUUID, "d")); !os.IsNotExist(err) {
		t.Fatal("folder d harusnya sudah hilang")
	}
}

func TestTidakBisaKeluarFolder(t *testing.T) {
	addr, root := setup(t)
	rahasia := filepath.Join(root, "rahasia.txt")
	if err := os.WriteFile(rahasia, []byte("rahasia"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(rahasia, filepath.Join(root, testUUID, "link")); err != nil {
		t.Fatal(err)
	}
	cl, err := dial(t, addr, "rw", "ok")
	if err != nil {
		t.Fatal(err)
	}
	if f, err := cl.Open("/link"); err == nil {
		f.Close()
		t.Fatal("symlink ke luar folder server bisa dibaca")
	}
	if f, err := cl.Open("/../rahasia.txt"); err == nil {
		f.Close()
		t.Fatal("path .. bisa keluar folder server")
	}
	if f, err := cl.Create("/link"); err == nil {
		f.Close()
		t.Fatal("symlink ke luar folder server bisa ditulis")
	}
	b, _ := os.ReadFile(rahasia)
	if string(b) != "rahasia" {
		t.Fatalf("file di luar folder server berubah: %q", b)
	}
}

func TestReadOnly(t *testing.T) {
	addr, root := setup(t)
	if err := os.WriteFile(filepath.Join(root, testUUID, "x.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	cl, err := dial(t, addr, "ro", "ok")
	if err != nil {
		t.Fatal(err)
	}
	rf, err := cl.Open("/x.txt")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := io.ReadAll(rf)
	rf.Close()
	if string(data) != "x" {
		t.Fatalf("isi salah: %q", data)
	}
	if f, err := cl.Create("/n.txt"); err == nil {
		f.Close()
		t.Fatal("user read-only bisa nulis")
	}
	if err := cl.Remove("/x.txt"); err == nil {
		t.Fatal("user read-only bisa hapus")
	}
	if err := cl.Mkdir("/dd"); err == nil {
		t.Fatal("user read-only bisa bikin folder")
	}
}
