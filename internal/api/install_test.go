package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/install"
	"github.com/Julakk/DockWings/internal/server"
)

func installRouter(t *testing.T, env docker.Environment, run install.Runner) http.Handler {
	t.Helper()
	data, bak := t.TempDir(), t.TempDir()
	mgr := server.NewManager()
	mgr.Add(&server.Server{UUID: "srv1", Image: "ghcr.io/x/y:latest"})
	return NewRouter(mgr, env, "tok", WithBackups(data, bak), WithInstallRunner(run))
}

func installStatus(t *testing.T, r http.Handler, uuid string) map[string]any {
	t.Helper()
	rec := doCall(t, r, "GET", "/api/servers/"+uuid+"/install", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("status install: %d %s", rec.Code, rec.Body)
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}

func waitInstall(t *testing.T, r http.Handler, uuid, want string) map[string]any {
	t.Helper()
	for i := 0; i < 300; i++ {
		if b := installStatus(t, r, uuid); b["status"] == want {
			return b
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("install nggak jadi %q", want)
	return nil
}

func TestInstallEndpoint(t *testing.T) {
	var got install.Job
	r := installRouter(t, docker.NewStubEnvironment(), func(_ context.Context, job install.Job) (string, error) {
		got = job
		return "ok", nil
	})

	if b := installStatus(t, r, "srv1"); b["status"] != "idle" {
		t.Fatalf("status awal = %v", b["status"])
	}

	body := `{"script":"echo hi","container":"alpine:3","env_variables":{"A":"x","N":5,"B":true,"Z":null}}`
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", body); rec.Code != http.StatusAccepted {
		t.Fatalf("harusnya 202, dapat %d: %s", rec.Code, rec.Body)
	}
	waitInstall(t, r, "srv1", "completed")

	if got.Script != "echo hi" || got.Image != "alpine:3" {
		t.Errorf("job = %+v", got)
	}
	want := map[string]string{"A": "x", "N": "5", "B": "true", "Z": ""}
	for k, v := range want {
		if got.Env[k] != v {
			t.Errorf("env[%s] = %q, mau %q", k, got.Env[k], v)
		}
	}
}

func TestInstallFallsBackToServerImage(t *testing.T) {
	var got install.Job
	r := installRouter(t, docker.NewStubEnvironment(), func(_ context.Context, job install.Job) (string, error) {
		got = job
		return "", nil
	})
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"true"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("dapat %d: %s", rec.Code, rec.Body)
	}
	waitInstall(t, r, "srv1", "completed")
	if got.Image != "ghcr.io/x/y:latest" {
		t.Errorf("image = %q", got.Image)
	}
}

func TestInstallFailureReported(t *testing.T) {
	r := installRouter(t, docker.NewStubEnvironment(), func(context.Context, install.Job) (string, error) {
		return "baris terakhir", os.ErrPermission
	})
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"exit 1","container":"alpine"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("dapat %d", rec.Code)
	}
	b := waitInstall(t, r, "srv1", "failed")
	if s, _ := b["error"].(string); s == "" {
		t.Error("error harusnya terisi")
	}
	if s, _ := b["log"].(string); !strings.Contains(s, "baris terakhir") {
		t.Errorf("log = %v", b["log"])
	}
}

