// Package server wires HTTP handlers, ConnectRPC services, and lifecycle.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/build"
	"github.com/reh3376/career-site/services/api/internal/config"
	"github.com/reh3376/career-site/services/api/internal/db"
	"github.com/reh3376/career-site/services/api/internal/handlers"
	"github.com/reh3376/career-site/services/api/internal/sidecar"
	"github.com/reh3376/career-site/services/api/internal/users"
)

type Server struct {
	cfg      config.Config
	log      *slog.Logger
	http     *http.Server
	system   *handlers.System
	auth     *handlers.Auth
	member   *handlers.Member
	contact  *handlers.Contact
	decision *handlers.AdminDecision
	admin    *handlers.Admin
	activity *handlers.Activity
	jd       *handlers.Jd
	meetings *handlers.Meetings
	chat     *handlers.Chat
	events   *handlers.Events
	sidecar  *sidecar.Client
	db       *db.Pool
}

// Deps carries the process-level singletons the server wires into handlers.
// Passing them as a struct keeps New's signature stable as later phases add
// dependencies (session store, rate limiter, scheduler, ...).
type Deps struct {
	Sidecar  *sidecar.Client
	DB       *db.Pool
	Auth     *handlers.Auth
	Member   *handlers.Member
	Contact  *handlers.Contact
	Decision *handlers.AdminDecision
	Admin    *handlers.Admin
	Activity *handlers.Activity
	Jd       *handlers.Jd
	Meetings *handlers.Meetings
	// Chat is Ask Roger. Nil leaves the service unmounted, which is
	// what happens before the sidecar is reachable; the site is
	// unaffected except that the panel cannot open.
	Chat   *handlers.Chat
	Events *handlers.Events
	// Users backs the public reviewer status on SystemService; nil
	// leaves that endpoint answering Unavailable.
	Users *users.Repo
}

func New(cfg config.Config, log *slog.Logger, deps Deps) *Server {
	s := &Server{
		cfg:      cfg,
		log:      log,
		system:   handlers.NewSystem(),
		auth:     deps.Auth,
		member:   deps.Member,
		contact:  deps.Contact,
		decision: deps.Decision,
		admin:    deps.Admin,
		activity: deps.Activity,
		jd:       deps.Jd,
		meetings: deps.Meetings,
		chat:     deps.Chat,
		events:   deps.Events,
		sidecar:  deps.Sidecar,
		db:       deps.DB,
	}
	if deps.Users != nil {
		s.system.SetUsers(log, deps.Users)
	}
	s.http = &http.Server{
		Addr:         cfg.Addr,
		Handler:      s.routes(),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
	}
	return s
}

func (s *Server) routes() http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /api/healthz", s.healthz)
	mux.HandleFunc("GET /api/readyz", s.readyz)

	// Plain HTTP for one-click admin approval/decline — the token IS the
	// auth. Called from the /admin/decision Next.js page server-side.
	if s.decision != nil {
		mux.HandleFunc("POST /api/admin/decision", s.decision.Handle)
	}
	// Plain HTTP download of a generated résumé PDF; the result token
	// in `?t=` is the auth.
	if s.jd != nil {
		mux.HandleFunc("GET /api/jd/resume/{file}", s.jd.ServeResumePDF)
	}

	if s.meetings != nil {
		// A file a browser downloads, so plain HTTP rather than an RPC.
		mux.HandleFunc("GET /api/meetings/{file}", s.meetings.ServeMeetingICS)
	}

	mount := func(path string, h http.Handler) {
		mux.Handle("/api"+path, http.StripPrefix("/api", h))
	}

	systemPath, systemHandler := careerv1connect.NewSystemServiceHandler(s.system)
	mount(systemPath, systemHandler)

	if s.auth != nil {
		authPath, authHandler := careerv1connect.NewAuthServiceHandler(s.auth)
		mount(authPath, authHandler)
	}

	if s.member != nil {
		memberPath, memberHandler := careerv1connect.NewMemberServiceHandler(s.member)
		mount(memberPath, memberHandler)
	}

	if s.contact != nil {
		contactPath, contactHandler := careerv1connect.NewContactServiceHandler(s.contact)
		mount(contactPath, contactHandler)
	}

	if s.admin != nil {
		adminPath, adminHandler := careerv1connect.NewAdminServiceHandler(s.admin)
		mount(adminPath, adminHandler)
	}

	if s.activity != nil {
		activityPath, activityHandler := careerv1connect.NewActivityServiceHandler(s.activity)
		mount(activityPath, activityHandler)
	}

	if s.jd != nil {
		jdPath, jdHandler := careerv1connect.NewJdServiceHandler(s.jd)
		mount(jdPath, jdHandler)
	}

	if s.meetings != nil {
		meetingsPath, meetingsHandler := careerv1connect.NewMeetingServiceHandler(s.meetings)
		mount(meetingsPath, meetingsHandler)
	}

	if s.chat != nil {
		chatPath, chatHandler := careerv1connect.NewChatServiceHandler(s.chat)
		mount(chatPath, withoutWriteDeadline(chatHandler))
	}

	if s.events != nil {
		eventsPath, eventsHandler := careerv1connect.NewEventServiceHandler(s.events)
		mount(eventsPath, eventsHandler)
	}

	// Every RPC body is small (the JD text is capped at 50k characters
	// by the proto); a 1 MB cap stops oversized bodies from being read
	// into memory and hashed. Verified live on 2026-09-22 that a 2 MB
	// body to Login was processed before this.
	return withLogging(s.log, http.MaxBytesHandler(mux, 1<<20))
}

