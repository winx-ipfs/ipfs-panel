package files

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
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
	c, err := New(Options{BaseURL: "http://127.0.0.1:5001"})
	if err != nil {
		t.Fatal(err)
	}
	if c.MaxBytes() != DefaultMaxBytes {
		t.Fatalf("max %d", c.MaxBytes())
	}
}

func TestListWriteRead(t *testing.T) {
	var written []byte
	var chunked bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method %s", r.Method)
		}
		switch {
		case strings.HasSuffix(r.URL.Path, "/files/ls"):
			if r.URL.Query().Get("long") != "true" {
				t.Errorf("long %s", r.URL.RawQuery)
			}
			_, _ = io.WriteString(w, `{"Entries":[{"Name":"notes.txt","Type":0,"Size":5,"Hash":"bafyabc","Mtime":1730000000},{"Name":"photos","Type":1,"Size":0,"Hash":"bafydir"},{"Name":"..","Type":1}]}`)
		case strings.HasSuffix(r.URL.Path, "/files/stat"):
			_, _ = io.WriteString(w, `{"Hash":"bafyabc","Size":5,"Type":"file","Mtime":1730000000}`)
		case strings.HasSuffix(r.URL.Path, "/files/write"):
			chunked = r.ContentLength < 0
			mr, err := r.MultipartReader()
			if err != nil {
				t.Errorf("multipart %v", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				t.Errorf("part %v", err)
				return
			}
			written, _ = io.ReadAll(part)
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/files/read"):
			_, _ = io.WriteString(w, "hello")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, 1024)
	list, err := c.List(context.Background(), "/docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 || list[0].Name != "notes.txt" || list[0].Type != "file" || list[0].CID != "bafyabc" || list[0].Modified.IsZero() {
		t.Fatalf("list %#v", list)
	}
	if list[1].Type != "directory" {
		t.Fatalf("dir %#v", list[1])
	}
	entry, err := c.Write(context.Background(), "/docs/notes.txt", strings.NewReader("hello"), true)
	if err != nil {
		t.Fatal(err)
	}
	if string(written) != "hello" || !chunked {
		t.Fatalf("written %q chunked %v", written, chunked)
	}
	if entry.CID != "bafyabc" || entry.Size != 5 {
		t.Fatalf("entry %#v", entry)
	}
	body, err := c.Open(context.Background(), "/docs/notes.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = body.Close() }()
	got, err := io.ReadAll(body)
	if err != nil || string(got) != "hello" {
		t.Fatalf("%q %v", got, err)
	}
}

func TestWriteLimitAndConflicts(t *testing.T) {
	// The handler runs on its own goroutine, so the byte count it records
	// is shared state. Send it back over a channel instead of reading the
	// variable from the test goroutine.
	received := make(chan int, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/files/stat"):
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"Message":"file does not exist"}`)
		case strings.HasSuffix(r.URL.Path, "/files/write"):
			mr, err := r.MultipartReader()
			if err != nil {
				return
			}
			part, err := mr.NextPart()
			if err != nil {
				return
			}
			buf, _ := io.ReadAll(part)
			select {
			case received <- len(buf):
			default:
			}
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/files/rm"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, 4)
	_, err := c.Write(context.Background(), "/a.txt", strings.NewReader("hello"), false)
	if CodeOf(err) != "too_large" {
		t.Fatalf("%v", err)
	}
	// The handler posts the count before it writes the response, so a value
	// is already buffered here whenever the write request was made at all.
	// No request means no buffering, which is also within the limit.
	buffered := 0
	select {
	case buffered = <-received:
	default:
	}
	if buffered > 4 {
		t.Fatalf("buffered %d bytes", buffered)
	}

	exists := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, "/files/stat") {
			_, _ = io.WriteString(w, `{"Hash":"bafy","Size":1,"Type":"file"}`)
			return
		}
		if strings.HasSuffix(r.URL.Path, "/files/write") {
			t.Error("write should not run")
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer exists.Close()
	c = newTestClient(t, exists.URL, 1024)
	_, err = c.Write(context.Background(), "/a.txt", strings.NewReader("z"), false)
	if CodeOf(err) != "already_exists" {
		t.Fatalf("%v", err)
	}
}

func TestMutationsAndErrors(t *testing.T) {
	var mvArgs []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/files/stat"):
			arg := r.URL.Query().Get("arg")
			if arg == "/dir" {
				_, _ = io.WriteString(w, `{"Hash":"bafydir","Size":0,"Type":"directory"}`)
				return
			}
			_, _ = io.WriteString(w, `{"Hash":"bafy","Size":1,"Type":"file"}`)
		case strings.HasSuffix(r.URL.Path, "/files/mkdir"):
			if r.URL.Query().Get("parents") != "false" {
				t.Errorf("parents %s", r.URL.RawQuery)
			}
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/files/mv"):
			mvArgs = append([]string{}, r.URL.Query()["arg"]...)
			w.WriteHeader(http.StatusOK)
		case strings.HasSuffix(r.URL.Path, "/files/cp"), strings.HasSuffix(r.URL.Path, "/files/rm"):
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"Message":"file already exists"}`)
		}
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, 1024)
	ctx := context.Background()
	if err := c.Mkdir(ctx, "/photos"); err != nil {
		t.Fatal(err)
	}
	if err := c.Mkdir(ctx, "/"); CodeOf(err) != "invalid_path" {
		t.Fatal(err)
	}
	dest, err := c.Rename(ctx, "/notes.txt", "renamed.txt")
	if err != nil || dest != "/renamed.txt" {
		t.Fatalf("%q %v", dest, err)
	}
	if len(mvArgs) != 2 || mvArgs[0] != "/notes.txt" || mvArgs[1] != "/renamed.txt" {
		t.Fatalf("mv %#v", mvArgs)
	}
	if err := c.Copy(ctx, "/notes.txt", "/other.txt"); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove(ctx, "/notes.txt"); err != nil {
		t.Fatal(err)
	}
	if err := c.Remove(ctx, "/"); CodeOf(err) != "invalid_path" {
		t.Fatal(err)
	}
	if err := c.Move(ctx, "/dir", "/dir/child"); CodeOf(err) != "invalid_path" {
		t.Fatalf("move into self %v", err)
	}
}

