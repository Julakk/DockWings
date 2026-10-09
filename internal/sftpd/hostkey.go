package sftpd

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/pem"
	"errors"
	"os"
	"path/filepath"

	"golang.org/x/crypto/ssh"
)

// LoadOrCreateHostKey baca host key SSH dari path, atau bikin baru (ed25519,
// mode 600) kalau belum ada. Key dipertahankan antar-restart supaya klien
// nggak kena peringatan "host key berubah".
func LoadOrCreateHostKey(path string) (ssh.Signer, error) {
	if b, err := os.ReadFile(path); err == nil {
		return ssh.ParsePrivateKey(b)
	} else if !errors.Is(err, os.ErrNotExist) {
		return nil, err
	}
	_, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return nil, err
	}
	blk, err := ssh.MarshalPrivateKey(priv, "dockwings")
	if err != nil {
		return nil, err
	}
	pemBytes := pem.EncodeToMemory(blk)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return LoadOrCreateHostKey(path)
		}
		return nil, err
	}
	_, werr := f.Write(pemBytes)
	cerr := f.Close()
	if werr != nil {
		return nil, werr
	}
	if cerr != nil {
		return nil, cerr
	}
	return ssh.ParsePrivateKey(pemBytes)
}
