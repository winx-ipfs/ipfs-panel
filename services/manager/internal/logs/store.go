package logs

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	// SourceManager is the panel's own operational and audit log.
	SourceManager = "manager"
	// SourceKubo is the daemon stdout and stderr this manager spawned.
	SourceKubo = "kubo"
	// SourceAll selects both sources.
	SourceAll = "all"

	maxLineBytes  = 2048
	defaultRing   = 2000
	defaultRotate = 1 << 20
	defaultLimit  = 500
	maxLimit      = 1000
	maxQueryRunes = 200
)

// logFileName maps a source to its file name. The names are the only ones a
// store may open: the switch returns literals, so no caller-supplied value
// reaches a path expression.
func logFileName(source string) (string, bool) {
	switch source {
	case SourceManager:
		return "manager.log", true
	case SourceKubo:
		return "kubo.log", true
	}
	return "", false
}

// logFilePath is logFileName joined to the store directory.
func logFilePath(dir, source string) (string, bool) {
	name, ok := logFileName(source)
	if !ok {
		return "", false
	}
	return filepath.Join(dir, name), true
}

// Line is one redacted record.
type Line struct {
	ID     int64     `json:"id"`
	Time   time.Time `json:"time"`
	Source string    `json:"source"`
	Level  string    `json:"level"`
	Text   string    `json:"text"`
}

// Query selects a page of the ring.
type Query struct {
	After  int64
	Source string
	Level  string
	Q      string
	Limit  int
}

// Page is a slice of the ring. Next is the cursor for the following page.
type Page struct {
	Lines []Line
	Next  int64
}

// Options opens a store. Zero Ring and RotateBytes use the spec defaults.
type Options struct {
	Dir         string
	Ring        int
	RotateBytes int
}

// Store is the ring plus the rotated files under Dir.
type Store struct {
	mu      sync.Mutex
	dir     string
	ring    int
	rotate  int
	next    int64
	lines   map[string][]Line
	files   map[string]*rotFile
	subs    map[uint64]*sub
	nextSub uint64
	closed  bool
}

type sub struct {
	id       uint64
	q        Query
	ch       chan Line
	overflow bool
}

// Open creates <dir> and replays the tail of each current file into the ring.
func Open(opt Options) (*Store, error) {
	if strings.TrimSpace(opt.Dir) == "" {
		return nil, errors.New("logs: empty directory")
	}
	if opt.Ring <= 0 {
		opt.Ring = defaultRing
	}
	if opt.RotateBytes <= 0 {
		opt.RotateBytes = defaultRotate
	}
	if err := os.MkdirAll(opt.Dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(opt.Dir, 0o700); err != nil {
		return nil, err
	}
	s := &Store{
		dir:    opt.Dir,
		ring:   opt.Ring,
		rotate: opt.RotateBytes,
		lines:  map[string][]Line{SourceManager: nil, SourceKubo: nil},
		files:  map[string]*rotFile{},
		subs:   map[uint64]*sub{},
	}
	var pending []Line
	for _, source := range []string{SourceManager, SourceKubo} {
		f, err := openRot(opt.Dir, source, opt.RotateBytes)
		if err != nil {
			_ = s.Close()
			return nil, err
		}
		s.files[source] = f
		pending = append(pending, s.readTail(source)...)
	}
	sort.SliceStable(pending, func(i, j int) bool {
		return pending[i].Time.Before(pending[j].Time)
	})
	for _, line := range pending {
		s.next++
		line.ID = s.next
		s.lines[line.Source] = append(s.lines[line.Source], line)
	}
	return s, nil
}

// Close flushes the files. Later appends are ignored.
func (s *Store) Close() error {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	var err error
	for _, f := range s.files {
		if e := f.close(); e != nil && err == nil {
			err = e
		}
	}
	return err
}

// KuboWriter is the stdout and stderr sink for a daemon this manager spawned.
func (s *Store) KuboWriter() *Writer {
	return &Writer{store: s}
}

// AppendManager records one slog line.
func (s *Store) AppendManager(level, text string, at time.Time) {
	s.append(SourceManager, level, text, at)
}

// AppendKubo parses one daemon line and records it.
func (s *Store) AppendKubo(raw string, at time.Time) {
	level, text, when := ParseKubo(raw, at)
	if level == "" {
		return
	}
	s.append(SourceKubo, level, text, when)
}

func (s *Store) append(source, level, text string, at time.Time) {
	text = capLine(redactText(stripControls(strings.TrimSpace(text))))
	if text == "" || !knownLevel(level) {
		return
	}
	if at.IsZero() {
		at = time.Now()
	}
	at = at.UTC()
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return
	}
	s.next++
	line := Line{ID: s.next, Time: at, Source: source, Level: level, Text: text}
	buf := append(s.lines[source], line)
	if len(buf) > s.ring {
		buf = append([]Line(nil), buf[len(buf)-s.ring:]...)
	}
	s.lines[source] = buf
	if f := s.files[source]; f != nil {
		_ = f.write(formatStored(line))
	}
	for _, sub := range s.subs {
		if !match(sub.q, line) || line.ID <= sub.q.After {
			continue
		}
		select {
		case sub.ch <- line:
		default:
			sub.overflow = true
		}
	}
}

