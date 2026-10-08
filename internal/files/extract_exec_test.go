package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractExecBit(t *testing.T) {
	root := t.TempDir()
	st := bikinZip(t, root, map[string]string{"samp03svr": "\x7fELF\x01\x01", "server.cfg": "port 7777"})
	if _, err := st.Extract("/t.zip"); err != nil {
		t.Fatal(err)
	}
	bin, err := os.Stat(filepath.Join(root, "u1", "samp03svr"))
	if err != nil || bin.Mode()&0o111 == 0 {
		t.Fatalf("ELF harusnya executable: %v %v", bin, err)
	}
	cfg, err := os.Stat(filepath.Join(root, "u1", "server.cfg"))
	if err != nil || cfg.Mode()&0o111 != 0 {
		t.Fatalf("file teks nggak boleh executable: %v %v", cfg, err)
	}
}