func (s *Server) Start() error {
	s.log.Info("api listening",
		slog.String("addr", s.cfg.Addr),
		slog.String("env", s.cfg.Env),
		slog.String("version", build.Version),
		slog.String("sidecar_addr", s.cfg.SidecarAddr),
	)
	if err := s.http.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	shutdownCtx, cancel := context.WithTimeout(ctx, s.cfg.ShutdownTimeout)
	defer cancel()
	return s.http.Shutdown(shutdownCtx)
}

type healthPayload struct {
	Status  string          `json:"status"`
	Version string          `json:"version,omitempty"`
	Checks  map[string]bool `json:"checks,omitempty"`
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthPayload{Status: "ok"})
}

// readyz reports readiness of the API's downstream dependencies. Phase 0
// wires the sidecar and postgres; object storage joins later.
// Returns 200 when every check passes, 503 with the same body otherwise.
func (s *Server) readyz(w http.ResponseWriter, r *http.Request) {
	checks := map[string]bool{
		"sidecar":  s.sidecarReady(r.Context()),
		"postgres": s.postgresReady(r.Context()),
	}
	allOk := true
	for _, ok := range checks {
		if !ok {
			allOk = false
			break
		}
	}
	body := healthPayload{Version: build.Version, Checks: checks}
	if allOk {
		body.Status = "ok"
		writeJSON(w, http.StatusOK, body)
	} else {
		body.Status = "unready"
		writeJSON(w, http.StatusServiceUnavailable, body)
	}
}

func (s *Server) sidecarReady(ctx context.Context) bool {
	if s.sidecar == nil {
		return false
	}
	resp, err := s.sidecar.Health(ctx)
	if err != nil {
		s.log.Warn("sidecar health check failed", slog.String("error", err.Error()))
		return false
	}
	return resp.GetReady()
}

func (s *Server) postgresReady(ctx context.Context) bool {
	if s.db == nil {
		return false
	}
	pingCtx, cancel := context.WithTimeout(ctx, s.cfg.DBTimeout)
	defer cancel()
	if err := s.db.Ping(pingCtx); err != nil {
		s.log.Warn("postgres health check failed", slog.String("error", err.Error()))
		return false
	}
	return true
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func withLogging(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &recorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rw, r)
		log.Info("http",
			slog.String("method", r.Method),
			slog.String("path", sanitizeForLog(r.URL.Path)),
			slog.Int("status", rw.status),
			slog.Duration("elapsed", time.Since(start)),
		)
	})
}

// sanitizeForLog strips CR/LF from a value before it goes into a
// structured-log field. An attacker who controls URL path bytes (a
// crafted request against the plain-HTTP mux) could otherwise inject
// a newline and forge a fake log line that log aggregators would
// treat as a separate record — CodeQL js/nodejs-log-injection style.
// slog's JSON handler already escapes special characters, so this is
// belt-and-suspenders against a future switch to a text handler.
func sanitizeForLog(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// withoutWriteDeadline takes the streamed answer out from under the
// server's write deadline.
//
// http.Server.WriteTimeout is 30 seconds and covers the whole response,
// which is right for every unary RPC on this mux and fatally wrong for
// one that streams. An Ask Roger answer is ten to twenty-five seconds
// to the first token on this hardware and can run past forty in total.
// The first real question asked on production took 44.9 seconds: the
// handler finished and logged 200, and the browser had been cut off at
// thirty and showed "network error".
//
// Only SendMessage is exempted. Clearing the deadline for the whole
// service would drop the protection from six quick RPCs to buy nothing,
// and clearing it globally would drop it from the entire API.
//
// The deadline is cleared rather than lengthened, because the useful
// bound on a streaming handler is the request context, which already
// carries the client going away. A second, longer wall-clock timeout
// would only be a slower version of the same bug.
func withoutWriteDeadline(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == careerv1connect.ChatServiceSendMessageProcedure {
			// Errors are ignored deliberately: a transport that cannot
			// do this is one where the deadline is not being enforced
			// either, so there is nothing to fix and nothing to report.
			_ = http.NewResponseController(w).SetWriteDeadline(time.Time{})
		}
		next.ServeHTTP(w, r)
	})
}

// Unwrap exposes the writer underneath, for http.ResponseController.
//
// Same trap as Flush below, one layer further out. ResponseController
// reaches optional behaviour (deadlines, flushing) by walking Unwrap()
// until it finds a writer that supports what it was asked for, and a
// wrapper without Unwrap is where that walk stops. Without this,
// SetWriteDeadline on a streaming handler returns
// http.ErrNotSupported and the long-answer fix below silently does
// nothing.
func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// Flush passes through to the real writer.
//
// Without this every server-streaming RPC fails. Embedding
// http.ResponseWriter gives this type the interface's methods but not
// the optional ones the concrete writer also implements, so wrapping
// the writer silently removed http.Flusher from it. connect-go checks
// for that on a streaming handler and refuses the call:
//
//	*server.recorder does not implement http.Flusher
//
// The only streaming RPC is ChatService.SendMessage, so this was the
// whole of Ask Roger failing with an internal error, on every call,
// while every unary RPC on the same mux worked perfectly. Found by
// sending a real Connect stream frame at it; nothing in the Go tests
// or the browser build would have shown it, because the wrapper
// satisfies http.ResponseWriter and compiles fine.
//
// Flushing is also what makes streaming worth having: it is what puts
// each frame on the wire as it is produced rather than at the end.
func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
