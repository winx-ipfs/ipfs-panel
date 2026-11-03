package pins

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestNewRejectsNonLoopback(t *testing.T) {
	for _, raw := range []string{
		"http://192.0.2.1:5001",
		"https://127.0.0.1:5001",
		"http://127.0.0.1",
		"http://localhost:5001",
		"http://127.0.0.1:5001/api",
		"http://user:pass@127.0.0.1:5001",
	} {
		if _, err := New(Options{BaseURL: raw}); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
}

func TestListNamesAndFallback(t *testing.T) {
	var namesCalls int
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		if !strings.HasSuffix(r.URL.Path, "/pin/ls") {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("names") == "true" {
			mu.Lock()
			namesCalls++
			mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"Message":"unrecognized argument: \"names\""}`)
			return
		}
		kind := r.URL.Query().Get("type")
		if kind == "recursive" {
			_, _ = io.WriteString(w, `{"Cid":"`+sampleCIDv1()+`","Type":"recursive"}`+"\n")
			_, _ = io.WriteString(w, `{"Cid":"`+sampleCIDv1()+`","Type":"indirect"}`+"\n")
			return
		}
		_, _ = io.WriteString(w, `{"Keys":{"`+sampleCIDv0()+`":{"Type":"direct"}}}`)
	}))
	defer srv.Close()
	c := newPinClient(t, srv.URL)
	support := &Support{}
	list, err := c.List(context.Background(), support)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("%#v", list)
	}
	if support.NamesFrom() != "panel" {
		t.Fatalf("names %s", support.NamesFrom())
	}
	if _, err := c.List(context.Background(), support); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if namesCalls != 1 {
		t.Fatalf("probed names %d times", namesCalls)
	}
}

func TestListKeepsKuboNames(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("names") != "true" {
			t.Errorf("names %s", r.URL.RawQuery)
		}
		if r.URL.Query().Get("type") == "direct" {
			_, _ = io.WriteString(w, `{"Keys":{}}`)
			return
		}
		_, _ = io.WriteString(w, `{"Keys":{"`+sampleCIDv1()+`":{"Type":"recursive","Name":"notes"}}}`)
	}))
	defer srv.Close()
	c := newPinClient(t, srv.URL)
	support := &Support{}
	list, err := c.List(context.Background(), support)
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 1 || list[0].Name != "notes" || list[0].Type != "recursive" || support.NamesFrom() != "kubo" {
		t.Fatalf("%#v %s", list, support.NamesFrom())
	}
}

func TestAddProgressCancelAndNameFallback(t *testing.T) {
	var mu sync.Mutex
	var sawName bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/pin/add") && r.URL.Query().Get("name") != "":
			mu.Lock()
			sawName = true
			mu.Unlock()
			w.WriteHeader(http.StatusBadRequest)
			_, _ = io.WriteString(w, `{"Message":"unused argument: \"name\""}`)
		case strings.HasSuffix(r.URL.Path, "/pin/add"):
			if r.URL.Query().Get("progress") != "true" {
				t.Errorf("progress %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"Progress":4}`+"\n")
			_, _ = io.WriteString(w, `{"Pins":["`+sampleCIDv1()+`"]}`+"\n")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newPinClient(t, srv.URL)
	var progress int
	cid, err := c.Add(context.Background(), sampleCIDv1(), "notes", true, func(n int) { progress = n }, &Support{})
	if err != nil || cid != sampleCIDv1() || progress != 4 {
		t.Fatalf("cid %s progress %d err %v", cid, progress, err)
	}
	mu.Lock()
	if !sawName {
		t.Fatal("name was not attempted")
	}
	mu.Unlock()

	block := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer block.Close()
	blocked := newPinClient(t, block.URL)
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := blocked.Add(ctx, sampleCIDv0(), "", true, nil, nil)
		done <- err
	}()
	time.Sleep(20 * time.Millisecond)
	cancel()
	if err := <-done; CodeOf(err) != "cancelled" {
		t.Fatalf("cancel %v", err)
	}
}

