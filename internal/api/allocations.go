package api

import (
	"fmt"
	"log"
	"net"
	"net/http"

	"github.com/Julakk/DockWings/internal/docker"
	"github.com/Julakk/DockWings/internal/server"
)

// validAllocations mastiin daftar allocation dari Panel masuk akal sebelum dipakai.
func validAllocations(list []server.Allocation) error {
	if len(list) == 0 {
		return fmt.Errorf("minimal harus ada satu allocation")
	}
	seen := map[string]bool{}
	for _, a := range list {
		if a.Port < 1 || a.Port > 65535 {
			return fmt.Errorf("port %d nggak valid", a.Port)
		}
		if a.IP != "" && net.ParseIP(a.IP) == nil {
			return fmt.Errorf("ip %q nggak valid", a.IP)
		}
		key := fmt.Sprintf("%s:%d", a.IP, a.Port)
		if seen[key] {
			return fmt.Errorf("allocation %s dobel", key)
		}
		seen[key] = true
	}
	return nil
}

// PUT /api/servers/{uuid}/allocations  {"allocations":[{"ip","port","primary"}]}
// Simpan allocation baru, lalu bikin ulang container supaya port mapping dan
// SERVER_IP/SERVER_PORT ikut berubah. Kalau server lagi jalan, dia direstart.
func (h *Handlers) UpdateAllocations(w http.ResponseWriter, r *http.Request) {
	uuid := r.PathValue("uuid")
	s, err := h.Manager.Get(uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	var req struct {
		Allocations []server.Allocation `json:"allocations"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := validAllocations(req.Allocations); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.Manager.Update(uuid, func(s *server.Server) { s.Allocations = req.Allocations }); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	rc, ok := h.Env.(docker.Reconfigurer)
	if !ok {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "applied": false, "restarted": false})
		return
	}

	restarted, err := rc.Reconfigure(r.Context(), s)
	if err != nil {
		log.Printf("reconfigure %s gagal: %v", uuid, err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "applied": true, "restarted": restarted})
}
