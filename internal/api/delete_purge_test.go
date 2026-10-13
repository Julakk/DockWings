package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
)

const deleteTestUUID = "3c61fc7f-f524-4176-9c1f-592c81059d42"

type deleteFixture struct {
	router http.Handler
	mgr    *server.Manager
	dir    string
}

func newDeleteFixture(t *testing.T, uuid string) deleteFixture {
	t.Helper()
	data, bak := t.TempDir(), t.TempDir()
	dir := filepath.Join(data, uuid)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "server.cfg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	mgr := server.NewManager()
	mgr.Add(&server.Server{UUID: uuid, Image: "img:1"})
	r := NewRouter(mgr, docker.NewStubEnvironment(), "tok", WithBackups(data, bak))
	return deleteFixture{router: r, mgr: mgr, dir: dir}
}

func (f deleteFixture) delete(uuid, query string) (int, map[string]any) {
	req := httptest.NewRequest("DELETE", "/api/servers/"+uuid+query, nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	f.router.ServeHTTP(rec, req)
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return rec.Code, body
}

func TestDeleteServerWithoutPurgeKeepsData(t *testing.T) {
	f := newDeleteFixture(t, deleteTestUUID)
	code, body := f.delete(deleteTestUUID, "")
	if code != http.StatusOK || body["purged"] != false {
		t.Fatalf("code=%d body=%v, mau 200 dan purged=false", code, body)
	}
	if _, err := os.Stat(f.dir); err != nil {
		t.Errorf("folder data harusnya tetap ada tanpa purge: %v", err)
	}
	if _, err := f.mgr.Get(deleteTestUUID); err == nil {
		t.Error("server harusnya sudah keluar dari manager")
	}
}

func TestDeleteServerPurgeRemovesData(t *testing.T) {
	f := newDeleteFixture(t, deleteTestUUID)
	code, body := f.delete(deleteTestUUID, "?purge=1")
	if code != http.StatusOK || body["purged"] != true {
		t.Fatalf("code=%d body=%v, mau 200 dan purged=true", code, body)
	}
	if _, err := os.Stat(f.dir); !os.IsNotExist(err) {
		t.Errorf("folder data harusnya terhapus: %v", err)
	}
	if _, err := f.mgr.Get(deleteTestUUID); err == nil {
		t.Error("server harusnya sudah keluar dari manager")
	}
}

func TestDeleteServerPurgeMissingFolderIsOK(t *testing.T) {
	f := newDeleteFixture(t, deleteTestUUID)
	if err := os.RemoveAll(f.dir); err != nil {
		t.Fatal(err)
	}
	code, body := f.delete(deleteTestUUID, "?purge=true")
	if code != http.StatusOK || body["purged"] != false {
		t.Fatalf("code=%d body=%v, mau 200 dan purged=false", code, body)
	}
}

func TestDeleteServerPurgeRejectsNonCanonicalUUID(t *testing.T) {
	f := newDeleteFixture(t, "korban")
	code, _ := f.delete("korban", "?purge=1")
	if code != http.StatusBadRequest {
		t.Fatalf("code=%d, mau 400", code)
	}
	if _, err := f.mgr.Get("korban"); err != nil {
		t.Errorf("server harusnya tetap tercatat setelah ditolak: %v", err)
	}
	if _, err := os.Stat(f.dir); err != nil {
		t.Errorf("folder harusnya tidak tersentuh: %v", err)
	}
}
