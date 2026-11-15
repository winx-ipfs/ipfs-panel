package configedit

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestClientShowAndReplace(t *testing.T) {
	var replaced string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config/show"):
			_, _ = io.WriteString(w, `{"Identity":{"PeerID":"abc","PrivKey":"secret-key"},"Addresses":{"API":"/ip4/127.0.0.1/tcp/5001","Gateway":["/ip4/127.0.0.1/tcp/8081"]}}`)
		case strings.HasSuffix(r.URL.Path, "/config/replace"):
			media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
			if err != nil || media != "multipart/form-data" {
				http.Error(w, "type", http.StatusBadRequest)
				return
			}
			reader := multipart.NewReader(r.Body, params["boundary"])
			part, err := reader.NextPart()
			if err != nil {
				http.Error(w, "part", http.StatusBadRequest)
				return
			}
			body, _ := io.ReadAll(part)
			replaced = string(body)
			w.WriteHeader(http.StatusOK)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	svc := New(client, t.TempDir(), t.TempDir(), "127.0.0.1:8080")
	view, err := svc.Show(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(view.Text, "secret-key") {
		t.Fatal("show leaked the key")
	}
	edited := strings.Replace(view.Text, `"PeerID": "abc"`, `"PeerID": "abc"`, 1)
	// Force a real edit that still passes the locks.
	edited = strings.Replace(view.Text, "{", "{\n  \"Discovery\": {\"MDNS\": {\"Enabled\": false}},", 1)
	if _, err := svc.Replace(context.Background(), edited); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(replaced, "PrivKey") || strings.Contains(replaced, "secret-key") {
		t.Fatalf("replace sent a secret: %s", replaced)
	}
}

func TestClientRefusedAndRejected(t *testing.T) {
	refused, err := NewClient("http://127.0.0.1:1", time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := refused.Show(context.Background()); CodeOf(err) != "node_not_running" {
		t.Fatalf("refused %v", err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = io.WriteString(w, `{"Message":"cannot add or remove remote pinning services with 'config replace'","Code":0,"Type":"error"}`)
	}))
	defer srv.Close()
	client, err := NewClient(srv.URL, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	err = client.Replace(context.Background(), []byte(`{}`))
	if CodeOf(err) != "kubo_rejected" || !strings.Contains(err.Error(), "kubo_rejected") {
		t.Fatal(err)
	}
	var edit *Error
	if !asError(err, &edit) || !strings.Contains(edit.Detail, "remote pinning") {
		t.Fatalf("%#v", err)
	}
}

func asError(err error, dest **Error) bool {
	edit, ok := err.(*Error)
	if !ok {
		return false
	}
	*dest = edit
	return true
}
