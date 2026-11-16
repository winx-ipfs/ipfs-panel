package logs

import (
	"bytes"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestRedactLeavesOrdinaryLines(t *testing.T) {
	const plain = "daemon is ready"
	if got := redactText(plain); got != plain {
		t.Fatalf("redact %q", got)
	}
	secret := `{"Identity":{"PrivKey":"c2VjcmV0a2V5"}} setup_token=abc ipfs_panel_session=cookie Bearer eyJ.abc api.key=pin`
	got := redactText(secret)
	for _, leak := range []string{"c2VjcmV0a2V5", "setup_token=abc", "session=cookie", "eyJ.abc", "api.key=pin"} {
		if strings.Contains(got, leak) {
			t.Fatalf("leaked %s in %s", leak, got)
		}
	}
	if !strings.Contains(got, redacted) {
		t.Fatalf("missing redaction %s", got)
	}
	line := redactText("ipfs-panel setup token: one-time-value")
	if strings.Contains(line, "one-time-value") {
		t.Fatal(line)
	}
}

func TestParseKuboLine(t *testing.T) {
	raw := "2026-11-15T11:14:00.123Z\tINFO\tcore\tdaemon.go:10\tdaemon is ready"
	level, text, at := ParseKubo(raw, time.Now())
	if level != "info" || text != "core daemon is ready" {
		t.Fatalf("level %s text %q", level, text)
	}
	if at.Format(time.RFC3339Nano) != "2026-11-15T11:14:00.123Z" {
		t.Fatalf("time %s", at)
	}
	level, text, _ = ParseKubo("Initializing daemon...", time.Unix(10, 0).UTC())
	if level != "info" || text != "Initializing daemon..." {
		t.Fatalf("plain %s %q", level, text)
	}
	level, text, _ = ParseKubo("\x1b[31m2026-11-15T11:14:00Z\tERROR\tbitswap\tmissing.go:3\tblock missing\x1b[0m", time.Now())
	if level != "error" || !strings.Contains(text, "block missing") || strings.Contains(text, "\x1b") {
		t.Fatalf("ansi %s %q", level, text)
	}
	js := `{"level":"warn","ts":"2026-11-15T11:14:00Z","logger":"dht","msg":"slow"}`
	level, text, _ = ParseKubo(js, time.Now())
	if level != "warn" || text != "slow" {
		t.Fatalf("json %s %q", level, text)
	}
}

func TestRingQueryRotationAndReplay(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(Options{Dir: dir, Ring: 3, RotateBytes: 180})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 15, 11, 0, 0, 0, time.UTC)
	for i := 0; i < 5; i++ {
		store.AppendManager("info", strings.Repeat("m", 40)+string(rune('a'+i)), now.Add(time.Duration(i)*time.Second))
	}
	store.AppendKubo("2026-11-15T11:14:00Z\tDEBUG\tcore\thidden.go:1\tsecret debug", now)
	page, err := store.Query(Query{Source: SourceManager, Level: "info", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Lines) != 3 {
		t.Fatalf("ring kept %d", len(page.Lines))
	}
	if !strings.HasSuffix(page.Lines[0].Text, "c") || !strings.HasSuffix(page.Lines[2].Text, "e") {
		t.Fatalf("ring %q %q", page.Lines[0].Text, page.Lines[2].Text)
	}
	debug, err := store.Query(Query{Source: SourceKubo, Level: "debug", Limit: 10})
	if err != nil || len(debug.Lines) != 1 || debug.Lines[0].Level != "debug" {
		t.Fatalf("debug %#v %v", debug.Lines, err)
	}
	hidden, err := store.Query(Query{Level: "info"})
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range hidden.Lines {
		if line.Level == "debug" {
			t.Fatal("info filter returned debug")
		}
	}
	found, err := store.Query(Query{Q: "secret", Level: "debug"})
	if err != nil || len(found.Lines) != 1 {
		t.Fatalf("search %#v %v", found.Lines, err)
	}
	if _, err := Normalize(Query{Limit: 1001}); err == nil || CodeOf(err) != "bad_request" {
		t.Fatalf("limit %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, "manager.log*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) < 2 || len(matches) > 3 {
		t.Fatalf("rotated files %v", matches)
	}
	reopen, err := Open(Options{Dir: dir, Ring: 3, RotateBytes: 180})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = reopen.Close() }()
	again, err := reopen.Query(Query{Source: SourceManager, Limit: 10})
	if err != nil || len(again.Lines) == 0 {
		t.Fatalf("replay %#v %v", again.Lines, err)
	}
	var buf bytes.Buffer
	store2, err := Open(Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store2.Close() }()
	store2.AppendManager("info", `PrivKey=c2VjcmV0`, now)
	if err := store2.WriteDownload(&buf, SourceManager); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "c2VjcmV0") || !strings.Contains(buf.String(), redacted) {
		t.Fatalf("download %s", buf.String())
	}
	info, err := os.Stat(filepath.Join(dir, "manager.log"))
	if err != nil {
		t.Fatal(err)
	}
	requireOwnerOnly(t, info)
	dirInfo, err := os.Stat(dir)
	if err != nil {
		t.Fatal(err)
	}
	requireOwnerDir(t, dirInfo)
}

