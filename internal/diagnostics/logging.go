// Package diagnostics provides structured logging with rotation, secret
// redaction and failure classification for model instances.
package diagnostics

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

const (
	maxLogFileSize = 8 << 20 // 8 MiB per segment
	maxLogSegments = 5       // keep at most 5 rotated segments per stream
)

// rotatingWriter writes to <name>.log and rotates by size, keeping a bounded
// number of segments so disk growth stays limited.
type rotatingWriter struct {
	mu   sync.Mutex
	path string
	f    *os.File
	size int64
}

func newRotatingWriter(dir, name string) (*rotatingWriter, error) {
	p := filepath.Join(dir, name+".log")
	f, err := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &rotatingWriter{path: p, f: f, size: st.Size()}, nil
}

func (w *rotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.size+int64(len(p)) > maxLogFileSize {
		w.rotateLocked()
	}
	n, err := w.f.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingWriter) rotateLocked() {
	w.f.Close()
	// Shift segments: .4 -> .5, ..., current -> .1
	for i := maxLogSegments - 1; i >= 1; i-- {
		os.Remove(fmt.Sprintf("%s.%d", w.path, i+1))
		os.Rename(fmt.Sprintf("%s.%d", w.path, i), fmt.Sprintf("%s.%d", w.path, i+1))
	}
	os.Rename(w.path, w.path+".1")
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o644)
	if err != nil {
		// Degrade to stderr rather than crashing.
		w.f = os.Stderr
	} else {
		w.f = f
	}
	w.size = 0
}

func (w *rotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.f != nil && w.f != os.Stderr {
		return w.f.Close()
	}
	return nil
}

// Logger is a named, leveled structured logger with redaction.
type Logger struct {
	*slog.Logger
	Name string
}

var secretPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(hf_[A-Za-z0-9]{20,})`),
	regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._\-]+`),
	regexp.MustCompile(`(?i)(api[-_ ]?key["'=:\s]+)[A-Za-z0-9._\-]{8,}`),
	regexp.MustCompile(`(?i)(authorization["':\s]+)[^\s"']+`),
	regexp.MustCompile(`(?i)(token["'=:\s]+)[A-Za-z0-9._\-]{16,}`),
}

// Redact scrubs tokens, API keys and authorization headers from s.
func Redact(s string) string {
	for _, re := range secretPatterns {
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			// Preserve the prefix (e.g. "Bearer ") where a group matched.
			if i := strings.IndexAny(m, " =:'\""); i > 0 && i < len(m)-4 {
				return m[:i+1] + "[REDACTED]"
			}
			if strings.Contains(m, " ") {
				parts := strings.SplitN(m, " ", 2)
				return parts[0] + " [REDACTED]"
			}
			return "[REDACTED]"
		})
	}
	return s
}

// redactHandler wraps a slog.Handler, redacting message and string attrs.
type redactHandler struct{ h slog.Handler }

func (r redactHandler) Enabled(c context.Context, l slog.Level) bool { return r.h.Enabled(c, l) }
func (r redactHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return redactHandler{r.h.WithAttrs(a)}
}
func (r redactHandler) WithGroup(g string) slog.Handler { return redactHandler{r.h.WithGroup(g)} }
func (r redactHandler) Handle(ctx context.Context, rec slog.Record) error {
	nr := slog.NewRecord(rec.Time, rec.Level, Redact(rec.Message), rec.PC)
	rec.Attrs(func(a slog.Attr) bool {
		if a.Value.Kind() == slog.KindString {
			a.Value = slog.StringValue(Redact(a.Value.String()))
		}
		nr.AddAttrs(a)
		return true
	})
	return r.h.Handle(ctx, nr)
}

// Manager owns all named log streams under the application log directory.
type Manager struct {
	dir     string
	closers []io.Closer
	mu      sync.Mutex
	// Subscribers receive every record for the live Logs page.
	subsMu sync.Mutex
	subs   map[chan Entry]struct{}
}

// Entry is a single log record for live-tail subscribers.
type Entry struct {
	Time    time.Time `json:"time"`
	Source  string    `json:"source"`
	Level   string    `json:"level"`
	Message string    `json:"message"`
}

func NewManager(dir string) *Manager { return &Manager{dir: dir, subs: map[chan Entry]struct{}{}} }

// Logger returns (creating if needed) a named logger writing to
// <dir>/<name>.log as JSON lines.
func (m *Manager) Logger(name string, level slog.Level) *Logger {
	w, err := newRotatingWriter(m.dir, name)
	if err != nil {
		w = &rotatingWriter{f: os.Stderr, path: "stderr"}
	} else {
		m.mu.Lock()
		m.closers = append(m.closers, w)
		m.mu.Unlock()
	}
	th := slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})
	h := redactHandler{th}
	fan := &fanHandler{inner: h, mgr: m, source: name}
	return &Logger{Logger: slog.New(fan).With("source", name), Name: name}
}

// fanHandler forwards records to the wrapped handler and to live subscribers.
type fanHandler struct {
	inner  slog.Handler
	mgr    *Manager
	source string
}

func (f *fanHandler) Enabled(c context.Context, l slog.Level) bool { return f.inner.Enabled(c, l) }
func (f *fanHandler) WithAttrs(a []slog.Attr) slog.Handler {
	return &fanHandler{inner: f.inner.WithAttrs(a), mgr: f.mgr, source: f.source}
}
func (f *fanHandler) WithGroup(g string) slog.Handler {
	return &fanHandler{inner: f.inner.WithGroup(g), mgr: f.mgr, source: f.source}
}
func (f *fanHandler) Handle(ctx context.Context, rec slog.Record) error {
	err := f.inner.Handle(ctx, rec)
	e := Entry{Time: rec.Time.UTC(), Source: f.source, Level: rec.Level.String(), Message: rec.Message}
	f.mgr.subsMu.Lock()
	for ch := range f.mgr.subs {
		select {
		case ch <- e:
		default: // drop for slow consumers; live tail is best-effort
		}
	}
	f.mgr.subsMu.Unlock()
	return err
}

// Subscribe returns a channel of live log entries and an unsubscribe func.
func (m *Manager) Subscribe() (chan Entry, func()) {
	ch := make(chan Entry, 256)
	m.subsMu.Lock()
	m.subs[ch] = struct{}{}
	m.subsMu.Unlock()
	return ch, func() {
		m.subsMu.Lock()
		delete(m.subs, ch)
		m.subsMu.Unlock()
		close(ch)
	}
}

// Close flushes and closes all log files.
func (m *Manager) Close() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, c := range m.closers {
		c.Close()
	}
}
