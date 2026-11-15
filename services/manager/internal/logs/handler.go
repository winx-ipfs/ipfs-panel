package logs

import (
	"context"
	"log/slog"
	"strconv"
	"strings"
	"time"
)

// Handler records Info and above into a Store. Debug from the manager is dropped.
type Handler struct {
	store *Store
	group string
	attrs []slog.Attr
}

// NewHandler returns a slog handler for the manager source.
func NewHandler(store *Store) *Handler {
	return &Handler{store: store}
}

func (h *Handler) Enabled(_ context.Context, level slog.Level) bool {
	return level >= slog.LevelInfo
}

func (h *Handler) Handle(_ context.Context, r slog.Record) error {
	if h == nil || h.store == nil || !h.Enabled(context.Background(), r.Level) {
		return nil
	}
	at := r.Time
	if at.IsZero() {
		at = time.Now()
	}
	h.store.AppendManager(slogLevel(r.Level), h.format(r), at)
	return nil
}

func (h *Handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := *h
	next.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &next
}

func (h *Handler) WithGroup(name string) slog.Handler {
	if name == "" {
		return h
	}
	next := *h
	if next.group == "" {
		next.group = name
	} else {
		next.group = next.group + "." + name
	}
	return &next
}

func (h *Handler) format(r slog.Record) string {
	var b strings.Builder
	b.WriteString(r.Message)
	for _, attr := range h.attrs {
		writeAttr(&b, h.group, attr)
	}
	r.Attrs(func(attr slog.Attr) bool {
		writeAttr(&b, h.group, attr)
		return true
	})
	return redactText(b.String())
}

func writeAttr(b *strings.Builder, group string, attr slog.Attr) {
	attr.Value = attr.Value.Resolve()
	if attr.Equal(slog.Attr{}) {
		return
	}
	if attr.Value.Kind() == slog.KindGroup {
		prefix := attr.Key
		if group != "" && attr.Key != "" {
			prefix = group + "." + attr.Key
		} else if attr.Key == "" {
			prefix = group
		}
		for _, nested := range attr.Value.Group() {
			writeAttr(b, prefix, nested)
		}
		return
	}
	key := attr.Key
	if group != "" {
		if key != "" {
			key = group + "." + key
		} else {
			key = group
		}
	}
	val := attr.Value.String()
	if secretAttr(key) {
		val = redacted
	}
	b.WriteByte(' ')
	b.WriteString(key)
	b.WriteByte('=')
	if strings.ContainsAny(val, " \t\"") {
		b.WriteString(strconv.Quote(val))
	} else {
		b.WriteString(val)
	}
}

func slogLevel(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "error"
	case level >= slog.LevelWarn:
		return "warn"
	case level >= slog.LevelInfo:
		return "info"
	default:
		return "debug"
	}
}

// Multi sends each record to every handler that is enabled for it.
func Multi(handlers ...slog.Handler) slog.Handler {
	clean := make([]slog.Handler, 0, len(handlers))
	for _, h := range handlers {
		if h != nil {
			clean = append(clean, h)
		}
	}
	return &multi{hs: clean}
}

type multi struct {
	hs []slog.Handler
}

func (m *multi) Enabled(ctx context.Context, level slog.Level) bool {
	for _, h := range m.hs {
		if h.Enabled(ctx, level) {
			return true
		}
	}
	return false
}

func (m *multi) Handle(ctx context.Context, r slog.Record) error {
	var err error
	for _, h := range m.hs {
		if !h.Enabled(ctx, r.Level) {
			continue
		}
		if e := h.Handle(ctx, r.Clone()); e != nil && err == nil {
			err = e
		}
	}
	return err
}

func (m *multi) WithAttrs(attrs []slog.Attr) slog.Handler {
	next := make([]slog.Handler, len(m.hs))
	for i, h := range m.hs {
		next[i] = h.WithAttrs(attrs)
	}
	return &multi{hs: next}
}

func (m *multi) WithGroup(name string) slog.Handler {
	next := make([]slog.Handler, len(m.hs))
	for i, h := range m.hs {
		next[i] = h.WithGroup(name)
	}
	return &multi{hs: next}
}
