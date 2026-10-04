package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
)

// runningEnv = environment palsu yang selalu melapor server lagi jalan.
type runningEnv struct{ *docker.StubEnvironment }

func (runningEnv) Resources(context.Context, *server.Server) (docker.Resources, error) {
	return docker.Resources{State: "running"}, nil
}

func doCall(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func buatBackup(t *testing.T, r http.Handler, uuid, id string) {
	t.Helper()
	if rec := doCall(t, r, "POST", "/api/servers/"+uuid+"/backups", `{"uuid":"`+id+`"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("create backup: %d %s", rec.Code, rec.Body)
	}
	for i := 0; i < 300; i++ {
		rec := doCall(t, r, "GET", "/api/servers/"+uuid+"/backups/"+id, "")
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["status"] == "completed" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("backup nggak selesai")
}

func TestRestoreEndpoint(t *testing.T) {
	data, bak := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(data, "srv1"), 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := filepath.Join(data, "srv1", "a.txt")
	if err := os.WriteFile(cfg, []byte("asli"), 0o644); err != nil {
		t.Fatal(err)
	}

	mgr := server.NewManager()
	mgr.Add(&server.Server{UUID: "srv1"})
	r := NewRouter(mgr, docker.NewStubEnvironment(), "tok", WithBackups(data, bak))
	buatBackup(t, r, "srv1", "b1")

	if err := os.WriteFile(cfg, []byte("rusak"), 0o644); err != nil {
		t.Fatal(err)
	}
	if rec := doCall(t, r, "POST", "/api/servers/srv1/backups/b1/restore", ""); rec.Code != http.StatusAccepted {
		t.Fatalf("restore harusnya 202, dapat %d: %s", rec.Code, rec.Body)
	}

	var status string
	for i := 0; i < 300 && status != "completed"; i++ {
		rec := doCall(t, r, "GET", "/api/servers/srv1/restore", "")
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		status, _ = body["status"].(string)
		time.Sleep(10 * time.Millisecond)
	}
	if status != "completed" {
		t.Fatalf("status restore = %q", status)
	}
	if b, _ := os.ReadFile(cfg); string(b) != "asli" {
		t.Errorf("isi file = %q", b)
	}

	if rec := doCall(t, r, "POST", "/api/servers/srv1/backups/nggak-ada/restore", ""); rec.Code != http.StatusNotFound {
		t.Errorf("backup tak dikenal harusnya 404, dapat %d", rec.Code)
	}
}

func TestRestoreRefusedWhileServerRunning(t *testing.T) {
	data, bak := t.TempDir(), t.TempDir()
	if err := os.MkdirAll(filepath.Join(data, "srv1"), 0o755); err != nil {
		t.Fatal(err)
	}
	mgr := server.NewManager()
	mgr.Add(&server.Server{UUID: "srv1"})
	r := NewRouter(mgr, runningEnv{docker.NewStubEnvironment()}, "tok", WithBackups(data, bak))
	buatBackup(t, r, "srv1", "b1")

	if rec := doCall(t, r, "POST", "/api/servers/srv1/backups/b1/restore", ""); rec.Code != http.StatusConflict {
		t.Fatalf("server jalan harusnya 409, dapat %d: %s", rec.Code, rec.Body)
	}
}
