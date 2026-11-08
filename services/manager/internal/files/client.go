// Package files calls Kubo's mutable-file RPC on a loopback address.
// It streams uploads and downloads and does not buffer a whole file.
package files

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuborpc"
)

const (
	// DefaultMaxBytes is the upload limit when none is configured.
	DefaultMaxBytes int64 = 512 << 20
	// DefaultOpTimeout is the metadata deadline.
	DefaultOpTimeout = 20 * time.Second
	// DefaultTransferTimeout is the upload and download deadline.
	DefaultTransferTimeout = 10 * time.Minute

	maxJSON        = 32 << 20
	maxStatEntries = 32
)

// Entry is one mutable-file record.
type Entry struct {
	Name     string
	Type     string
	Size     int64
	CID      string
	Modified time.Time
}

// Options configures a Client. Zero durations and a non-positive MaxBytes
// pick the defaults. BaseURL must be an http loopback origin with a port.
type Options struct {
	BaseURL         string
	MaxBytes        int64
	OpTimeout       time.Duration
	TransferTimeout time.Duration
}

// Client talks to one Kubo RPC.
type Client struct {
	base     *url.URL
	http     *http.Client
	maxBytes int64
	op       time.Duration
	transfer time.Duration
}

// New checks the base URL and builds a client that dials loopback only.
func New(opt Options) (*Client, error) {
	base, err := kuborpc.ParseBase(opt.BaseURL)
	if err != nil {
		return nil, err
	}
	if opt.MaxBytes <= 0 {
		opt.MaxBytes = DefaultMaxBytes
	}
	if opt.OpTimeout <= 0 {
		opt.OpTimeout = DefaultOpTimeout
	}
	if opt.TransferTimeout <= 0 {
		opt.TransferTimeout = DefaultTransferTimeout
	}
	return &Client{
		base:     base,
		http:     kuborpc.Client(),
		maxBytes: opt.MaxBytes,
		op:       opt.OpTimeout,
		transfer: opt.TransferTimeout,
	}, nil
}

// MaxBytes is the configured upload limit.
func (c *Client) MaxBytes() int64 { return c.maxBytes }

// List returns the entries in an absolute folder.
func (c *Client) List(ctx context.Context, mfsPath string) ([]Entry, error) {
	p, err := Clean(mfsPath)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.op)
	defer cancel()
	var out struct {
		Entries []lsEntry `json:"Entries"`
	}
	if err := c.postDecode(ctx, "files/ls", url.Values{"arg": {p}, "long": {"true"}}, &out); err != nil {
		return nil, err
	}
	entries := make([]Entry, 0, len(out.Entries))
	for _, item := range out.Entries {
		if !Segment(item.Name) {
			continue
		}
		entries = append(entries, item.entry())
	}
	c.fillModified(ctx, p, entries)
	return entries, nil
}

// Stat returns one path.
func (c *Client) Stat(ctx context.Context, mfsPath string) (Entry, error) {
	p, err := Clean(mfsPath)
	if err != nil {
		return Entry{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, c.op)
	defer cancel()
	return c.stat(ctx, p)
}

// Mkdir creates one folder. Parents are not created.
func (c *Client) Mkdir(ctx context.Context, mfsPath string) error {
	p, err := Clean(mfsPath)
	if err != nil || p == "/" {
		return invalidPath()
	}
	return c.postEmpty(ctx, "files/mkdir", url.Values{"arg": {p}, "parents": {"false"}})
}

// Write streams r to path. overwrite is required when a file is already there.
// A folder is never replaced. The body is not buffered.
func (c *Client) Write(ctx context.Context, mfsPath string, r io.Reader, overwrite bool) (Entry, error) {
	p, err := Clean(mfsPath)
	if err != nil || p == "/" {
		return Entry{}, invalidPath()
	}
	if err := c.prepareWrite(ctx, p, overwrite); err != nil {
		return Entry{}, err
	}
	writeCtx, cancel := context.WithTimeout(ctx, c.transfer)
	err = c.postFile(writeCtx, p, &maxReader{r: r, max: c.maxBytes})
	cancel()
	if err != nil {
		if CodeOf(err) == "too_large" && !overwrite {
			cleanCtx, cleanCancel := context.WithTimeout(context.WithoutCancel(ctx), c.op)
			_ = c.remove(cleanCtx, p)
			cleanCancel()
		}
		return Entry{}, err
	}
	return c.Stat(ctx, p)
}

// Rename changes the last segment. The file stays in the same folder.
func (c *Client) Rename(ctx context.Context, mfsPath, name string) (string, error) {
	parent, err := Parent(mfsPath)
	if err != nil {
		return "", err
	}
	dest, err := Join(parent, name)
	if err != nil {
		return "", err
	}
	if err := c.Move(ctx, mfsPath, dest); err != nil {
		return "", err
	}
	return dest, nil
}

// Move moves from to the absolute path to.
func (c *Client) Move(ctx context.Context, from, to string) error {
	return c.transferPath(ctx, "files/mv", from, to)
}

// Copy copies from to the absolute path to.
func (c *Client) Copy(ctx context.Context, from, to string) error {
	return c.transferPath(ctx, "files/cp", from, to)
}

// Remove deletes a file or, recursively, a folder. The root is rejected.
func (c *Client) Remove(ctx context.Context, mfsPath string) error {
	p, err := Clean(mfsPath)
	if err != nil || p == "/" {
		return invalidPath()
	}
	ctx, cancel := context.WithTimeout(ctx, c.op)
	defer cancel()
	return c.remove(ctx, p)
}

// Open streams a file. The caller must Close the reader.
// Closing also cancels the transfer deadline.
func (c *Client) Open(ctx context.Context, mfsPath string) (io.ReadCloser, error) {
	p, err := Clean(mfsPath)
	if err != nil || p == "/" {
		return nil, invalidPath()
	}
	ctx, cancel := context.WithTimeout(ctx, c.transfer)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("files/read", url.Values{"arg": {p}}), nil)
	if err != nil {
		cancel()
		return nil, &OpError{Code: "unavailable"}
	}
	resp, err := c.http.Do(req)
	if err != nil {
		cancel()
		return nil, classify(err)
	}
	if resp.StatusCode >= 300 {
		defer func() { _ = resp.Body.Close() }()
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		cancel()
		return nil, kuboResult(resp.StatusCode, buf)
	}
	return &cancelBody{ReadCloser: resp.Body, cancel: cancel}, nil
}

