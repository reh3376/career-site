// Package server wires HTTP handlers, ConnectRPC services, and lifecycle.
package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"github.com/reh3376/career-site/services/api/gen/career/v1/careerv1connect"
	"github.com/reh3376/career-site/services/api/internal/build"
	"github.com/reh3376/career-site/services/api/internal/config"
	"github.com/reh3376/career-site/services/api/internal/db"
	"github.com/reh3376/career-site/services/api/internal/handlers"
	"github.com/reh3376/career-site/services/api/internal/sidecar"
)

type Server struct {
	cfg     config.Config
	log     *slog.Logger
	http    *http.Server
	system  *handlers.System
	sidecar *sidecar.Client
	db      *db.Pool
}

func New(cfg config.Config, log *slog.Logger, sc *sidecar.Client, pool *db.Pool) *Server {
	s := &Server{
		cfg:     cfg,
		log:     log,
		system:  handlers.NewSystem(),
		sidecar: sc,
		db:      pool,
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

	systemPath, systemHandler := careerv1connect.NewSystemServiceHandler(s.system)
	mux.Handle("/api"+systemPath, http.StripPrefix("/api", systemHandler))

	return withLogging(s.log, mux)
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

// readyz reports readiness of the API's downstream dependencies. Phase 0 only
// wires the sidecar; postgres and object storage join in later phases.
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
			slog.String("path", r.URL.Path),
			slog.Int("status", rw.status),
			slog.Duration("elapsed", time.Since(start)),
		)
	})
}

type recorder struct {
	http.ResponseWriter
	status int
}

func (r *recorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}
