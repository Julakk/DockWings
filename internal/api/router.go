package api

import (
	"net/http"
	"runtime"

	"github.com/Julakk/DockWings/internal/api/middleware"
	"github.com/Julakk/DockWings/internal/backup"
	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/install"
	"github.com/Julakk/DockWings/internal/server"
	"github.com/Julakk/DockWings/internal/version"
)

type routerConfig struct {
	filesRoot  string
	backupRoot string
	installRun install.Runner
}

// RouterOption ngatur fitur opsional di NewRouter.
type RouterOption func(*routerConfig)

// WithFilesRoot ngaktifin endpoint file manager. root = data_directory
// (folder induk yang isinya <uuid>/ per server).
func WithFilesRoot(root string) RouterOption {
	return func(c *routerConfig) { c.filesRoot = root }
}

// WithBackups ngaktifin endpoint backup. dataRoot = data_directory,
// backupRoot = backup_directory (tempat arsip tar.gz disimpan).
func WithBackups(dataRoot, backupRoot string) RouterOption {
	return func(c *routerConfig) { c.filesRoot, c.backupRoot = dataRoot, backupRoot }
}

// WithInstallRunner ganti cara script install dijalanin (default: Docker). Dipakai test.
func WithInstallRunner(run install.Runner) RouterOption {
	return func(c *routerConfig) { c.installRun = run }
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
	// GET /api/system — info daemon, mirip endpoint /api/system di Wings Pterodactyl.
	// Dipakai Panel buat nampilin versi Wings + cek node online.
	protected.HandleFunc("GET /api/system", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"version":        version.Version,
			"kernel_version": "",
			"architecture":   runtime.GOARCH,
			"os":             runtime.GOOS,
			"cpu_count":      runtime.NumCPU(),
			"servers":        len(mgr.All()),
		})
	})
	protected.HandleFunc("POST /api/servers", h.CreateServer)
	protected.HandleFunc("POST /api/servers/{uuid}/power", h.Power)
	protected.HandleFunc("POST /api/servers/{uuid}/commands", h.SendCommand)
	protected.HandleFunc("DELETE /api/servers/{uuid}", h.DeleteServer)
	protected.HandleFunc("GET /api/servers/{uuid}/resources", h.Resources)
	protected.HandleFunc("PUT /api/servers/{uuid}/allocations", h.UpdateAllocations)

	if cfg.filesRoot != "" {
		var inst *install.Installer
		if cfg.installRun != nil {
			inst = install.NewWithRunner(cfg.filesRoot, cfg.installRun)
		} else {
			inst = install.New(cfg.filesRoot)
		}
		inst.Recover()
		h.Installing = inst.IsInstalling
		ih := &InstallHandlers{
			Manager:   mgr,
			Env:       env,
			Installer: inst,
			Restoring: func(uuid string) bool { return h.Restoring != nil && h.Restoring(uuid) },
		}
		protected.HandleFunc("POST /api/servers/{uuid}/install", ih.Start)
		protected.HandleFunc("GET /api/servers/{uuid}/install", ih.Status)
	}

	if cfg.backupRoot != "" {
		store := backup.New(cfg.filesRoot, cfg.backupRoot)
		restorer := backup.NewRestorer(store)
		restorer.Recover()
		h.Restoring = restorer.IsRestoring
		bh := &BackupHandlers{Manager: mgr, Env: env, Store: store, Restorer: restorer, Installing: h.Installing}
		protected.HandleFunc("POST /api/servers/{uuid}/backups", bh.Create)
		protected.HandleFunc("GET /api/servers/{uuid}/backups/{backup}", bh.Status)
		protected.HandleFunc("GET /api/servers/{uuid}/backups/{backup}/download", bh.Download)
		protected.HandleFunc("DELETE /api/servers/{uuid}/backups/{backup}", bh.Delete)
		protected.HandleFunc("POST /api/servers/{uuid}/backups/{backup}/restore", bh.Restore)
		protected.HandleFunc("GET /api/servers/{uuid}/restore", bh.RestoreStatus)
	}

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
