package backup

import (
	"archive/tar"
	"compress/gzip"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// tarDir ecrit le contenu d'un repertoire dans une archive tar, compressee si
// gzip est demande. Les chemins sont relatifs au repertoire : l'archive se
// deploie n'importe ou.
func tarDir(dir, out string, compress bool) error {
	f, err := os.Create(out)
	if err != nil {
		return err
	}
	defer f.Close()

	var w io.Writer = f
	var gz *gzip.Writer
	if compress {
		gz = gzip.NewWriter(f)
		w = gz
	}
	tw := tar.NewWriter(w)

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if rel == "." {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		// Pas de liens symboliques dans les pieces jointes ; s'il y en a, on
		// les ignore plutot que d'archiver ce qu'ils pointent.
		if info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if d.IsDir() {
			header.Name += "/"
		}
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		src, err := os.Open(path)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(tw, src)
		return err
	})
	if err != nil {
		return err
	}

	if err := tw.Close(); err != nil {
		return err
	}
	if gz != nil {
		if err := gz.Close(); err != nil {
			return err
		}
	}
	return f.Close()
}

// untar deploie une archive tar, compressee ou non, dans un repertoire. Un
// chemin qui sortirait du repertoire est refuse : une archive n'est pas
// forcement la notre.
func untar(archive, dir string, compressed bool) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()

	var r io.Reader = f
	if compressed {
		gz, err := gzip.NewReader(f)
		if err != nil {
			return err
		}
		defer gz.Close()
		r = gz
	}
	tr := tar.NewReader(r)

	root, err := filepath.Abs(dir)
	if err != nil {
		return err
	}
	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		target := filepath.Join(root, filepath.FromSlash(header.Name))
		if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
			return fmt.Errorf("chemin hors de l'archive : %s", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.MkdirAll(target, 0o750); err != nil {
				return err
			}
		case tar.TypeReg:
			if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
				return err
			}
			dst, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, fs.FileMode(header.Mode)&0o777|0o600)
			if err != nil {
				return err
			}
			if _, err := io.Copy(dst, tr); err != nil {
				dst.Close()
				return err
			}
			if err := dst.Close(); err != nil {
				return err
			}
		}
	}
}
