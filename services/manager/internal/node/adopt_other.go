//go:build !linux

package node

// sameDaemon is Linux-only. Other systems cannot check /proc cmdline and
// environ, so a live pid is not adopted. A held repo lock becomes repo_busy
// instead of attaching to a Kubo the panel cannot prove it started.
func sameDaemon(int, string, string) bool { return false }
