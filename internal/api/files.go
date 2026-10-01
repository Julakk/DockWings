package api

import (
	"encoding/json"
	"errors"
	"io/fs"
	"log"
	"net/http"

	"github.com/Julakk/DockWings/internal/files"
	"github.com/Julakk/DockWings/internal/server"
)

const (
	maxUploadBytes = 100 << 20 // 100 MB per file
	maxJSONBytes   = 1 << 20
)

// FilesHandlers nangani endpoint file manager. Root = data_directory dari config.
type FilesHandlers struct {
	Manager *server.Manager
	Root    string
}

func (f *FilesHandlers) store(w http.ResponseWriter, r *http.Request) (*files.Store, bool) {
	uuid := r.PathValue("uuid")
	if !validUUID(uuid) {
		writeError(w, http.StatusBadRequest, "uuid nggak valid")
		return nil, false
	}
	if _, err := f.Manager.Get(uuid); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return nil, false
	}
	return files.New(f.Root, uuid), true
}

// writeFileError sengaja nggak bocorin path host ke client.
func writeFileError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, files.ErrForbidden), errors.Is(err, files.ErrRoot):
		writeError(w, http.StatusForbidden, err.Error())
	case errors.Is(err, fs.ErrNotExist):
		writeError(w, http.StatusNotFound, "file atau folder nggak ketemu")
	case errors.Is(err, files.ErrIsDir), errors.Is(err, files.ErrNotDir):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, files.ErrExists):
		writeError(w, http.StatusConflict, err.Error())
	default:
		log.Printf("operasi file gagal: %v", err)
		writeError(w, http.StatusInternalServerError, "operasi file gagal")
	}
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxJSONBytes)
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, "body request nggak valid")
		return false
	}
	return true
}

// GET /api/servers/{uuid}/files/list?path=/
func (f *FilesHandlers) List(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	p := r.URL.Query().Get("path")
	entries, err := st.List(p)
	if err != nil {
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"path": p, "entries": entries})
}

// GET /api/servers/{uuid}/files/contents?path=/server.cfg
func (f *FilesHandlers) Contents(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	file, info, err := st.Open(r.URL.Query().Get("path"))
	if err != nil {
		writeFileError(w, err)
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	http.ServeContent(w, r, info.Name(), info.ModTime(), file)
}

// POST /api/servers/{uuid}/files/write?path=/server.cfg  (body = isi file mentah)
func (f *FilesHandlers) Write(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	n, err := st.Write(r.URL.Query().Get("path"), r.Body)
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "file kegedean")
			return
		}
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "size": n})
}

// POST /api/servers/{uuid}/files/mkdir  {"path":"/plugins"}
func (f *FilesHandlers) Mkdir(w http.ResponseWriter, r *http.Request) {
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
	if err := st.Mkdir(req.Path); err != nil {
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// POST /api/servers/{uuid}/files/rename  {"from":"/a","to":"/b"}
func (f *FilesHandlers) Rename(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	var req struct {
		From string `json:"from"`
		To   string `json:"to"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if err := st.Rename(req.From, req.To); err != nil {
		writeFileError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

// POST /api/servers/{uuid}/files/delete  {"paths":["/a.txt","/plugins"]}
func (f *FilesHandlers) Delete(w http.ResponseWriter, r *http.Request) {
	st, ok := f.store(w, r)
	if !ok {
		return
	}
	var req struct {
		Paths []string `json:"paths"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	for _, p := range req.Paths {
		if err := st.Delete(p); err != nil {
			writeFileError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}
