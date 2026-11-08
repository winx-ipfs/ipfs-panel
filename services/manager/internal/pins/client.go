package pins

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/winx-ipfs/ipfs-panel/services/manager/internal/kuborpc"
)

const (
	defaultMaxBytes int64 = 512 << 20
	defaultOp             = 20 * time.Second
	defaultTransfer       = 10 * time.Minute
	maxJSON               = 32 << 20
	maxPins               = 20000
	maxSizeLookups        = 32
	sizeBudget            = 8 * time.Second
)

// Pin is one recursive or direct pin.
type Pin struct {
	CID  string
	Name string
	Type string
	Size *int64
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

// Support remembers whether this Kubo accepts pin names.
// A nil Support never sends the name arguments.
type Support struct {
	mu       sync.Mutex
	lsKnown  bool
	lsOK     bool
	addKnown bool
	addOK    bool
}

// New checks the base URL and builds a client that dials loopback only.
func New(opt Options) (*Client, error) {
	base, err := kuborpc.ParseBase(opt.BaseURL)
	if err != nil {
		return nil, err
	}
	if opt.MaxBytes <= 0 {
		opt.MaxBytes = defaultMaxBytes
	}
	if opt.OpTimeout <= 0 {
		opt.OpTimeout = defaultOp
	}
	if opt.TransferTimeout <= 0 {
		opt.TransferTimeout = defaultTransfer
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

// NamesFrom reports "kubo" after a successful names listing, otherwise "panel".
func (s *Support) NamesFrom() string {
	if s == nil {
		return "panel"
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lsKnown && s.lsOK {
		return "kubo"
	}
	return "panel"
}

func (s *Support) wantLSNames() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lsKnown {
		return s.lsOK
	}
	return true
}

func (s *Support) wantAddName() bool {
	if s == nil {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.addKnown {
		return s.addOK
	}
	return true
}

func (s *Support) markLS(ok bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.lsKnown, s.lsOK = true, ok
	s.mu.Unlock()
}

func (s *Support) markAdd(ok bool) {
	if s == nil {
		return
	}
	s.mu.Lock()
	s.addKnown, s.addOK = true, ok
	s.mu.Unlock()
}

// List returns recursive and direct pins. Indirect pins are dropped.
func (c *Client) List(ctx context.Context, support *Support) ([]Pin, error) {
	ctx, cancel := context.WithTimeout(ctx, c.op)
	defer cancel()
	out := make([]Pin, 0)
	for _, kind := range []string{"recursive", "direct"} {
		pins, err := c.listKind(ctx, kind, support)
		if err != nil {
			return nil, err
		}
		out = append(out, pins...)
		if len(out) > maxPins {
			return nil, &OpError{Code: "too_large"}
		}
	}
	return out, nil
}

func (c *Client) listKind(ctx context.Context, kind string, support *Support) ([]Pin, error) {
	withNames := support.wantLSNames()
	pins, err := c.ls(ctx, kind, withNames)
	if err != nil && withNames && CodeOf(err) == "names_unsupported" {
		support.markLS(false)
		return c.ls(ctx, kind, false)
	}
	if err == nil && withNames {
		support.markLS(true)
	}
	return pins, err
}

func (c *Client) ls(ctx context.Context, kind string, withNames bool) ([]Pin, error) {
	q := url.Values{"type": {kind}, "stream": {"true"}}
	if withNames {
		q.Set("names", "true")
	}
	resp, err := c.open(ctx, "pin/ls", q, nil, "")
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return nil, kuboResult(resp.StatusCode, buf)
	}
	dec := json.NewDecoder(&maxRead{r: resp.Body, max: maxJSON})
	out := make([]Pin, 0)
	for {
		var line lsLine
		if err := dec.Decode(&line); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if CodeOf(err) == "too_large" {
				return nil, err
			}
			return nil, &OpError{Code: "kubo_error"}
		}
		out = append(out, line.pins(kind)...)
		if len(out) > maxPins {
			return nil, &OpError{Code: "too_large"}
		}
	}
	return out, nil
}

// Add pins target. onProgress may be nil. The caller owns the deadline.
func (c *Client) Add(ctx context.Context, target, name string, recursive bool, onProgress func(int), support *Support) (string, error) {
	if name != "" && support.wantAddName() {
		cid, err := c.pinAdd(ctx, target, name, recursive, onProgress)
		if CodeOf(err) == "name_unsupported" {
			support.markAdd(false)
			return c.pinAdd(ctx, target, "", recursive, onProgress)
		}
		if err == nil {
			support.markAdd(true)
		}
		return cid, err
	}
	return c.pinAdd(ctx, target, "", recursive, onProgress)
}

func (c *Client) pinAdd(ctx context.Context, target, name string, recursive bool, onProgress func(int)) (string, error) {
	q := url.Values{
		"arg":       {target},
		"recursive": {strconv.FormatBool(recursive)},
		"progress":  {"true"},
	}
	if name != "" {
		q.Set("name", name)
	}
	resp, err := c.open(ctx, "pin/add", q, nil, "")
	if err != nil {
		return "", err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode >= 300 {
		buf, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", kuboResult(resp.StatusCode, buf)
	}
	dec := json.NewDecoder(resp.Body)
	var cid string
	for {
		var ev pinAddEvent
		if err := dec.Decode(&ev); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			if ctx.Err() != nil {
				return "", classify(ctx.Err())
			}
			return "", classify(err)
		}
		if ev.Message != "" && (ev.Type == "error" || ev.Code != 0) {
			raw, _ := json.Marshal(ev)
			return "", kuboResult(http.StatusInternalServerError, raw)
		}
		if ev.Progress > 0 && onProgress != nil {
			onProgress(ev.Progress)
		}
		if len(ev.Pins) > 0 && ev.Pins[0] != "" {
			cid = ev.Pins[0]
		}
	}
	if cid == "" && !strings.HasPrefix(target, "/") {
		return target, nil
	}
	if cid == "" {
		return "", &OpError{Code: "kubo_error"}
	}
	return cid, nil
}

// RememberName asks Kubo to store a label. Unsupported names are not an error.
// The caller should bound the context. A failure still leaves the panel label.
func (c *Client) RememberName(ctx context.Context, cid, name string, support *Support) error {
	if cid == "" || name == "" || !support.wantAddName() {
		return nil
	}
	_, err := c.pinAdd(ctx, cid, name, true, nil)
	if CodeOf(err) == "name_unsupported" {
		support.markAdd(false)
		return nil
	}
	return err
}

// Remove unpins one CID of the given type, "recursive" or "direct".
func (c *Client) Remove(ctx context.Context, cid, kind string) error {
	if kind != "recursive" && kind != "direct" {
		return &OpError{Code: "bad_request"}
	}
	ctx, cancel := context.WithTimeout(ctx, c.op)
	defer cancel()
	q := url.Values{"arg": {cid}, "recursive": {"true"}}
	if kind == "direct" {
		q.Set("recursive", "false")
	}
	body, status, err := c.do(ctx, "pin/rm", q, nil, "")
	if err != nil {
		return err
	}
	if status >= 300 {
		return kuboResult(status, body)
	}
	return nil
}

// Upload streams r to ipfs add and pins it. The body is not buffered.
func (c *Client) Upload(ctx context.Context, filename string, r io.Reader) (Pin, error) {
	ctx, cancel := context.WithTimeout(ctx, c.transfer)
	defer cancel()
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	contentType := mw.FormDataContentType()
	errCh := make(chan error, 1)
	go func() {
		part, err := mw.CreateFormFile("file", filename)
		if err == nil {
			_, err = io.Copy(part, &maxRead{r: r, max: c.maxBytes})
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
	q := url.Values{
		"pin":         {"true"},
		"cid-version": {"1"},
		"hash":        {"sha2-256"},
		"quieter":     {"true"},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint("add", q), pr)
	if err != nil {
		_ = pr.CloseWithError(err)
		partErr := <-errCh
		if CodeOf(partErr) == "too_large" {
			return Pin{}, partErr
		}
		return Pin{}, &OpError{Code: "unavailable"}
	}
	req.Header.Set("Content-Type", contentType)
	resp, err := c.http.Do(req)
	if err != nil {
		_ = pr.CloseWithError(err)
		partErr := <-errCh
		if CodeOf(partErr) == "too_large" {
			return Pin{}, partErr
		}
		return Pin{}, classify(err)
	}
	defer func() { _ = resp.Body.Close() }()
	partErr := <-errCh
	if CodeOf(partErr) == "too_large" {
		return Pin{}, partErr
	}
	buf, readErr := io.ReadAll(io.LimitReader(resp.Body, maxJSON+1))
	if readErr != nil {
		return Pin{}, classify(readErr)
	}
	if len(buf) > maxJSON {
		return Pin{}, &OpError{Code: "too_large"}
	}
	if resp.StatusCode >= 300 {
		return Pin{}, kuboResult(resp.StatusCode, buf)
	}
	if partErr != nil {
		return Pin{}, classify(partErr)
	}
	pin, err := parseAdd(buf)
	if err != nil {
		return Pin{}, err
	}
	pin.Name = filename
	pin.Type = "recursive"
	return pin, nil
}

// FillSizes sets CumulativeSize for the first 32 pins when object/stat answers
// inside the shared budget. Misses leave Size nil.
func (c *Client) FillSizes(ctx context.Context, pins []Pin) {
	n := len(pins)
	if n == 0 {
		return
	}
	if n > maxSizeLookups {
		n = maxSizeLookups
	}
	ctx, cancel := context.WithTimeout(ctx, sizeBudget)
	defer cancel()
	var wg sync.WaitGroup
	sem := make(chan struct{}, 4)
	var mu sync.Mutex
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			select {
			case <-ctx.Done():
				return
			case sem <- struct{}{}:
			}
			defer func() { <-sem }()
			child, stop := context.WithTimeout(ctx, 3*time.Second)
			size, err := c.cumulative(child, pins[i].CID)
			stop()
			if err != nil {
				return
			}
			mu.Lock()
			pins[i].Size = &size
			mu.Unlock()
		}(i)
	}
	wg.Wait()
}

func (c *Client) cumulative(ctx context.Context, cid string) (int64, error) {
	var out struct {
		CumulativeSize int64 `json:"CumulativeSize"`
	}
	if err := c.postDecode(ctx, "object/stat", url.Values{"arg": {cid}}, &out); err != nil {
		return 0, err
	}
	if out.CumulativeSize < 0 {
		return 0, &OpError{Code: "kubo_error"}
	}
	return out.CumulativeSize, nil
}

func parseAdd(buf []byte) (Pin, error) {
	dec := json.NewDecoder(strings.NewReader(string(buf)))
	var pin Pin
	saw := false
	for {
		var ev struct {
			Hash string   `json:"Hash"`
			Size flexSize `json:"Size"`
		}
		if err := dec.Decode(&ev); err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return Pin{}, &OpError{Code: "kubo_error"}
		}
		if ev.Hash != "" {
			size := int64(ev.Size)
			pin = Pin{CID: ev.Hash, Size: &size}
			saw = true
		}
	}
	if !saw || pin.CID == "" {
		return Pin{}, &OpError{Code: "kubo_error"}
	}
	return pin, nil
}

type flexSize int64

func (s *flexSize) UnmarshalJSON(raw []byte) error {
	text := strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return nil
	}
	if text[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err != nil {
			return err
		}
		n, err := strconv.ParseInt(str, 10, 64)
		if err != nil {
			return err
		}
		*s = flexSize(n)
		return nil
	}
	n, err := strconv.ParseInt(text, 10, 64)
	if err != nil {
		return err
	}
	*s = flexSize(n)
	return nil
}

