package api

import (
	"errors"
	"log"
	"net/http"
	"os"

	"github.com/Julakk/DockWings/internal/backup"
	"github.com/Julakk/DockWings/internal/server"
)

// BackupHandlers nangani endpoint backup server.
type BackupHandlers struct {
	Manager *server.Manager
	Store   *backup.Store
}

func (b *BackupHandlers) serverUUID(w http.ResponseWriter, r *http.Request) (string, bool) {
	uuid := r.PathValue("uuid")
	if !validUUID(uuid) {
		writeError(w, http.StatusBadRequest, "uuid nggak valid")
		return "", false
	}
	if _, err := b.Manager.Get(uuid); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return "", false
	}
	return uuid, true
}

func writeBackupError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, backup.ErrInvalidID):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, backup.ErrNotFound):
		writeError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, backup.ErrBusy):
		writeError(w, http.StatusConflict, err.Error())
	case errors.Is(err, backup.ErrNoData):
		writeError(w, http.StatusUnprocessableEntity, err.Error())
	default:
		log.Printf("operasi backup gagal: %v", err)
		writeError(w, http.StatusInternalServerError, "operasi backup gagal")
	}
}

// POST /api/servers/{uuid}/backups  {"uuid":"<id backup>"}
func (b *BackupHandlers) Create(w http.ResponseWriter, r *http.Request) {
	uuid, ok := b.serverUUID(w, r)
	if !ok {
		return
	}
	var req struct {
		UUID string `json:"uuid"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := b.Store.Start(uuid, req.UUID); err != nil {
		writeBackupError(w, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]any{"uuid": req.UUID, "status": backup.StatusCreating})
}

// GET /api/servers/{uuid}/backups/{backup}
func (b *BackupHandlers) Status(w http.ResponseWriter, r *http.Request) {
	uuid, ok := b.serverUUID(w, r)
	if !ok {
		return
	}
	info, err := b.Store.Get(uuid, r.PathValue("backup"))
	if err != nil {
		writeBackupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, info)
}

// GET /api/servers/{uuid}/backups/{backup}/download
func (b *BackupHandlers) Download(w http.ResponseWriter, r *http.Request) {
	uuid, ok := b.serverUUID(w, r)
	if !ok {
		return
	}
	id := r.PathValue("backup")
	path, err := b.Store.Path(uuid, id)
	if err != nil {
		writeBackupError(w, err)
		return
	}
	f, err := os.Open(path)
	if err != nil {
		writeBackupError(w, backup.ErrNotFound)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil {
		writeBackupError(w, err)
		return
	}

	w.Header().Set("Content-Type", "application/gzip")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, id+".tar.gz", st.ModTime(), f)
}

// DELETE /api/servers/{uuid}/backups/{backup}
func (b *BackupHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	uuid, ok := b.serverUUID(w, r)
	if !ok {
		return
	}
	if err := b.Store.Delete(uuid, r.PathValue("backup")); err != nil {
		writeBackupError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
