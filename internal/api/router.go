package api

import (
	"net/http"

	"github.com/Julakk/DockWings/internal/api/middleware"
	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
)

// NewRouter bikin http.Handler lengkap dengan semua route + middleware auth.
// Pakai net/http.ServeMux bawaan Go 1.22+ yang udah support method + path
// pattern (contoh "POST /api/servers/{uuid}/power"), jadi belum butuh
// router pihak ketiga (chi/gorilla) buat sekarang.
func NewRouter(mgr *server.Manager, env docker.Environment, authToken string) http.Handler {
	h := NewHandlers(mgr, env)

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	mux.HandleFunc("POST /api/servers", h.CreateServer)
	mux.HandleFunc("POST /api/servers/{uuid}/power", h.Power)
	mux.HandleFunc("POST /api/servers/{uuid}/commands", h.SendCommand)
	mux.HandleFunc("DELETE /api/servers/{uuid}", h.DeleteServer)

	return middleware.RequireAuth(authToken, mux)
}
