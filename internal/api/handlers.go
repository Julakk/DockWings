package api

import (
	"encoding/json"
	"net/http"

	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
)

// Handlers nyimpen dependency yang dibutuhin semua endpoint API.
type Handlers struct {
	Manager *server.Manager
	Env     docker.Environment

	// Restoring (opsional) dipakai buat nolak start server yang lagi di-restore.
	Restoring func(uuid string) bool
}

func NewHandlers(mgr *server.Manager, env docker.Environment) *Handlers {
	return &Handlers{Manager: mgr, Env: env}
}

type createServerRequest struct {
	UUID      string `json:"uuid"`
	Container struct {
		Image          string            `json:"image"`
		StartupCommand string            `json:"startup_command"`
		EnvVariables   map[string]string `json:"env_variables"`
	} `json:"container"`
	Build struct {
		MemoryLimit int64   `json:"memory_limit"`
		Swap        int64   `json:"swap"`
		IOWeight    int64   `json:"io_weight"`
		CPULimit    float64 `json:"cpu_limit"`
		DiskSpace   int64   `json:"disk_space"`
	} `json:"build"`
	Allocations []server.Allocation `json:"allocations"`
}

// POST /api/servers — sesuai WingsService::createServer()
func (h *Handlers) CreateServer(w http.ResponseWriter, r *http.Request) {
	var req createServerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body request nggak valid: "+err.Error())
		return
	}

	if !validUUID(req.UUID) {
		writeError(w, http.StatusBadRequest, "uuid nggak valid")
		return
	}

	if !validImage(req.Container.Image) {
		writeError(w, http.StatusBadRequest, "image nggak valid")
		return
	}

	s := &server.Server{
		UUID:            req.UUID,
		Image:           req.Container.Image,
		StartupCommand:  req.Container.StartupCommand,
		EnvVariables:    req.Container.EnvVariables,
		Allocations:     req.Allocations,
		MemoryLimitMB:   req.Build.MemoryLimit,
		SwapMB:          req.Build.Swap,
		IOWeight:        req.Build.IOWeight,
		CPULimitPercent: req.Build.CPULimit,
		DiskSpaceMB:     req.Build.DiskSpace,
		Status:          server.StatusInstalling,
	}

	h.Manager.Add(s)

	if err := h.Env.Create(r.Context(), s); err != nil {
		writeError(w, http.StatusInternalServerError, "gagal create container: "+err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"uuid":   s.UUID,
		"status": s.Status,
	})
}

type powerRequest struct {
	Action string `json:"action"` // start | stop | restart | kill
}

// POST /api/servers/{uuid}/power — sesuai WingsService::power()
func (h *Handlers) Power(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")

	s, err := h.Manager.Get(uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req powerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body request nggak valid: "+err.Error())
		return
	}

	if (req.Action == "start" || req.Action == "restart") && h.Restoring != nil && h.Restoring(uuid) {
		writeError(w, http.StatusConflict, "restore backup sedang berjalan, tunggu selesai")
		return
	}

	ctx := r.Context()
	var actionErr error

	switch req.Action {
	case "start":
		s.Status = server.StatusStarting
		actionErr = h.Env.Start(ctx, s)
	case "stop":
		s.Status = server.StatusStopping
		actionErr = h.Env.Stop(ctx, s)
	case "restart":
		actionErr = h.Env.Stop(ctx, s)
		if actionErr == nil {
			actionErr = h.Env.Start(ctx, s)
		}
	case "kill":
		actionErr = h.Env.Kill(ctx, s)
	default:
		writeError(w, http.StatusBadRequest, "action nggak dikenal: "+req.Action)
		return
	}

	if actionErr != nil {
		writeError(w, http.StatusInternalServerError, actionErr.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type commandRequest struct {
	Commands []string `json:"commands"`
}

// POST /api/servers/{uuid}/commands — sesuai WingsService::sendCommand()
func (h *Handlers) SendCommand(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")

	s, err := h.Manager.Get(uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req commandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body request nggak valid: "+err.Error())
		return
	}

	for _, cmd := range req.Commands {
		if err := h.Env.SendCommand(r.Context(), s, cmd); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// DELETE /api/servers/{uuid} — sesuai WingsService::delete()
func (h *Handlers) DeleteServer(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")

	s, err := h.Manager.Get(uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	if err := h.Env.Remove(r.Context(), s); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.Manager.Remove(uuid)

	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}

// GET /api/servers/{uuid}/resources — dipanggil App\Services\ServerResourceService di Panel
func (h *Handlers) Resources(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")

	s, err := h.Manager.Get(uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	res, err := h.Env.Resources(r.Context(), s)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"current_state": res.State,
		"utilization": map[string]any{
			"cpu_absolute": res.CPUAbsolute,
			"memory_bytes": res.MemoryBytes,
			"disk_bytes":   res.DiskBytes,
		},
	})
}