func TestKuboFailures(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("arg") == "/missing" {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"Message":"file does not exist"}`)
			return
		}
		if r.URL.Query().Get("arg") == "/slow" {
			select {
			case <-r.Context().Done():
			case <-time.After(time.Second):
			}
			return
		}
		http.Redirect(w, r, "http://192.0.2.1/nope", http.StatusFound)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, 1024)
	c.op = 40 * time.Millisecond
	if _, err := c.Stat(context.Background(), "/missing"); CodeOf(err) != "not_found" {
		t.Fatalf("missing %v", err)
	}
	if _, err := c.Stat(context.Background(), "/slow"); CodeOf(err) != "timeout" {
		t.Fatalf("slow %v", err)
	}
	if err := c.Mkdir(context.Background(), "/redir"); CodeOf(err) != "kubo_error" {
		t.Fatalf("redirect %v", err)
	}
}

func TestIgnoresProxyAndRefusesPublicDial(t *testing.T) {
	t.Setenv("HTTP_PROXY", "http://192.0.2.1:9")
	t.Setenv("http_proxy", "http://192.0.2.1:9")
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"Entries":[]}`)
	}))
	defer srv.Close()
	c := newTestClient(t, srv.URL, 1024)
	if _, err := c.List(context.Background(), "/"); err != nil {
		t.Fatal(err)
	}
	refused, err := New(Options{BaseURL: "http://127.0.0.1:1", OpTimeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	if err := refused.Mkdir(context.Background(), "/a"); CodeOf(err) != "node_not_running" {
		t.Fatalf("refused %v", err)
	}
}

func newTestClient(t *testing.T, base string, max int64) *Client {
	t.Helper()
	c, err := New(Options{
		BaseURL:         base,
		MaxBytes:        max,
		OpTimeout:       time.Second,
		TransferTimeout: time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	return c
}