func (c *Client) transferPath(ctx context.Context, cmd, from, to string) error {
	src, err := Clean(from)
	if err != nil || src == "/" {
		return invalidPath()
	}
	dst, err := Clean(to)
	if err != nil || dst == "/" || src == dst {
		return invalidPath()
	}
	st, err := c.Stat(ctx, src)
	if err != nil {
		return err
	}
	if st.Type == "directory" && Inside(src, dst) && dst != src {
		return invalidPath()
	}
	q := url.Values{}
	q.Add("arg", src)
	q.Add("arg", dst)
	return c.postEmpty(ctx, cmd, q)
}

func (c *Client) prepareWrite(ctx context.Context, p string, overwrite bool) error {
	st, err := c.Stat(ctx, p)
	if err != nil {
		if CodeOf(err) == "not_found" {
			return nil
		}
		return err
	}
	if st.Type == "directory" || !overwrite {
		return &OpError{Code: "already_exists"}
	}
	return nil
}

func (c *Client) stat(ctx context.Context, p string) (Entry, error) {
	var out statBody
	if err := c.postDecode(ctx, "files/stat", url.Values{"arg": {p}}, &out); err != nil {
		return Entry{}, err
	}
	name := ""
	if p != "/" {
		name = p[strings.LastIndex(p, "/")+1:]
	}
	return out.entry(name), nil
}

func (c *Client) remove(ctx context.Context, p string) error {
	body, status, err := c.do(ctx, "files/rm", url.Values{"arg": {p}, "recursive": {"true"}}, nil, "")
	if err != nil {
		return err
	}
	if status >= 300 {
		return kuboResult(status, body)
	}
	return nil
}

func (c *Client) postEmpty(ctx context.Context, cmd string, q url.Values) error {
	ctx, cancel := context.WithTimeout(ctx, c.op)
	defer cancel()
	body, status, err := c.do(ctx, cmd, q, nil, "")
	if err != nil {
		return err
	}
	if status >= 300 {
		return kuboResult(status, body)
	}
	return nil
}

func (c *Client) postDecode(ctx context.Context, cmd string, q url.Values, dest any) error {
	body, status, err := c.do(ctx, cmd, q, nil, "")
	if err != nil {
		return err
	}
	if status >= 300 {
		return kuboResult(status, body)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return &OpError{Code: "kubo_error"}
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return &OpError{Code: "kubo_error"}
	}
	return nil
}

