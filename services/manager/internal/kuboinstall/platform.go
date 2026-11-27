package kuboinstall

import "fmt"

func platformKey(goos, goarch string) string {
	return goos + "-" + goarch
}

func supportedPlatform(goos, goarch string) bool {
	switch goos {
	case "linux", "darwin", "windows":
	default:
		return false
	}
	switch goarch {
	case "amd64", "arm64":
		return true
	default:
		return false
	}
}

func assetName(version, goos, goarch string) (string, error) {
	if !supportedPlatform(goos, goarch) {
		return "", &Error{Code: CodeUnsupported}
	}
	ext := ".tar.gz"
	if goos == "windows" {
		ext = ".zip"
	}
	return fmt.Sprintf("kubo_v%s_%s-%s%s", version, goos, goarch, ext), nil
}

func exeName(goos string) string {
	if goos == "windows" {
		return "ipfs.exe"
	}
	return "ipfs"
}
