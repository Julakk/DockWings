package sftpd

import (
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/crypto/ssh"
)

func TestHostKeyTetapSama(t *testing.T) {
	p := filepath.Join(t.TempDir(), "sub", "hostkey")
	a, err := LoadOrCreateHostKey(p)
	if err != nil {
		t.Fatal(err)
	}
	fi, err := os.Stat(p)
	if err != nil || fi.Mode().Perm() != 0o600 {
		t.Fatalf("file key harus mode 600: %v %v", fi, err)
	}
	b, err := LoadOrCreateHostKey(p)
	if err != nil {
		t.Fatal(err)
	}
	if ssh.FingerprintSHA256(a.PublicKey()) != ssh.FingerprintSHA256(b.PublicKey()) {
		t.Fatal("host key berubah antar-pemanggilan")
	}
}
