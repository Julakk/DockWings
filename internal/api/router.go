package api

import (
	"net/http"

	"github.com/Julakk/DockWings/internal/api/middleware"
	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
)

// NewRouter bikin http.Handler lengkap dengan semua route.
// Sebagian besar route pakai auth header "Authorization: Bearer {token}"
// (middleware.RequireAuth). Route console WebSocket beda: browser nggak
// bisa kirim header custom pas buka koneksi WS, jadi dia diverifikasi
// sendiri lewat token di query param (lihat ConsoleWS).
func NewRouter(mgr *server.Manager, env docker.Environment, authToken string) http.Handler {
	h := NewHandlers(mgr, env)

	protected := http.NewServeMux()
	protected.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	protected.HandleFunc("POST /api/servers", h.CreateServer)
	protected.HandleFunc("POST /api/servers/{uuid}/power", h.Power)
	protected.HandleFunc("POST /api/servers/{uuid}/commands", h.SendCommand)
	protected.HandleFunc("DELETE /api/servers/{uuid}", h.DeleteServer)
	protected.HandleFunc("GET /api/servers/{uuid}/resources", h.Resources)

	mux := http.NewServeMux()
	mux.Handle("GET /api/servers/{uuid}/ws/console", h.ConsoleWS(authToken))
	mux.Handle("/", middleware.RequireAuth(authToken, protected))

	return mux
}
