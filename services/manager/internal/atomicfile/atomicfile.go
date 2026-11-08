// Package atomicfile writes a file by renaming a fully written temporary file
// over the destination. The temporary file only appears under its final name
// after its bytes are on disk, so a crash never leaves a half-written
// credential file or pin index behind.
package atomicfile

import (
	"os"
	"path/filepath"
)

// Write replaces path with data. It creates the parent directory with mode
// 0700 and sets mode on the result. A temporary file is removed when any step
// fails.
func Write(path string, data []byte, mode os.FileMode) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Rename(tmpName, path); err != nil {
		// Windows refuses to rename over a destination that another
		// handle still has open. Removing it first still leaves the
		// new bytes arriving whole, which is what this package promises.
		_ = os.Remove(path)
		if err2 := os.Rename(tmpName, path); err2 != nil {
			return err2
		}
	}
	cleanup = false
	if err := os.Chmod(path, mode); err != nil {
		return err
	}
	return syncDir(dir)
}