// Normalize checks a query and fills defaults. The error code is bad_request.
func Normalize(q Query) (Query, error) {
	if q.Source == "" {
		q.Source = SourceAll
	}
	switch q.Source {
	case SourceAll, SourceManager, SourceKubo:
	default:
		return Query{}, &Error{Code: "bad_request"}
	}
	if q.Level == "" {
		q.Level = "info"
	}
	if _, ok := rank[q.Level]; !ok {
		return Query{}, &Error{Code: "bad_request"}
	}
	if q.After < 0 {
		return Query{}, &Error{Code: "bad_request"}
	}
	if utf8.RuneCountInString(q.Q) > maxQueryRunes {
		return Query{}, &Error{Code: "bad_request"}
	}
	if q.Limit == 0 {
		q.Limit = defaultLimit
	}
	if q.Limit < 0 || q.Limit > maxLimit {
		return Query{}, &Error{Code: "bad_request"}
	}
	return q, nil
}

// Query returns a page of the ring. It does not read Kubo.
func (s *Store) Query(q Query) (Page, error) {
	q, err := Normalize(q)
	if err != nil {
		return Page{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	matched := s.collect(q)
	page := Page{Lines: matched, Next: q.After}
	if len(matched) > 0 {
		page.Next = matched[len(matched)-1].ID
	}
	return page, nil
}

func (s *Store) collect(q Query) []Line {
	var merged []Line
	if q.Source == SourceAll || q.Source == SourceManager {
		merged = mergeLines(merged, s.lines[SourceManager])
	}
	if q.Source == SourceAll || q.Source == SourceKubo {
		merged = mergeLines(merged, s.lines[SourceKubo])
	}
	matched := make([]Line, 0, len(merged))
	for _, line := range merged {
		if match(q, line) && line.ID > q.After {
			matched = append(matched, line)
		}
	}
	if q.After == 0 && len(matched) > q.Limit {
		matched = append([]Line(nil), matched[len(matched)-q.Limit:]...)
	} else if len(matched) > q.Limit {
		matched = append([]Line(nil), matched[:q.Limit]...)
	}
	return matched
}

// Subscribe returns a channel of new matching lines and an id for TakeOverflow.
// Close it with Unsubscribe. The channel is not closed.
func (s *Store) Subscribe(q Query) (uint64, <-chan Line, error) {
	q, err := Normalize(q)
	if err != nil {
		return 0, nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nextSub++
	id := s.nextSub
	ch := make(chan Line, 64)
	s.subs[id] = &sub{id: id, q: q, ch: ch}
	return id, ch, nil
}

// Unsubscribe drops a subscriber. Further lines are not sent.
func (s *Store) Unsubscribe(id uint64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.subs, id)
}

// TakeOverflow reports and clears a dropped-line flag for one subscriber.
func (s *Store) TakeOverflow(id uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	sub := s.subs[id]
	if sub == nil || !sub.overflow {
		return false
	}
	sub.overflow = false
	return true
}

// WriteDownload writes the retained files, oldest first.
// source all prefixes each line with the source name.
func (s *Store) WriteDownload(w io.Writer, source string) error {
	switch source {
	case "", SourceAll, SourceManager, SourceKubo:
	default:
		return &Error{Code: "bad_request"}
	}
	if source == "" {
		source = SourceAll
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, f := range s.files {
		_ = f.flush()
	}
	sources := []string{source}
	if source == SourceAll {
		sources = []string{SourceManager, SourceKubo}
	}
	for _, name := range sources {
		prefix := ""
		if source == SourceAll {
			prefix = name
		}
		base, ok := logFilePath(s.dir, name)
		if !ok {
			return &Error{Code: "bad_request"}
		}
		for _, path := range []string{base + ".2", base + ".1", base} {
			if err := writePrefixed(w, path, prefix); err != nil {
				return err
			}
		}
	}
	return nil
}

func (s *Store) readTail(source string) []Line {
	path, ok := logFilePath(s.dir, source)
	if !ok {
		return nil
	}
	body, err := os.ReadFile(path)
	if err != nil || len(body) == 0 {
		return nil
	}
	rawLines := bytes.Split(body, []byte("\n"))
	parsed := make([]Line, 0, len(rawLines))
	for _, raw := range rawLines {
		at, level, text, ok := parseStored(string(raw))
		if !ok {
			continue
		}
		text = capLine(redactText(text))
		if text == "" {
			continue
		}
		parsed = append(parsed, Line{Time: at, Source: source, Level: level, Text: text})
	}
	if len(parsed) > s.ring {
		parsed = parsed[len(parsed)-s.ring:]
	}
	return parsed
}

var rank = map[string]int{"debug": 0, "info": 1, "warn": 2, "error": 3}

func knownLevel(level string) bool {
	_, ok := rank[level]
	return ok
}

func match(q Query, line Line) bool {
	if q.Source != SourceAll && q.Source != "" && line.Source != q.Source {
		return false
	}
	min, ok := rank[q.Level]
	if !ok {
		min = rank["info"]
	}
	if rank[line.Level] < min {
		return false
	}
	if q.Q != "" && !strings.Contains(strings.ToLower(line.Text), strings.ToLower(q.Q)) {
		return false
	}
	return true
}

func mergeLines(a, b []Line) []Line {
	if len(a) == 0 {
		return b
	}
	if len(b) == 0 {
		return a
	}
	out := make([]Line, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].ID <= b[j].ID {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	out = append(out, a[i:]...)
	out = append(out, b[j:]...)
	return out
}

func formatStored(line Line) string {
	return line.Time.UTC().Format(time.RFC3339Nano) + "\t" + line.Level + "\t" + line.Text + "\n"
}

func parseStored(raw string) (time.Time, string, string, bool) {
	raw = strings.TrimRight(raw, "\r")
	timeS, rest, ok := strings.Cut(raw, "\t")
	if !ok {
		return time.Time{}, "", "", false
	}
	level, text, ok := strings.Cut(rest, "\t")
	if !ok || !knownLevel(level) {
		return time.Time{}, "", "", false
	}
	at, err := time.Parse(time.RFC3339Nano, timeS)
	if err != nil {
		return time.Time{}, "", "", false
	}
	return at.UTC(), level, text, true
}

// Writer splits a daemon stream into lines.
type Writer struct {
	mu    sync.Mutex
	buf   []byte
	store *Store
}

// Write implements io.Writer. It never fails the daemon because of a log error.
func (w *Writer) Write(p []byte) (int, error) {
	if w == nil || w.store == nil {
		return len(p), nil
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	for _, b := range p {
		if b == '\r' {
			continue
		}
		if b == '\n' {
			w.emit()
			continue
		}
		w.buf = append(w.buf, b)
		if len(w.buf) >= maxLineBytes {
			w.emitCut()
		}
	}
	return len(p), nil
}

// Flush records a trailing partial line.
func (w *Writer) Flush() {
	if w == nil {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	w.emit()
}

func (w *Writer) emit() {
	if len(w.buf) == 0 {
		return
	}
	text := string(w.buf)
	w.buf = w.buf[:0]
	w.store.AppendKubo(text, time.Now())
}

func (w *Writer) emitCut() {
	n := len(w.buf)
	for n > 0 && n > len(w.buf)-utf8.UTFMax && !utf8.Valid(w.buf[:n]) {
		n--
	}
	if n <= 0 {
		n = len(w.buf)
	}
	head := string(w.buf[:n])
	tail := append([]byte(nil), w.buf[n:]...)
	w.buf = tail
	w.store.AppendKubo(head, time.Now())
}

type rotFile struct {
	path   string
	f      *os.File
	w      *bufio.Writer
	size   int
	rotate int
}

func openRot(dir, source string, rotate int) (*rotFile, error) {
	path, ok := logFilePath(dir, source)
	if !ok {
		return nil, &Error{Code: "bad_request"}
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, err
	}
	return &rotFile{path: path, f: f, w: bufio.NewWriter(f), size: int(info.Size()), rotate: rotate}, nil
}

func (r *rotFile) write(line string) error {
	if r.size > 0 && r.size+len(line) > r.rotate {
		if err := r.rotateFile(); err != nil {
			return err
		}
	}
	n, err := r.w.WriteString(line)
	r.size += n
	if err != nil {
		return err
	}
	return r.w.Flush()
}

func (r *rotFile) flush() error {
	if r == nil || r.w == nil {
		return nil
	}
	return r.w.Flush()
}

func (r *rotFile) close() error {
	if r == nil || r.f == nil {
		return nil
	}
	flushErr := r.w.Flush()
	syncErr := r.f.Sync()
	closeErr := r.f.Close()
	r.f = nil
	if flushErr != nil {
		return flushErr
	}
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func (r *rotFile) rotateFile() error {
	if err := r.w.Flush(); err != nil {
		return err
	}
	if err := r.f.Sync(); err != nil {
		return err
	}
	if err := r.f.Close(); err != nil {
		return err
	}
	oldest := r.path + ".2"
	mid := r.path + ".1"
	_ = os.Remove(oldest)
	if err := renameReplace(mid, oldest); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := renameReplace(r.path, mid); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	f, err := os.OpenFile(r.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	r.f = f
	r.w = bufio.NewWriter(f)
	r.size = 0
	return nil
}

func renameReplace(old, newName string) error {
	if _, err := os.Stat(old); err != nil {
		return err
	}
	_ = os.Remove(newName)
	return os.Rename(old, newName)
}

func writePrefixed(w io.Writer, path, prefix string) error {
	body, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	for _, line := range bytes.Split(body, []byte("\n")) {
		if len(line) == 0 {
			continue
		}
		if prefix != "" {
			if _, err := io.WriteString(w, prefix+"\t"); err != nil {
				return err
			}
		}
		if _, err := w.Write(line); err != nil {
			return err
		}
		if _, err := w.Write([]byte("\n")); err != nil {
			return err
		}
	}
	return nil
}
