package api

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/install"
	"github.com/Julakk/DockWings/internal/server"
)

// InstallHandlers nangani endpoint install / reinstall server.
type InstallHandlers struct {
	Manager   *server.Manager
	Env       docker.Environment
	Installer *install.Installer

	// Restoring (opsional) nolak install selama restore backup berjalan.
	Restoring func(uuid string) bool
}

type installRequest struct {
	Script       string         `json:"script"`
	Container    string         `json:"container"`
	EnvVariables map[string]any `json:"env_variables"`
}

// POST /api/servers/{uuid}/install — sesuai WingsService::startInstall() di Panel.
// Jalanin script install egg di container sementara (background, balas 202).
// Server harus mati, kalau nggak balas 409.
func (ih *InstallHandlers) Start(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	s, err := ih.Manager.Get(uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, 2<<20)
	dec := json.NewDecoder(r.Body)
	dec.UseNumber()
	var req installRequest
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "body request nggak valid: "+err.Error())
		return
	}

	env, err := stringifyEnv(req.EnvVariables)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	image := req.Container
	if image == "" {
		image = s.Image
	}

	if res, err := ih.Env.Resources(r.Context(), s); err == nil &&
		(res.State == "running" || res.State == "starting") {
		writeError(w, http.StatusConflict, "matikan server dulu sebelum install ulang")
		return
	}
	if ih.Restoring != nil && ih.Restoring(uuid) {
		writeError(w, http.StatusConflict, "restore backup sedang berjalan, tunggu selesai")
		return
	}

	err = ih.Installer.Start(uuid, install.Spec{Script: req.Script, Image: image, Env: env})
	switch {
	case errors.Is(err, install.ErrBusy):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, install.ErrInvalid):
		writeError(w, http.StatusBadRequest, err.Error())
	case err != nil:
		writeError(w, http.StatusInternalServerError, "gagal mulai install: "+err.Error())
	default:
		writeJSON(w, http.StatusAccepted, map[string]any{"status": install.Running})
	}
}

// GET /api/servers/{uuid}/install — status install terakhir
// (idle | running | completed | failed).
func (ih *InstallHandlers) Status(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	if _, err := ih.Manager.Get(uuid); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, ih.Installer.State(uuid))
}

// stringifyEnv ngubah env_variables dari Panel jadi string semua
// (PHP kadang ngirim angka atau boolean).
func stringifyEnv(in map[string]any) (map[string]string, error) {
	out := make(map[string]string, len(in))
	for k, v := range in {
		switch t := v.(type) {
		case nil:
			out[k] = ""
		case string:
			out[k] = t
		case json.Number:
			out[k] = t.String()
		case bool:
			out[k] = strconv.FormatBool(t)
		default:
			return nil, fmt.Errorf("env_variables.%s harus string, angka, atau boolean", k)
		}
	}
	return out, nil
}
