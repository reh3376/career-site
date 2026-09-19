// Command api runs the career-site HTTP API.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/config"
	"github.com/reh3376/career-site/services/api/internal/db"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/handlers"
	"github.com/reh3376/career-site/services/api/internal/scheduler"
	"github.com/reh3376/career-site/services/api/internal/server"
	"github.com/reh3376/career-site/services/api/internal/sidecar"
	"github.com/reh3376/career-site/services/api/internal/users"
)

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		log.Error("config load failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if !cfg.SkipMigrate {
		log.Info("running migrations")
		if err := db.Migrate(ctx, cfg.DatabaseURL); err != nil {
			log.Error("migrate failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	pool, err := db.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("db open failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer pool.Close()

	sc, err := sidecar.Dial(cfg.SidecarAddr, cfg.SidecarTimeout)
	if err != nil {
		log.Error("sidecar client init failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer sc.Close()

	mailer, err := email.NewFromConfig(email.Config{
		Provider:  cfg.EmailProvider,
		From:      cfg.MailFrom,
		SMTPHost:  cfg.SMTPHost,
		SMTPPort:  cfg.SMTPPort,
		SMTPUser:  cfg.SMTPUser,
		SMTPPass:  cfg.SMTPPass,
		ResendKey: cfg.ResendAPIKey,
	})
	if err != nil {
		log.Error("email provider init failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("email provider ready", slog.String("provider", mailer.Name()))

	var pwned auth.PwnedChecker = auth.NoopPwnedChecker{}
	if cfg.PwnedCheckEnabled {
		pwned = auth.NewHIBPChecker()
		log.Info("HIBP pwned-password check enabled")
	}

	userRepo := users.New(pool.Pool)
	authHandler := handlers.NewAuth(log, userRepo, mailer, pwned, handlers.AuthConfig{
		WebBaseURL:        cfg.WebBaseURL,
		OwnerContactEmail: cfg.OwnerContactEmail,
		MailFrom:          cfg.MailFrom,
		ConsentVersion:    cfg.ConsentVersion,
	})

	srv := server.New(cfg, log, server.Deps{
		Sidecar: sc,
		DB:      pool,
		Auth:    authHandler,
	})

	// Expiry jobs run in-process; interval intentionally low for dev so a
	// manually-set expires_at gets picked up quickly. Prod overrides via env
	// once the tests confirm behaviour.
	expiry := handlers.NewExpiryJobs(log, userRepo, mailer, cfg.MailFrom, cfg.OwnerContactEmail)
	sched := scheduler.New(log,
		scheduler.Job{Name: "expiry-warn", Interval: cfg.ExpirySchedulerInterval, Run: expiry.WarnJob},
		scheduler.Job{Name: "expiry-cut", Interval: cfg.ExpirySchedulerInterval, Run: expiry.ExpireJob},
	)
	sched.Start(ctx)
	defer sched.Stop()

	errCh := make(chan error, 1)
	go func() { errCh <- srv.Start() }()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		if err != nil {
			log.Error("server exited with error", slog.String("error", err.Error()))
			os.Exit(1)
		}
	}

	if err := srv.Shutdown(context.Background()); err != nil {
		log.Error("shutdown failed", slog.String("error", err.Error()))
		os.Exit(1)
	}
	log.Info("shutdown complete")
}
