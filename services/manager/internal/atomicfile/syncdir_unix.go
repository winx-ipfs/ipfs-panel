//go:build !windows

package atomicfile

import "os"

// syncDir makes the rename durable by fsyncing the containing directory.
func syncDir(dir string) error {
	dirFile, err := os.Open(dir)
	if err != nil {
		return err
	}
	syncErr := dirFile.Sync()
	closeErr := dirFile.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}
