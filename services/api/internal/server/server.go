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
	"github.com/reh3376/career-site/services/api/internal/handlers"
)

type Server struct {
	cfg    config.Config
	log    *slog.Logger
	http   *http.Server
	system *handlers.System
}

func New(cfg config.Config, log *slog.Logger) *Server {
	s := &Server{
		cfg:    cfg,
		log:    log,
		system: handlers.NewSystem(),
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
	Status  string `json:"status"`
	Version string `json:"version,omitempty"`
}

func (s *Server) healthz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthPayload{Status: "ok"})
}

// readyz will fan out to database, object storage, and sidecar checks in a
// later phase; today it only reports process liveness plus the build version.
func (s *Server) readyz(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthPayload{Status: "ok", Version: build.Version})
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
