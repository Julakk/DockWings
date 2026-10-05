package api

import (
	"net/http"

	"github.com/Julakk/DockWings/internal/backup"
)

// POST /api/servers/{uuid}/backups/{backup}/restore
// Ganti seluruh isi folder data server dengan isi backup. Server harus mati.
func (b *BackupHandlers) Restore(w http.ResponseWriter, r *http.Request) {
	uuid, ok := b.serverUUID(w, r)
	if !ok {
		return
	}

	s, err := b.Manager.Get(uuid)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	if b.Installing != nil && b.Installing(uuid) {
		writeError(w, http.StatusConflict, "install sedang berjalan, tunggu selesai")
		return
	}
	if res, err := b.Env.Resources(r.Context(), s); err == nil &&
		(res.State == "running" || res.State == "starting") {
		writeError(w, http.StatusConflict, "matikan server dulu sebelum restore backup")
		return
	}

	id := r.PathValue("backup")
	if err := b.Restorer.Start(uuid, id); err != nil {
		writeBackupError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"status": backup.RestoreRunning, "backup": id})
}

// GET /api/servers/{uuid}/restore
func (b *BackupHandlers) RestoreStatus(w http.ResponseWriter, r *http.Request) {
	uuid, ok := b.serverUUID(w, r)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, b.Restorer.State(uuid))
}
