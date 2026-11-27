package kuboinstall

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	maxArchive   = 256 << 20
	sidecarLimit = 4096
	catalogLimit = 1 << 20
)

type sumFetch struct {
	sum    string
	netErr bool
}

func (s *Service) distFile(version, name string) string {
	return strings.TrimRight(s.distBase, "/") + "/kubo/v" + version + "/" + name
}

func (s *Service) ghFile(version, name string) string {
	return strings.TrimRight(s.githubBase, "/") + "/v" + version + "/" + name
}

func (s *Service) sidecar(ctx context.Context, rawURL string) sumFetch {
	ctx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	body, err := s.getLimited(ctx, rawURL, sidecarLimit)
	if err != nil {
		return sumFetch{netErr: isNet(err)}
	}
	sum, err := parseSHA512(body)
	if err != nil {
		return sumFetch{}
	}
	return sumFetch{sum: sum}
}

// decideHash applies the pin rule or the two-origin rule from the spec.
func decideHash(pin string, dist, github sumFetch) (string, *Error) {
	if pin != "" {
		if (dist.sum != "" && dist.sum != pin) || (github.sum != "" && github.sum != pin) {
			return "", errCode(CodeVerify)
		}
		if dist.sum == pin || github.sum == pin {
			return pin, nil
		}
		if dist.netErr && github.netErr {
			return "", errCode(CodeOffline)
		}
		return "", errCode(CodeVerify)
	}
	if dist.sum != "" && github.sum != "" && dist.sum == github.sum {
		return dist.sum, nil
	}
	if dist.netErr && github.netErr {
		return "", errCode(CodeOffline)
	}
	return "", errCode(CodeVerify)
}

func (s *Service) download(ctx context.Context, version string, progress func(got, total int64)) (string, string, error) {
	name, err := assetName(version, s.goos, s.goarch)
	if err != nil {
		return "", "", err
	}
	pin := ""
	if version == Pinned {
		pin = s.pin()
		if pin == "" {
			return "", "", errCode(CodeUnsupported)
		}
	}
	expect, derr := decideHash(pin, s.sidecar(ctx, s.distFile(version, name+".sha512")), s.sidecar(ctx, s.ghFile(version, name+".sha512")))
	if derr != nil {
		return "", "", derr
	}
	urls := []string{s.distFile(version, name), s.ghFile(version, name)}
	sawNet := false
	sawVerify := false
	for _, rawURL := range urls {
		path, ferr := s.saveArchive(ctx, rawURL, expect, progress)
		if ferr == nil {
			return path, expect, nil
		}
		var ke *Error
		if errors.As(ferr, &ke) {
			if ke.Code == CodeOffline {
				sawNet = true
			} else {
				sawVerify = true
			}
			continue
		}
		if isNet(ferr) {
			sawNet = true
			continue
		}
		sawVerify = true
	}
	if sawVerify {
		return "", "", errCode(CodeVerify)
	}
	if sawNet {
		return "", "", errCode(CodeOffline)
	}
	return "", "", errCode(CodeVerify)
}

func (s *Service) saveArchive(ctx context.Context, rawURL, expect string, progress func(got, total int64)) (string, error) {
	if err := os.MkdirAll(stagingDir(s.dataDir), 0o700); err != nil {
		return "", err
	}
	id, err := newID()
	if err != nil {
		return "", err
	}
	dest := filepath.Join(stagingDir(s.dataDir), id+".partial")
	f, err := os.OpenFile(dest, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return "", err
	}
	cleanup := true
	defer func() {
		_ = f.Close()
		if cleanup {
			_ = os.Remove(dest)
		}
	}()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ipfs-panel")
	res, err := s.http.Do(req)
	if err != nil {
		if isNet(err) {
			return "", errCode(CodeOffline)
		}
		return "", errCode(CodeVerify)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		if res.StatusCode >= 500 {
			return "", errCode(CodeOffline)
		}
		return "", errCode(CodeVerify)
	}
	total := res.ContentLength
	if total > maxArchive {
		return "", errCode(CodeVerify)
	}
	if total < 0 {
		total = 0
	}
	if progress != nil {
		progress(0, total)
	}
	written, err := copyCap(f, res.Body, maxArchive, func(n int64) {
		if progress != nil {
			progress(n, total)
		}
	})
	if err != nil {
		return "", err
	}
	if err := f.Close(); err != nil {
		return "", err
	}
	sum, err := hashFile(dest)
	if err != nil {
		return "", err
	}
	if sum != expect || written == 0 {
		return "", errCode(CodeVerify)
	}
	cleanup = false
	return dest, nil
}

func copyCap(dst io.Writer, src io.Reader, max int64, progress func(int64)) (int64, error) {
	buf := make([]byte, 32*1024)
	var n int64
	for {
		nr, er := src.Read(buf)
		if nr > 0 {
			if n+int64(nr) > max {
				return n, errCode(CodeVerify)
			}
			nw, ew := dst.Write(buf[:nr])
			n += int64(nw)
			if progress != nil {
				progress(n)
			}
			if ew != nil {
				return n, ew
			}
			if nw != nr {
				return n, io.ErrShortWrite
			}
		}
		if er == io.EOF {
			return n, nil
		}
		if er != nil {
			if isNet(er) {
				return n, errCode(CodeOffline)
			}
			return n, er
		}
	}
}

func (s *Service) getLimited(ctx context.Context, rawURL string, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ipfs-panel")
	req.Header.Set("Accept", "application/vnd.github+json, text/plain, */*")
	res, err := s.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, errors.New("status")
	}
	return io.ReadAll(io.LimitReader(res.Body, limit+1))
}

func (s *Service) fetchCatalog(ctx context.Context) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	body, err := s.getLimited(ctx, s.versionsURL, catalogLimit)
	if err == nil && int64(len(body)) <= catalogLimit {
		if lines := parseVersionLines(body); len(lines) > 0 {
			return lines, nil
		}
	}
	ctx, cancel = context.WithTimeout(ctx, 4*time.Second)
	defer cancel()
	body, err = s.getLimited(ctx, s.githubAPI, catalogLimit)
	if err != nil {
		return nil, err
	}
	return parseGitHubReleases(body)
}

func parseVersionLines(body []byte) []string {
	var out []string
	for _, line := range strings.Split(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}

func parseGitHubReleases(body []byte) ([]string, error) {
	var rels []struct {
		Tag        string `json:"tag_name"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.Unmarshal(body, &rels); err != nil {
		return nil, err
	}
	out := make([]string, 0, len(rels))
	for _, rel := range rels {
		if rel.Draft || rel.Prerelease {
			continue
		}
		out = append(out, rel.Tag)
	}
	return out, nil
}

func isNet(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return true
	}
	var ne net.Error
	if errors.As(err, &ne) {
		return true
	}
	var ue *url.Error
	return errors.As(err, &ue)
}

func newID() (string, error) {
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(buf[:]), nil
}
