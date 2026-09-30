package server

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// The logging wrapper must not hide the optional interfaces the real
// ResponseWriter implements.
//
// Embedding http.ResponseWriter gives the wrapper that interface's
// methods and none of the optional ones, so wrapping silently removed
// http.Flusher. connect-go checks for it on a server-streaming handler
// and refuses the call outright:
//
//	*server.recorder does not implement http.Flusher
//
// ChatService.SendMessage is the only streaming RPC, so that was the
// whole of Ask Roger returning an internal error on every call while
// every unary RPC on the same mux worked. It compiled, it passed the Go
// tests, it passed the browser build, and it was only found by sending
// a real Connect stream frame at a running server.
//
// A type assertion in a test is the cheapest thing that would have
// caught it, so here it is.
func TestLoggingWrapperStaysFlushable(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))

	var sawFlusher bool
	h := withLogging(log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		f, ok := w.(http.Flusher)
		sawFlusher = ok
		if ok {
			// Calling it must not panic when the writer underneath
			// supports it, and must not panic when it does not.
			f.Flush()
		}
		w.WriteHeader(http.StatusOK)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/api/x", nil))

	if !sawFlusher {
		t.Fatal("the logging wrapper is not an http.Flusher; every server-streaming RPC will fail")
	}
}

// A writer that cannot flush must not make the wrapper panic. The
// wrapper is on every request, not only the streaming ones, so Flush
// has to be safe to call against anything.
func TestFlushIsSafeWhenTheWriterCannotFlush(t *testing.T) {
	rw := &recorder{ResponseWriter: &plainWriter{header: http.Header{}}, status: 200}
	rw.Flush() // must not panic
}

// plainWriter implements http.ResponseWriter and nothing else, unlike
// httptest.ResponseRecorder, which does implement Flush and so cannot
// stand in for a writer that does not.
type plainWriter struct {
	header http.Header
	status int
}

func (p *plainWriter) Header() http.Header         { return p.header }
func (p *plainWriter) Write(b []byte) (int, error) { return len(b), nil }
func (p *plainWriter) WriteHeader(code int)        { p.status = code }

// The status a handler writes still reaches the log line. Flushing
// changed the wrapper, and the thing the wrapper exists for must
// survive the change.
func TestStatusIsStillRecorded(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	rec := httptest.NewRecorder()
	h := withLogging(log, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	}))
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/x", nil))
	if rec.Code != http.StatusTeapot {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusTeapot)
	}
}

// The streaming path must actually escape the write deadline.
//
// This is two bugs in one place, and the second hid behind the first.
// http.ResponseController reaches optional behaviour by walking
// Unwrap() until it finds a writer that supports what it was asked
// for, so a wrapper without Unwrap stops the walk and SetWriteDeadline
// returns http.ErrNotSupported. The exemption would then compile, run,
// report nothing, and leave the 30 second deadline exactly where it
// was, which is the failure it exists to prevent.
func TestStreamingPathCanClearTheWriteDeadline(t *testing.T) {
	rw := &recorder{ResponseWriter: &deadlineWriter{header: http.Header{}}, status: 200}
	if err := http.NewResponseController(rw).SetWriteDeadline(time.Time{}); err != nil {
		t.Fatalf("SetWriteDeadline through the logging wrapper: %v", err)
	}
}

// deadlineWriter supports write deadlines, the way a real
// http.ResponseWriter from net/http does.
type deadlineWriter struct {
	header http.Header
	status int
	set    bool
}

func (d *deadlineWriter) Header() http.Header         { return d.header }
func (d *deadlineWriter) Write(b []byte) (int, error) { return len(b), nil }
func (d *deadlineWriter) WriteHeader(code int)        { d.status = code }
func (d *deadlineWriter) SetWriteDeadline(time.Time) error {
	d.set = true
	return nil
}
