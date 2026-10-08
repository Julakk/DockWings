package api

import (
	"errors"
	"net/http"

	"github.com/Julakk/DockWings/internal/files"
)

// POST /api/servers/{uuid}/files/compress  {"paths":["/plugins"],"dest":"/plugins.zip"}
func (f *FilesHandlers) Compress(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	var req struct {
		Paths []string `json:"paths"`
		Dest  string   `json:"dest"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if len(req.Paths) > 100 {
		writeError(w, http.StatusBadRequest, "kebanyakan item (maks 100)")
		return
	}
	n, err := st.Compress(req.Paths, req.Dest)
	if err != nil {
		switch {
		case errors.Is(err, files.ErrNeedZip), errors.Is(err, files.ErrNoInput), errors.Is(err, files.ErrTooBig):
			writeError(w, http.StatusUnprocessableEntity, err.Error())
		default:
			writeFileError(w, err)
		}
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "files": n})
}

// POST /api/servers/{uuid}/files/chmod  {"path":"/samp03svr","executable":true}
func (f *FilesHandlers) Chmod(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	var req struct {
		Path       string `json:"path"`
		Executable bool   `json:"executable"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := st.SetExecutable(req.Path, req.Executable); err != nil {
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
