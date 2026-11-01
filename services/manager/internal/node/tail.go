package node

import "sync"

// tailBuf keeps the last max bytes written to it.
type tailBuf struct {
	mu  sync.Mutex
	buf []byte
	max int
}

func newTail(max int) *tailBuf {
	if max < 1 {
		max = detailLimit
	}
	return &tailBuf{max: max}
}

func (t *tailBuf) Write(p []byte) (int, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.buf = append(t.buf, p...)
	if len(t.buf) > t.max {
		t.buf = append([]byte(nil), t.buf[len(t.buf)-t.max:]...)
	}
	return len(p), nil
}

func (t *tailBuf) String() string {
	if t == nil {
		return ""
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return string(t.buf)
}

// headBuf keeps the first max bytes and discards the rest without blocking the writer.
type headBuf struct {
	buf []byte
	max int
}

func (h *headBuf) Write(p []byte) (int, error) {
	if h.max <= 0 {
		h.max = 1 << 20
	}
	if len(h.buf) < h.max {
		room := h.max - len(h.buf)
		if len(p) > room {
			h.buf = append(h.buf, p[:room]...)
		} else {
			h.buf = append(h.buf, p...)
		}
	}
	return len(p), nil
}
