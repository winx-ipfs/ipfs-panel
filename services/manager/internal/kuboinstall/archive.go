package kuboinstall

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
)

const maxBinary = 256 << 20

// extractBinary unpacks kubo/<exe> from a tar.gz or zip archive. Any member
// that escapes the destination, and a binary that is a symlink, is refused.
// Other members (license, readme, install.sh) are ignored and never run.
func extractBinary(archivePath, dest string, zipArchive bool, exe string) error {
	if err := os.RemoveAll(dest); err != nil {
		return err
	}
	if err := os.MkdirAll(dest, 0o700); err != nil {
		return err
	}
	var err error
	if zipArchive {
		err = extractZip(archivePath, dest, exe)
	} else {
		err = extractTar(archivePath, dest, exe)
	}
	if err != nil {
		_ = os.RemoveAll(dest)
		return err
	}
	return nil
}

func wanted(name, exe string) (bool, error) {
	name = path.Clean(strings.TrimPrefix(name, "./"))
	name = strings.TrimPrefix(name, "/")
	if name == ".." || strings.HasPrefix(name, "../") || path.IsAbs(name) {
		return false, fmt.Errorf("archive path escapes")
	}
	return name == "kubo/"+exe, nil
}

func extractTar(archivePath, dest, exe string) error {
	f, err := os.Open(archivePath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	found := false
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		ok, err := wanted(hdr.Name, exe)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if hdr.Typeflag == tar.TypeSymlink || hdr.Typeflag == tar.TypeLink {
			return fmt.Errorf("archive binary is a link")
		}
		if hdr.Typeflag != tar.TypeReg && hdr.Typeflag != tar.TypeRegA && hdr.Typeflag != 0 {
			return fmt.Errorf("archive binary is not a file")
		}
		if hdr.Size < 0 || hdr.Size > maxBinary {
			return fmt.Errorf("archive binary is too large")
		}
		if err := writeExe(dest, exe, io.LimitReader(tr, hdr.Size+1), hdr.Size); err != nil {
			return err
		}
		found = true
	}
	if !found {
		return fmt.Errorf("archive has no kubo binary")
	}
	return nil
}

func extractZip(archivePath, dest, exe string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()
	found := false
	for _, file := range r.File {
		ok, err := wanted(file.Name, exe)
		if err != nil {
			return err
		}
		if !ok {
			continue
		}
		if file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("archive binary is a link")
		}
		if file.UncompressedSize64 > maxBinary {
			return fmt.Errorf("archive binary is too large")
		}
		rc, err := file.Open()
		if err != nil {
			return err
		}
		err = writeExe(dest, exe, io.LimitReader(rc, int64(file.UncompressedSize64)+1), int64(file.UncompressedSize64))
		_ = rc.Close()
		if err != nil {
			return err
		}
		found = true
	}
	if !found {
		return fmt.Errorf("archive has no kubo binary")
	}
	return nil
}

func writeExe(dest, exe string, r io.Reader, claimed int64) error {
	out := filepath.Join(dest, exe)
	f, err := os.OpenFile(out, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o755)
	if err != nil {
		return err
	}
	n, err := io.Copy(f, r)
	if closeErr := f.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(out)
		return err
	}
	if claimed >= 0 && n != claimed {
		_ = os.Remove(out)
		return fmt.Errorf("archive binary changed size")
	}
	if n == 0 || n > maxBinary {
		_ = os.Remove(out)
		return fmt.Errorf("archive binary is empty or too large")
	}
	return os.Chmod(out, 0o755)
}