func (c *Client) postFile(ctx context.Context, mfsPath string, r io.Reader) error {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()
	errCh := make(chan error, 1)
	go func() {
		part, err := mw.CreateFormFile("file", pathBase(mfsPath))
		if err == nil {
			if _, copyErr := io.Copy(part, r); copyErr != nil {
				err = copyErr
			}
		}
		if err == nil {
			err = mw.Close()
		}
		if err != nil {
			_ = pw.CloseWithError(err)
		} else {
			_ = pw.Close()
		}
		errCh <- err
	}()
	q := url.Values{"arg": {mfsPath}, "create": {"true"}, "truncate": {"true"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("files/write", q), pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		partErr := <-errCh
		if CodeOf(partErr) == "too_large" {
			return partErr
		}
		return &OpError{Code: "unavailable"}
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.http.Do(req)
	if err != nil {
		_ = pr.CloseWithError(err)
		partErr := <-errCh
		if CodeOf(partErr) == "too_large" {
			return partErr
		}
		return classify(err)
	}
	defer func() { _ = resp.Body.Close() }()
	partErr := <-errCh
	buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if CodeOf(partErr) == "too_large" {
		return partErr
	}
	if resp.StatusCode >= 300 {
		return kuboResult(resp.StatusCode, buf)
	}
	if partErr != nil {
		return classify(partErr)
	}
	return nil
}

func (c *Client) do(ctx context.Context, cmd string, q url.Values, body io.Reader, contentType string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(cmd, q), body)
	if err != nil {
		return nil, 0, &OpError{Code: "unavailable"}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, 0, classify(err)
	}
	defer func() { _ = resp.Body.Close() }()
	buf, err := io.ReadAll(io.LimitReader(resp.Body, maxJSON+1))
	if err != nil {
		return nil, resp.StatusCode, classify(err)
	}
	if len(buf) > maxJSON {
		return nil, resp.StatusCode, &OpError{Code: "too_large"}
	}
	return buf, resp.StatusCode, nil
}

func (c *Client) endpoint(cmd string, q url.Values) string {
	u := c.base.ResolveReference(&url.URL{Path: "/api/v0/" + cmd, RawQuery: q.Encode()})
	return u.String()
}

func (c *Client) fillModified(ctx context.Context, dir string, entries []Entry) {
	if len(entries) == 0 || len(entries) > maxStatEntries {
		return
	}
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	var mu sync.Mutex
	for i := range entries {
		if !entries[i].Modified.IsZero() {
			continue
		}
		name := entries[i].Name
		wg.Add(1)
		go func(i int, name string) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			child, cancel := context.WithTimeout(ctx, 3*time.Second)
			defer cancel()
			full := "/" + name
			if dir != "/" {
				full = dir + "/" + name
			}
			st, err := c.stat(child, full)
			if err != nil || st.Modified.IsZero() {
				return
			}
			mu.Lock()
			entries[i].Modified = st.Modified
			if entries[i].CID == "" {
				entries[i].CID = st.CID
			}
			mu.Unlock()
		}(i, name)
	}
	wg.Wait()
}

type lsEntry struct {
	Name       string `json:"Name"`
	Type       int    `json:"Type"`
	Size       int64  `json:"Size"`
	Hash       string `json:"Hash"`
	Mtime      int64  `json:"Mtime"`
	MtimeNsecs int64  `json:"MtimeNsecs"`
}

func (e lsEntry) entry() Entry {
	kind := "file"
	if e.Type == 1 {
		kind = "directory"
	}
	return Entry{
		Name:     e.Name,
		Type:     kind,
		Size:     e.Size,
		CID:      e.Hash,
		Modified: unixTime(e.Mtime, e.MtimeNsecs),
	}
}

type statBody struct {
	Hash       string `json:"Hash"`
	Size       int64  `json:"Size"`
	Type       string `json:"Type"`
	Mtime      int64  `json:"Mtime"`
	MtimeNsecs int64  `json:"MtimeNsecs"`
}

func (s statBody) entry(name string) Entry {
	kind := "file"
	if strings.EqualFold(s.Type, "directory") {
		kind = "directory"
	}
	return Entry{
		Name:     name,
		Type:     kind,
		Size:     s.Size,
		CID:      s.Hash,
		Modified: unixTime(s.Mtime, s.MtimeNsecs),
	}
}

func unixTime(sec, nsec int64) time.Time {
	if sec <= 0 && nsec <= 0 {
		return time.Time{}
	}
	return time.Unix(sec, nsec).UTC()
}

type maxReader struct {
	r   io.Reader
	n   int64
	max int64
}

func (m *maxReader) Read(p []byte) (int, error) {
	if m.n > m.max {
		return 0, &OpError{Code: "too_large"}
	}
	if m.n == m.max {
		var b [1]byte
		n, err := m.r.Read(b[:])
		if n > 0 {
			m.n++
			return 0, &OpError{Code: "too_large"}
		}
		return 0, err
	}
	allow := m.max - m.n
	if int64(len(p)) > allow {
		p = p[:allow]
	}
	n, err := m.r.Read(p)
	m.n += int64(n)
	return n, err
}

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
	once   sync.Once
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.once.Do(b.cancel)
	return err
}

func classify(err error) error {
	if err == nil {
		return nil
	}
	if CodeOf(err) != "" {
		return err
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return &OpError{Code: "timeout"}
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return &OpError{Code: "timeout"}
	}
	if kuborpc.IsRefused(err) {
		return &OpError{Code: "node_not_running"}
	}
	return &OpError{Code: "unavailable"}
}

func kuboResult(status int, body []byte) error {
	if status == http.StatusNotFound {
		return &OpError{Code: "not_found"}
	}
	msg := strings.ToLower(kuboMessage(body))
	switch {
	case strings.Contains(msg, "already exists"):
		return &OpError{Code: "already_exists"}
	case strings.Contains(msg, "does not exist"), strings.Contains(msg, "not found"), strings.Contains(msg, "no such"):
		return &OpError{Code: "not_found"}
	case strings.Contains(msg, "not a directory"):
		return &OpError{Code: "invalid_path"}
	default:
		return &OpError{Code: "kubo_error"}
	}
}

func kuboMessage(body []byte) string {
	var wrap struct {
		Message string `json:"Message"`
	}
	if json.Unmarshal(body, &wrap) == nil && strings.TrimSpace(wrap.Message) != "" {
		return wrap.Message
	}
	return strings.TrimSpace(string(body))
}

func pathBase(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
