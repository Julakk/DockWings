package docker

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"syscall"

	"github.com/Julakk/DockWings/internal/server"
)

// SetUser nentuin uid:gid yang dipakai proses di dalam container (ngikutin
// konsep user `pterodactyl` di Wings). uid <= 0 = fitur mati (perilaku lama).
func (e *DockerEnvironment) SetUser(uid, gid int) {
	if gid <= 0 {
		gid = uid
	}
	e.uid, e.gid = uid, gid
}

// userSpec balikin "uid:gid" buat `docker create --user`, atau "" kalau mati.
func (e *DockerEnvironment) userSpec() string {
	if e.uid <= 0 {
		return ""
	}
	return fmt.Sprintf("%d:%d", e.uid, e.gid)
}

// ensureOwnership dipanggil sebelum container dinyalakan: folder data server
// diserahkan ke user container, supaya file hasil install (root) dan upload
// lewat panel/SFTP bisa ditulis server. Container lama yang dibuat tanpa
// --user sengaja nggak disentuh, biar server yang sudah jalan nggak rusak.
func (e *DockerEnvironment) ensureOwnership(ctx context.Context, s *server.Server) error {
	want := e.userSpec()
	if want == "" {
		return nil
	}
	cur, err := run(ctx, "inspect", "-f", "{{.Config.User}}", s.ContainerName())
	if err != nil || cur != want {
		return nil
	}
	return ChownTree(filepath.Join(e.dataDir, s.UUID), e.uid, e.gid)
}

// ChownTree chown rekursif tanpa ngikutin symlink (Lchown). Entri yang sudah
// benar dilewati. Lanjut walau satu entri gagal, error pertama dikembalikan.
func ChownTree(root string, uid, gid int) error {
	var first error
	note := func(err error) {
		if err != nil && first == nil {
			first = err
		}
	}

	note(filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			note(err)
			return nil
		}
		info, err := d.Info()
		if err != nil {
			note(err)
			return nil
		}
		if st, ok := info.Sys().(*syscall.Stat_t); ok && int(st.Uid) == uid && int(st.Gid) == gid {
			return nil
		}
		note(os.Lchown(path, uid, gid))
		return nil
	}))

	return first
}