func TestUploadStreamsAndLimits(t *testing.T) {
	var got []byte
	var chunked bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/add") {
			t.Errorf("path %s", r.URL.Path)
		}
		if r.URL.Query().Get("pin") != "true" || r.URL.Query().Get("cid-version") != "1" {
			t.Errorf("query %s", r.URL.RawQuery)
		}
		chunked = r.ContentLength < 0
		mr, err := r.MultipartReader()
		if err != nil {
			t.Errorf("multipart %v", err)
			return
		}
		part, err := mr.NextPart()
		if err != nil {
			t.Errorf("part %v", err)
			return
		}
		got, _ = io.ReadAll(part)
		_, _ = io.WriteString(w, `{"Name":"notes.txt","Hash":"`+sampleCIDv1()+`","Size":"5"}`)
	}))
	defer srv.Close()
	c := newPinClient(t, srv.URL)
	pin, err := c.Upload(context.Background(), "notes.txt", strings.NewReader("hello"))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "hello" || !chunked || pin.CID != sampleCIDv1() || pin.Size == nil || *pin.Size != 5 {
		t.Fatalf("got %q chunked %v pin %#v", got, chunked, pin)
	}
	limited := newPinClient(t, srv.URL)
	limited.maxBytes = 4
	if _, err := limited.Upload(context.Background(), "notes.txt", strings.NewReader("hello")); CodeOf(err) != "too_large" {
		t.Fatalf("limit %v", err)
	}
}

func TestRemoveIndirectAndNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("arg") == sampleCIDv0() {
			if r.URL.Query().Get("recursive") != "false" {
				t.Errorf("recursive %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"Message":"not pinned or pinned indirectly"}`)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"Message":"not pinned"}`)
	}))
	defer srv.Close()
	c := newPinClient(t, srv.URL)
	if err := c.Remove(context.Background(), sampleCIDv0(), "direct"); CodeOf(err) != "indirect" {
		t.Fatalf("indirect %v", err)
	}
	if err := c.Remove(context.Background(), sampleCIDv1(), "recursive"); CodeOf(err) != "not_found" {
		t.Fatalf("missing %v", err)
	}
}

func TestIgnoresProxyRedirectAndDial(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://192.0.2.1:9")
	t.Setenv("http_proxy", "http://192.0.2.1:9")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("arg") == "redir" {
			http.Redirect(w, r, "http://192.0.2.1/nope", http.StatusFound)
			return
		}
		if r.URL.Query().Get("arg") == "slow" {
			time.Sleep(200 * time.Millisecond)
			return
		}
		_, _ = io.WriteString(w, `{"Keys":{}}`)
	}))
	defer srv.Close()
	c := newPinClient(t, srv.URL)
	if _, err := c.List(context.Background(), &Support{}); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove(context.Background(), "redir", "recursive"); CodeOf(err) != "kubo_error" {
		t.Fatalf("redirect %v", err)
	}
	c.op = 30 * time.Millisecond
	if err := c.Remove(context.Background(), "slow", "recursive"); CodeOf(err) != "timeout" {
		t.Fatalf("timeout %v", err)
	}
	refused, err := New(Options{BaseURL: "http://127.0.0.1:1", OpTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := refused.List(context.Background(), nil); CodeOf(err) != "node_not_running" {
		t.Fatalf("refused %v", err)
	}
}

func TestFillSizes(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/object/stat") {
			t.Errorf("path %s", r.URL.Path)
		}
		_, _ = io.WriteString(w, `{"CumulativeSize":1200}`)
	}))
	defer srv.Close()
	c := newPinClient(t, srv.URL)
	pins := []Pin{{CID: sampleCIDv1(), Type: "recursive"}}
	c.FillSizes(context.Background(), pins)
	if pins[0].Size == nil || *pins[0].Size != 1200 {
		t.Fatalf("%#v", pins[0])
	}
}

func newPinClient(t *testing.T, base string) *Client {
	t.Helper()
	c, err := New(Options{BaseURL: base, MaxBytes: 1024, OpTimeout: time.Second, TransferTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
