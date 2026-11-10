package explore

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

const helloCID = "bafkqac3imvwgy3zao5xxe3de"

func TestInspectBlockAndPath(t *testing.T) {
	var codec atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arg := r.URL.Query().Get("arg")
		if strings.HasSuffix(r.URL.Path, "/dag/resolve") {
			if !strings.HasSuffix(arg, "/notes.txt") {
				w.WriteHeader(http.StatusInternalServerError)
				_, _ = io.WriteString(w, `{"Message":"no link named \"missing\" under the path"}`)
				return
			}
			_, _ = io.WriteString(w, `{"Cid":{"/":"`+helloCID+`"},"RemPath":"inside"}`)
			return
		}
		codec.Store(r.URL.Query().Get("output-codec"))
		if arg != helloCID {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"Message":"merkledag: not found"}`)
			return
		}
		_, _ = io.WriteString(w, `{"/":{"bytes":"aGVsbG8"}}`)
	}))
	defer srv.Close()
	c := newClient(t, srv.URL)

	block, err := c.Inspect(context.Background(), "  "+helloCID+"\n")
	if err != nil {
		t.Fatal(err)
	}
	if block.Kind != "bytes" || block.Text != "hello" || block.CID != helloCID || block.Target != helloCID {
		t.Fatalf("%+v", block)
	}
	if codec.Load() != "dag-json" {
		t.Fatalf("codec %v", codec.Load())
	}

	walked, err := c.Inspect(context.Background(), "/ipfs/"+helloCID+"/notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	if walked.CID != helloCID || walked.Focus != "inside" || walked.Kind != "bytes" || walked.Target != "/ipfs/"+helloCID+"/notes.txt" {
		t.Fatalf("walked %+v", walked)
	}
	if _, err := c.Inspect(context.Background(), "/ipfs/"+helloCID+"/missing"); CodeOf(err) != "not_found" {
		t.Fatalf("missing %v", err)
	}
	if _, err := c.Inspect(context.Background(), "not a cid"); CodeOf(err) != "invalid_target" {
		t.Fatalf("bad %v", err)
	}
}

func TestInspectTimeoutTooLargeRedirectAndProxy(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://192.0.2.1:9")
	t.Setenv("http_proxy", "http://192.0.2.1:9")
	var mode atomic.Value
	mode.Store("ok")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch mode.Load() {
		case "slow":
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
		case "huge":
			_, _ = io.Copy(w, io.LimitReader(neverEnding('x'), maxBody+2))
		case "redirect":
			http.Redirect(w, r, "http://192.0.2.1/nope", http.StatusFound)
		default:
			_, _ = io.WriteString(w, `{"/":{"bytes":"aGVsbG8"}}`)
		}
	}))
	defer srv.Close()
	if _, err := New(Options{BaseURL: "http://192.0.2.1:5001"}); err == nil {
		t.Fatal("public dial accepted")
	}
	c := newClient(t, srv.URL)
	if _, err := c.Inspect(context.Background(), helloCID); err != nil {
		t.Fatalf("proxy %v", err)
	}
	c.timeout = 40 * time.Millisecond
	mode.Store("slow")
	if _, err := c.Inspect(context.Background(), helloCID); CodeOf(err) != "timeout" {
		t.Fatalf("timeout %v", err)
	}
	c.timeout = time.Second
	mode.Store("huge")
	if _, err := c.Inspect(context.Background(), helloCID); CodeOf(err) != "too_large" {
		t.Fatalf("huge %v", err)
	}
	mode.Store("redirect")
	if _, err := c.Inspect(context.Background(), helloCID); CodeOf(err) != "kubo_error" {
		t.Fatalf("redirect %v", err)
	}
}

func newClient(t *testing.T, base string) *Client {
	t.Helper()
	c, err := New(Options{BaseURL: base, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return c
}

type neverEnding byte

func (b neverEnding) Read(p []byte) (int, error) {
	for i := range p {
		p[i] = byte(b)
	}
	return len(p), nil
}
