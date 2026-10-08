package files

import (
	"archive/zip"
	"errors"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

var (
	ErrNeedZip = errors.New("nama arsip harus berakhiran .zip")
	ErrNoInput = errors.New("nggak ada file atau folder yang dipilih")
)

type compressor struct {
	zw    *zip.Writer
	skip  string
	count int
	bytes int64
}

func (c *compressor) add(src, base string) error {
	return filepath.WalkDir(src, func(p string, d fs.DirEntry, werr error) error {
		if werr != nil {
			return werr
		}
		if p == c.skip || d.Type()&fs.ModeSymlink != 0 {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		name := base
		if rel, err := filepath.Rel(src, p); err == nil && rel != "." {
			name = path.Join(base, filepath.ToSlash(rel))
		}
		hdr, err := zip.FileInfoHeader(info)
		if err != nil {
			return err
		}
		if info.IsDir() {
			hdr.Name = name + "/"
			hdr.Method = zip.Store
			_, err = c.zw.CreateHeader(hdr)
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		c.count++
		c.bytes += info.Size()
		if c.count > MaxExtractFiles || c.bytes > MaxExtractBytes {
			return ErrTooBig
		}
		hdr.Name = name
		hdr.Method = zip.Deflate
		w, err := c.zw.CreateHeader(hdr)
		if err != nil {
			return err
		}
		f, err := os.Open(p)
		if err != nil {
			return err
		}
		_, err = io.Copy(w, f)
		f.Close()
		return err
	})
}

// Compress bikin arsip .zip dari file/folder. Symlink dilewati, hasil
// ditulis ke file sementara dulu lalu di-rename, dan nggak menimpa arsip lama.
// Balikannya jumlah file yang dimasukkan.
func (s *Store) Compress(rels []string, destRel string) (int, error) {
	if len(rels) == 0 {
		return 0, ErrNoInput
	}
	if isRoot(destRel) {
		return 0, ErrRoot
	}
	if !strings.HasSuffix(strings.ToLower(destRel), ".zip") {
		return 0, ErrNeedZip
	}
	dst, err := s.resolveNoFollow(destRel)
	if err != nil {
		return 0, err
	}
	if _, err := os.Lstat(dst); err == nil {
		return 0, ErrExists
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return 0, err
	}
	tmp, err := os.CreateTemp(filepath.Dir(dst), ".dockwings-zip-*")
	if err != nil {
		return 0, err
	}
	done := false
	defer func() {
		if !done {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()

	c := &compressor{zw: zip.NewWriter(tmp), skip: tmp.Name()}
	for _, rel := range rels {
		if isRoot(rel) {
			return 0, ErrRoot
		}
		src, err := s.resolve(rel)
		if err != nil {
			return 0, err
		}
		if _, err := os.Stat(src); err != nil {
			return 0, err
		}
		base := path.Base(path.Clean("/" + filepath.ToSlash(rel)))
		if err := c.add(src, base); err != nil {
			return 0, err
		}
	}
	if err := c.zw.Close(); err != nil {
		return 0, err
	}
	if err := tmp.Close(); err != nil {
		return 0, err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return 0, err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		return 0, err
	}
	done = true
	return c.count, nil
}
