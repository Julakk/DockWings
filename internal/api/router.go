package api

import (
	"net/http"

	"github.com/Julakk/DockWings/internal/api/middleware"
	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
)

type routerConfig struct {
	filesRoot string
}

// RouterOption ngatur fitur opsional di NewRouter.
type RouterOption func(*routerConfig)

// WithFilesRoot ngaktifin endpoint file manager. root = data_directory
// (folder induk yang isinya <uuid>/ per server).
func WithFilesRoot(root string) RouterOption {
	return func(c *routerConfig) { c.filesRoot = root }
}

// NewRouter bikin http.Handler lengkap dengan semua route.
// Sebagian besar route pakai auth header "Authorization: Bearer {token}"
// (middleware.RequireAuth). Route console WebSocket beda: browser nggak
// bisa kirim header custom pas buka koneksi WS, jadi dia diverifikasi
// sendiri lewat token di query param (lihat ConsoleWS).
func NewRouter(mgr *server.Manager, env docker.Environment, authToken string, opts ...RouterOption) http.Handler {
	var cfg routerConfig
	for _, o := range opts {
		o(&cfg)
	}

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

	if cfg.filesRoot != "" {
		fh := &FilesHandlers{Manager: mgr, Root: cfg.filesRoot}
		protected.HandleFunc("GET /api/servers/{uuid}/files/list", fh.List)
		protected.HandleFunc("GET /api/servers/{uuid}/files/contents", fh.Contents)
		protected.HandleFunc("POST /api/servers/{uuid}/files/write", fh.Write)
		protected.HandleFunc("POST /api/servers/{uuid}/files/mkdir", fh.Mkdir)
		protected.HandleFunc("POST /api/servers/{uuid}/files/rename", fh.Rename)
		protected.HandleFunc("POST /api/servers/{uuid}/files/delete", fh.Delete)
	}

	mux := http.NewServeMux()
	mux.Handle("GET /api/servers/{uuid}/ws/console", h.ConsoleWS(authToken))
	mux.Handle("/", middleware.RequireAuth(authToken, protected))

	return mux
}