func TestSlogHandlerRedactsSetupToken(t *testing.T) {
	store, err := Open(Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	logger := slog.New(NewHandler(store))
	const token = "super-secret-token"
	logger.Warn("one-time setup token; treat this line as a secret", "setup_token", token)
	logger.Debug("hidden debug")
	page, err := store.Query(Query{Level: "debug", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Lines) != 1 {
		t.Fatalf("lines %#v", page.Lines)
	}
	if strings.Contains(page.Lines[0].Text, token) || page.Lines[0].Level != "warn" {
		t.Fatalf("line %#v", page.Lines[0])
	}
}

func TestWriterSplitsLines(t *testing.T) {
	store, err := Open(Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	w := store.KuboWriter()
	_, _ = w.Write([]byte("2026-11-15T11:14:00Z\tINFO\tcore\thello\npartial"))
	w.Flush()
	page, err := store.Query(Query{Source: SourceKubo, Level: "debug", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Lines) != 2 || page.Lines[1].Text != "partial" {
		t.Fatalf("%#v", page.Lines)
	}
}

func TestAppendRace(t *testing.T) {
	store, err := Open(Options{Dir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 50; n++ {
				store.AppendManager("info", "race", time.Now())
			}
		}()
	}
	wg.Wait()
	page, err := store.Query(Query{Limit: 1000})
	if err != nil || len(page.Lines) != 400 {
		t.Fatalf("len %d %v", len(page.Lines), err)
	}
}

func TestClientLevelAndList(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		switch r.URL.Path {
		case "/api/v0/log/ls":
			_, _ = w.Write([]byte(`{"Strings":["bitswap","core","core"]}`))
		case "/api/v0/log/level":
			args := r.URL.Query()["arg"]
			if len(args) != 2 || args[0] != "core" || args[1] != "error" {
				http.Error(w, `{"Message":"bad"}`, http.StatusBadRequest)
				return
			}
			_, _ = w.Write([]byte(`{"Message":"Changed log level of 'core' to 'error'\n"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	client, err := New(ClientOptions{BaseURL: srv.URL})
	if err != nil {
		t.Fatal(err)
	}
	names, err := client.Subsystems(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 || names[0] != "bitswap" || names[1] != "core" {
		t.Fatalf("names %v", names)
	}
	msg, err := client.SetLevel(t.Context(), "core", "error")
	if err != nil || !strings.Contains(msg, "Changed log level of 'core' to 'error'") {
		t.Fatalf("msg %q %v", msg, err)
	}
	before := hits
	if _, err := client.SetLevel(t.Context(), "core", "nope"); CodeOf(err) != "bad_level" || hits != before {
		t.Fatalf("bad level %v hits %d/%d", err, hits, before)
	}
	if _, err := client.SetLevel(t.Context(), "has space", "info"); CodeOf(err) != "bad_subsystem" {
		t.Fatal(err)
	}
	if _, err := New(ClientOptions{BaseURL: "http://example.com:5001"}); err == nil {
		t.Fatal("accepted non-loopback")
	}
}

func TestClientRefused(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	client, err := New(ClientOptions{BaseURL: "http://" + addr, Timeout: time.Second})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.Subsystems(t.Context())
	if CodeOf(err) != "node_not_running" {
		t.Fatalf("refused %v", err)
	}
}

func TestLogFileNameOnlyKnownSources(t *testing.T) {
	cases := []struct {
		source string
		name   string
		ok     bool
	}{
		{SourceManager, "manager.log", true},
		{SourceKubo, "kubo.log", true},
		{SourceAll, "", false},
		{"", "", false},
		{"../etc/passwd", "", false},
		{"Manager", "", false},
		{"manager.log", "", false},
		{"manager/../kubo", "", false},
	}
	for _, tc := range cases {
		name, ok := logFileName(tc.source)
		if ok != tc.ok || name != tc.name {
			t.Fatalf("logFileName(%q) = %q,%v; want %q,%v", tc.source, name, ok, tc.name, tc.ok)
		}
		if ok {
			path, ok := logFilePath(t.TempDir(), tc.source)
			if !ok || filepath.Base(path) != tc.name {
				t.Fatalf("logFilePath(%q) = %q,%v", tc.source, path, ok)
			}
		}
	}
	if _, ok := logFilePath(t.TempDir(), SourceAll); ok {
		t.Fatal("logFilePath accepted all")
	}
}