type lsLine struct {
	Keys map[string]lsType `json:"Keys"`
	Cid  string            `json:"Cid"`
	Type string            `json:"Type"`
	Name string            `json:"Name"`
	List *struct {
		Keys map[string]lsType `json:"Keys"`
	} `json:"PinLsList"`
	Object *struct {
		Cid  string `json:"Cid"`
		Type string `json:"Type"`
		Name string `json:"Name"`
	} `json:"PinLsObject"`
}

type lsType struct {
	Type string `json:"Type"`
	Name string `json:"Name"`
}

func (line lsLine) pins(requested string) []Pin {
	var out []Pin
	seen := map[string]bool{}
	add := func(cid, kind, name string) {
		kind = strings.ToLower(kind)
		if cid == "" || (kind != "recursive" && kind != "direct") {
			return
		}
		if requested != "" && kind != requested {
			return
		}
		key := cid + "\n" + kind
		if seen[key] {
			return
		}
		seen[key] = true
		out = append(out, Pin{CID: cid, Name: name, Type: kind})
	}
	for cid, item := range line.Keys {
		add(cid, item.Type, item.Name)
	}
	if line.List != nil {
		for cid, item := range line.List.Keys {
			add(cid, item.Type, item.Name)
		}
	}
	if line.Object != nil {
		add(line.Object.Cid, line.Object.Type, line.Object.Name)
	}
	add(line.Cid, line.Type, line.Name)
	return out
}

type pinAddEvent struct {
	Pins     []string `json:"Pins"`
	Progress int      `json:"Progress"`
	Message  string   `json:"Message"`
	Type     string   `json:"Type"`
	Code     int      `json:"Code"`
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

func (c *Client) do(ctx context.Context, cmd string, q url.Values, body io.Reader, contentType string) ([]byte, int, error) {
	resp, err := c.open(ctx, cmd, q, body, contentType)
	if err != nil {
		return nil, 0, err
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

func (c *Client) open(ctx context.Context, cmd string, q url.Values, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(cmd, q), body)
	if err != nil {
		return nil, &OpError{Code: "unavailable"}
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, classify(err)
	}
	return resp, nil
}

func (c *Client) endpoint(cmd string, q url.Values) string {
	u := c.base.ResolveReference(&url.URL{Path: "/api/v0/" + cmd, RawQuery: q.Encode()})
	return u.String()
}

type maxRead struct {
	r   io.Reader
	n   int64
	max int64
}

func (m *maxRead) Read(p []byte) (int, error) {
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
