package api

import (
	"errors"
	"net/http"

	"github.com/Julakk/DockWings/internal/files"
)

// POST /api/servers/{uuid}/files/extract  {"path":"/bot.zip"}
// Isi arsip diekstrak ke folder yang sama dengan arsipnya.
func (f *FilesHandlers) Extract(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	var req struct {
		Path string `json:"path"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	n, err := st.Extract(req.Path)
	if err != nil {
		switch {
		case errors.Is(err, files.ErrUnsupported), errors.Is(err, files.ErrBadArchive), errors.Is(err, files.ErrTooBig):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeFileError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "files": n})
}