func TestInstallRefusals(t *testing.T) {
	noop := func(context.Context, install.Job) (string, error) { return "", nil }

	r := installRouter(t, runningEnv{docker.NewStubEnvironment()}, noop)
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"true","container":"alpine"}`); rec.Code != http.StatusConflict {
		t.Errorf("server jalan harusnya 409, dapat %d", rec.Code)
	}

	r = installRouter(t, docker.NewStubEnvironment(), noop)
	if rec := doCall(t, r, "POST", "/api/servers/nggak-ada/install", `{"script":"true"}`); rec.Code != http.StatusNotFound {
		t.Errorf("server tak dikenal harusnya 404, dapat %d", rec.Code)
	}
	if rec := doCall(t, r, "GET", "/api/servers/nggak-ada/install", ""); rec.Code != http.StatusNotFound {
		t.Errorf("status server tak dikenal harusnya 404, dapat %d", rec.Code)
	}
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{`); rec.Code != http.StatusBadRequest {
		t.Errorf("body rusak harusnya 400, dapat %d", rec.Code)
	}
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"   ","container":"alpine"}`); rec.Code != http.StatusBadRequest {
		t.Errorf("script kosong harusnya 400, dapat %d", rec.Code)
	}
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"true","container":"alpine","env_variables":{"A":[1]}}`); rec.Code != http.StatusBadRequest {
		t.Errorf("env bukan skalar harusnya 400, dapat %d", rec.Code)
	}
}

func TestInstallBlocksPowerDeleteAndRestore(t *testing.T) {
	release := make(chan struct{})
	r := installRouter(t, docker.NewStubEnvironment(), func(context.Context, install.Job) (string, error) {
		<-release
		return "", nil
	})
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"true","container":"alpine"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("dapat %d", rec.Code)
	}

	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"true","container":"alpine"}`); rec.Code != http.StatusConflict {
		t.Errorf("install kedua harusnya 409, dapat %d", rec.Code)
	}
	if rec := doCall(t, r, "POST", "/api/servers/srv1/power", `{"action":"start"}`); rec.Code != http.StatusConflict {
		t.Errorf("start saat install harusnya 409, dapat %d", rec.Code)
	}
	if rec := doCall(t, r, "POST", "/api/servers/srv1/power", `{"action":"stop"}`); rec.Code != http.StatusOK {
		t.Errorf("stop saat install harusnya tetap boleh, dapat %d", rec.Code)
	}
	if rec := doCall(t, r, "DELETE", "/api/servers/srv1", ""); rec.Code != http.StatusConflict {
		t.Errorf("hapus saat install harusnya 409, dapat %d", rec.Code)
	}
	if rec := doCall(t, r, "POST", "/api/servers/srv1/backups/b1/restore", ""); rec.Code != http.StatusConflict {
		t.Errorf("restore saat install harusnya 409, dapat %d", rec.Code)
	}

	close(release)
	waitInstall(t, r, "srv1", "completed")
	if rec := doCall(t, r, "POST", "/api/servers/srv1/power", `{"action":"start"}`); rec.Code != http.StatusOK {
		t.Errorf("start setelah install harusnya 200, dapat %d", rec.Code)
	}
}

func TestInstallCreatesDataDir(t *testing.T) {
	data, bak := t.TempDir(), t.TempDir()
	mgr := server.NewManager()
	mgr.Add(&server.Server{UUID: "srv1"})
	r := NewRouter(mgr, docker.NewStubEnvironment(), "tok", WithBackups(data, bak),
		WithInstallRunner(func(context.Context, install.Job) (string, error) { return "", nil }))
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"true","container":"alpine"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("dapat %d", rec.Code)
	}
	if st, err := os.Stat(filepath.Join(data, "srv1")); err != nil || !st.IsDir() {
		t.Error("folder data server harusnya dibuat")
	}
	waitInstall(t, r, "srv1", "completed")
}

func TestInstallExitCodeInStatus(t *testing.T) {
	r := installRouter(t, docker.NewStubEnvironment(), func(context.Context, install.Job) (string, error) {
		return "boom", fmt.Errorf("script install gagal: %w", exec.Command("sh", "-c", "exit 3").Run())
	})
	if rec := doCall(t, r, "POST", "/api/servers/srv1/install", `{"script":"exit 3","container":"alpine"}`); rec.Code != http.StatusAccepted {
		t.Fatalf("dapat %d", rec.Code)
	}
	b := waitInstall(t, r, "srv1", "failed")
	if code, ok := b["exit_code"].(float64); !ok || code != 3 {
		t.Errorf("exit_code = %v, mau 3", b["exit_code"])
	}
}
