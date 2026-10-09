package sftpd

import (
	"errors"
	"io"
	"os"

	"github.com/pkg/sftp"

	"github.com/Julakk/DockWings/internal/files"
)

// handler ngimplementasi 4 interface request-server pkg/sftp di atas Store
// satu server. Semua path lewat Store.Resolve, jadi nggak bisa keluar folder.
type handler struct {
	st *files.Store
	ro bool
}

func newHandlers(st *files.Store, ro bool) sftp.Handlers {
	h := &handler{st: st, ro: ro}
	return sftp.Handlers{FileGet: h, FilePut: h, FileCmd: h, FileList: h}
}

func mapErr(err error) error {
	if errors.Is(err, files.ErrForbidden) || errors.Is(err, files.ErrRoot) {
		return sftp.ErrSSHFxPermissionDenied
	}
	return err
}

func (h *handler) Fileread(r *sftp.Request) (io.ReaderAt, error) {
	p, err := h.st.Resolve(r.Filepath)
	if err != nil {
		return nil, mapErr(err)
	}
	f, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	if !fi.Mode().IsRegular() {
		f.Close()
		return nil, sftp.ErrSSHFxFailure
	}
	return f, nil
}

func (h *handler) Filewrite(r *sftp.Request) (io.WriterAt, error) {
	if h.ro || files.IsRootPath(r.Filepath) {
		return nil, sftp.ErrSSHFxPermissionDenied
	}
	p, err := h.st.Resolve(r.Filepath)
	if err != nil {
		return nil, mapErr(err)
	}
	fl := r.Pflags()
	flag := os.O_WRONLY
	if fl.Read {
		flag = os.O_RDWR
	}
	if fl.Append {
		flag |= os.O_APPEND
	}
	if fl.Creat {
		flag |= os.O_CREATE
	}
	if fl.Trunc {
		flag |= os.O_TRUNC
	}
	if fl.Excl {
		flag |= os.O_EXCL
	}
	f, err := os.OpenFile(p, flag, 0o644)
	if err != nil {
		return nil, err
	}
	return f, nil
}

func (h *handler) Filecmd(r *sftp.Request) error {
	if h.ro {
		return sftp.ErrSSHFxPermissionDenied
	}
	switch r.Method {
	case "Setstat":
		return h.setstat(r)
	case "Rename", "PosixRename":
		return mapErr(h.st.Rename(r.Filepath, r.Target))
	case "Rmdir":
		return h.remove(r.Filepath, true)
	case "Remove":
		return h.remove(r.Filepath, false)
	case "Mkdir":
		if files.IsRootPath(r.Filepath) {
			return sftp.ErrSSHFxPermissionDenied
		}
		p, err := h.st.Resolve(r.Filepath)
		if err != nil {
			return mapErr(err)
		}
		return os.Mkdir(p, 0o755)
	}
	// Link dan Symlink sengaja nggak didukung.
	return sftp.ErrSSHFxOpUnsupported
}

func (h *handler) remove(rel string, dir bool) error {
	if files.IsRootPath(rel) {
		return sftp.ErrSSHFxPermissionDenied
	}
	p, err := h.st.ResolveNoFollow(rel)
	if err != nil {
		return mapErr(err)
	}
	fi, err := os.Lstat(p)
	if err != nil {
		return err
	}
	if dir != fi.IsDir() {
		return sftp.ErrSSHFxFailure
	}
	return os.Remove(p)
}

// setstat cuma ngurus chmod, dan dibatasi ke 0755/0644 (atau di bawahnya):
// nggak ada setuid, sticky, atau write buat group/other. Atribut lain
// (waktu, uid/gid) diabaikan diam-diam supaya klien yang nyalin timestamp
// tetap jalan.
func (h *handler) setstat(r *sftp.Request) error {
	if !r.AttrFlags().Permissions {
		return nil
	}
	if files.IsRootPath(r.Filepath) {
		return sftp.ErrSSHFxPermissionDenied
	}
	p, err := h.st.Resolve(r.Filepath)
	if err != nil {
		return mapErr(err)
	}
	fi, err := os.Stat(p)
	if err != nil {
		return err
	}
	mode := r.Attributes().FileMode().Perm() & 0o755
	if fi.IsDir() {
		mode |= 0o700
	} else {
		mode |= 0o600
	}
	return os.Chmod(p, mode)
}

type listerAt []os.FileInfo

func (l listerAt) ListAt(f []os.FileInfo, offset int64) (int, error) {
	if offset >= int64(len(l)) {
		return 0, io.EOF
	}
	n := copy(f, l[offset:])
	if n < len(f) {
		return n, io.EOF
	}
	return n, nil
}

func (h *handler) Filelist(r *sftp.Request) (sftp.ListerAt, error) {
	switch r.Method {
	case "List":
		p, err := h.st.Resolve(r.Filepath)
		if err != nil {
			return nil, mapErr(err)
		}
		des, err := os.ReadDir(p)
		if err != nil {
			return nil, err
		}
		infos := make([]os.FileInfo, 0, len(des))
		for _, de := range des {
			if fi, err := de.Info(); err == nil {
				infos = append(infos, fi)
			}
		}
		return listerAt(infos), nil
	case "Stat", "Lstat":
		p, err := h.st.Resolve(r.Filepath)
		if err != nil {
			return nil, mapErr(err)
		}
		fi, err := os.Stat(p)
		if err != nil {
			return nil, err
		}
		return listerAt{fi}, nil
	}
	return nil, sftp.ErrSSHFxOpUnsupported
}
