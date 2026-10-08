package files

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"errors"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const (
	// MaxExtractBytes = batas total ukuran hasil ekstrak (anti zip bomb).
	MaxExtractBytes int64 = 1 << 30
	// MaxExtractFiles = batas jumlah file per arsip.
	MaxExtractFiles = 20000
)

var (
	ErrUnsupported = errors.New("format arsip nggak didukung (cuma .zip, .tar, .tar.gz, .tgz; .rar belum bisa)")
	ErrTooBig      = errors.New("isi arsip kegedean atau file-nya kebanyakan")
	ErrBadArchive  = errors.New("arsip rusak atau nggak valid")
)

type extractor struct {
	st      *Store
	destRel string
	bytes   int64
	count   int
}

// cleanEntry nolak nama entry yang mencoba keluar folder (zip-slip).
// skip=true buat entry kosong seperti "./".
func cleanEntry(name string) (clean string, skip bool, err error) {
	if strings.ContainsRune(name, 0) {
		return "", false, ErrForbidden
	}
	n := strings.ReplaceAll(name, "\\", "/")
	for _, seg := range strings.Split(n, "/") {
		if seg == ".." {
			return "", false, ErrForbidden
		}
	}
	c := path.Clean("/" + n)
	if c == "/" {
		return "", true, nil
	}
	return c, false, nil
}

func (e *extractor) dir(clean string) error {
	p, err := e.st.resolve(path.Join(e.destRel, clean))
	if err != nil {
		return err
	}
	return os.MkdirAll(p, 0o755)
}

func (e *extractor) file(clean string, mode os.FileMode, r io.Reader) error {
	e.count++
	if e.count > MaxExtractFiles {
		return ErrTooBig
	}
	p, err := e.st.resolve(path.Join(e.destRel, clean))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	perm := os.FileMode(0o644)
	if mode&0o111 != 0 {
		perm = 0o755
	}
	out, err := os.OpenFile(p, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
	if err != nil {
		return err
	}
	remaining := MaxExtractBytes - e.bytes
	n, err := io.Copy(out, io.LimitReader(r, remaining+1))
	e.bytes += n
	if cerr := out.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return err
	}
	if n > remaining {
		return ErrTooBig
	}
	return nil
}

func (e *extractor) zip(src string) error {
	zr, err := zip.OpenReader(src)
	if err != nil {
		return ErrBadArchive
	}
	defer zr.Close()

	var total uint64
	for _, f := range zr.File {
		total += f.UncompressedSize64
	}
	if total > uint64(MaxExtractBytes) || len(zr.File) > MaxExtractFiles {
		return ErrTooBig
	}

	for _, f := range zr.File {
		clean, skip, err := cleanEntry(f.Name)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		mode := f.Mode()
		switch {
		case f.FileInfo().IsDir():
			err = e.dir(clean)
		case mode.IsRegular():
			rc, oerr := f.Open()
			if oerr != nil {
				return ErrBadArchive
			}
			err = e.file(clean, mode, rc)
			rc.Close()
		default:
			continue // symlink dan sejenisnya dilewati
		}
		if err != nil {
			return err
		}
	}
	return nil
}

func (e *extractor) tar(src string, gz bool) error {
	f, err := os.Open(src)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f
	if gz {
		gr, err := gzip.NewReader(f)
		if err != nil {
			return ErrBadArchive
		}
		defer gr.Close()
		r = gr
	}

	tr := tar.NewReader(r)
	for {
		h, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return ErrBadArchive
		}
		clean, skip, err := cleanEntry(h.Name)
		if err != nil {
			return err
		}
		if skip {
			continue
		}
		switch h.Typeflag {
		case tar.TypeDir:
			err = e.dir(clean)
		case tar.TypeReg:
			err = e.file(clean, h.FileInfo().Mode(), tr)
		default:
			continue // symlink, hardlink, device dilewati
		}
		if err != nil {
			return err
		}
	}
}

// Extract ngeluarin isi arsip ke folder yang sama dengan arsipnya.
// Balikannya jumlah file yang diekstrak.
func (s *Store) Extract(rel string) (int, error) {
	if isRoot(rel) {
		return 0, ErrRoot
	}
	src, err := s.resolve(rel)
	if err != nil {
		return 0, err
	}
	fi, err := os.Stat(src)
	if err != nil {
		return 0, err
	}
	if fi.IsDir() {
		return 0, ErrIsDir
	}

	clean := path.Clean("/" + filepath.ToSlash(rel))
	e := &extractor{st: s, destRel: path.Dir(clean)}
	lower := strings.ToLower(clean)
	switch {
	case strings.HasSuffix(lower, ".zip"):
		err = e.zip(src)
	case strings.HasSuffix(lower, ".tar.gz"), strings.HasSuffix(lower, ".tgz"):
		err = e.tar(src, true)
	case strings.HasSuffix(lower, ".tar"):
		err = e.tar(src, false)
	default:
		return 0, ErrUnsupported
	}
	return e.count, err
}
