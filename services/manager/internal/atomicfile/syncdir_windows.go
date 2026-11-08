//go:build windows

package atomicfile

// syncDir is a no-op on Windows. A directory handle cannot be passed to
// FlushFileBuffers, and opening one for fsync is refused with
// ERROR_ACCESS_DENIED. NTFS journals the rename itself, which is the
// durability this package needs.
func syncDir(string) error { return nil }
