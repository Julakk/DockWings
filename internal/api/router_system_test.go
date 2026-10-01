package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
	"github.com/Julakk/DockWings/internal/version"
)

func TestSystemEndpoint(t *testing.T) {
	r := NewRouter(server.NewManager(), docker.NewStubEnvironment(), "tok")

	req := httptest.NewRequest("GET", "/api/system", nil)
	req.Header.Set("Authorization", "Bearer tok")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, mau 200", rec.Code)
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	if body["version"] != version.Version {
		t.Errorf("version = %v, mau %s", body["version"], version.Version)
	}

	req = httptest.NewRequest("GET", "/api/system", nil)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Errorf("tanpa token harusnya 401, dapat %d", rec.Code)
	}
}
