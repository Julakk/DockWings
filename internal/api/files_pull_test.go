package api

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Julakk/DockWings/internal/files"
)

func TestPublicIP(t *testing.T) {
	cases := map[string]bool{
		"8.8.8.8": true, "1.1.1.1": true,
		"127.0.0.1": false, "10.0.0.1": false, "192.168.1.1": false,
		"172.16.0.1": false, "169.254.169.254": false, "100.64.0.1": false,
		"0.0.0.0": false, "::1": false, "fe80::1": false, "::ffff:10.0.0.1": false,
	}
	for s, want := range cases {
		if got := publicIP(net.ParseIP(s)); got != want {
			t.Errorf("publicIP(%s)=%v, harusnya %v", s, got, want)
		}
	}
}

func siapStore(t *testing.T) *files.Store {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "u1"), 0o755); err != nil {
		t.Fatal(err)
	}
	return files.New(root, "u1")
}

func TestPullBlokirLoopback(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("x"))
	}))
	defer srv.Close()
	_, err := pullToStore(context.Background(), siapStore(t), srv.URL+"/f.txt", "/f.txt", 1<<20)
	if !errors.Is(err, errBlockedAddr) {
		t.Fatalf("harusnya errBlockedAddr, dapat %v", err)
	}
}

func TestPullSkemaSalah(t *testing.T) {
	for _, u := range []string{"file:///etc/passwd", "ftp://a.b/c", "bukan url", ""} {
		if _, err := pullToStore(context.Background(), siapStore(t), u, "/f", 1<<20); !errors.Is(err, errBadURL) {
			t.Errorf("%q harusnya errBadURL, dapat %v", u, err)
		}
	}
}
