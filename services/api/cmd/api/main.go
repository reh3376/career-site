// Command api runs the career-site HTTP API.
package main

import (
	"context"
	"crypto/rand"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/reh3376/career-site/services/api/internal/auth"
	"github.com/reh3376/career-site/services/api/internal/config"
	"github.com/reh3376/career-site/services/api/internal/db"
	"github.com/reh3376/career-site/services/api/internal/email"
	"github.com/reh3376/career-site/services/api/internal/handlers"
	"github.com/reh3376/career-site/services/api/internal/ingest"
	"github.com/reh3376/career-site/services/api/internal/jd"
	"github.com/reh3376/career-site/services/api/internal/llm"
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

	// Read-only role for the /admin/db console. When
	// DB_READONLY_PASSWORD is set, flip the role created by migration
	// 00005 to LOGIN + password. Then, if DATABASE_URL_READONLY is
	// set, open a second pool that connects with that role — the SQL
	// console runs through it so a bug in the SELECT-only guard can't
	// write anything. In dev, both are unset and the console falls
	// back to the main pool with a warn.
	var readonlyPool *db.Pool
	if cfg.DBReadonlyPassword != "" {
		if err := pool.SetReadonlyRolePassword(ctx, cfg.DBReadonlyPassword); err != nil {
			log.Error("set readonly role password failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		log.Info("readonly role password synced")
	}
	if cfg.DatabaseURLReadonly != "" {
		readonlyPool, err = db.Open(ctx, cfg.DatabaseURLReadonly)
		if err != nil {
			log.Error("readonly db open failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		defer readonlyPool.Close()
		log.Info("admin/db surface using dedicated readonly pool")
	} else {
		readonlyPool = pool
		log.Warn("admin/db surface using write-capable pool — set DATABASE_URL_READONLY in prod")
	}

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

	// Decision-token secret. Prod supplies a stable value in
	// DECISION_TOKEN_SECRET so existing links keep working across restarts;
	// dev boots with a random one and warns.
	if len(cfg.DecisionTokenSecret) == 0 {
		buf := make([]byte, 32)
		if _, err := rand.Read(buf); err != nil {
			log.Error("generate decision-token secret failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		cfg.DecisionTokenSecret = buf
		log.Warn("DECISION_TOKEN_SECRET not set — generated a random one; existing email links will not survive a restart")
	}

	userRepo := users.New(pool.Pool)

	// Every outbound email is audited to notification_deliveries so
	// the admin console can show per-member delivery history and
	// offer a resend. Wrapped here, after the repo exists, and handed
	// to every handler in place of the raw provider.
	mailer = email.Audited{Inner: mailer, Sink: userRepo, Log: log}

	// Admin bootstrap. When ADMIN_USERNAME + CAREER_SITE_ADMIN_PW are set,
	// ensure the row exists in role=admin, status=active so Roger can sign
	// in on the very first deploy. See users.EnsureAdmin for the upsert.
	if cfg.AdminEmail != "" && cfg.AdminPassword != "" {
		hash, err := auth.HashPassword(cfg.AdminPassword)
		if err != nil {
			log.Error("hash admin password failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		if err := userRepo.EnsureAdmin(ctx, cfg.AdminEmail, "Roger Henley", hash); err != nil {
			log.Error("ensure admin failed", slog.String("error", err.Error()))
			os.Exit(1)
		}
		log.Info("admin bootstrap ok", slog.String("email", cfg.AdminEmail))
	}

	authHandler := handlers.NewAuth(log, userRepo, mailer, pwned, handlers.AuthConfig{
		WebBaseURL:          cfg.WebBaseURL,
		OwnerContactEmail:   cfg.OwnerContactEmail,
		MailFrom:            cfg.MailFrom,
		ConsentVersion:      cfg.ConsentVersion,
		DecisionTokenTTL:    cfg.DecisionTokenTTL,
		DecisionTokenSecret: cfg.DecisionTokenSecret,
		SessionTTL:          cfg.SessionTTL,
		CookieSecure:        cfg.CookieSecure,
	})
	memberHandler := handlers.NewMember(authHandler, userRepo)
	contactHandler := handlers.NewContact(
		log, userRepo, authHandler, mailer,
		cfg.MailFrom, cfg.OwnerContactEmail, cfg.WebBaseURL,
	)
	decisionHandler := handlers.NewAdminDecision(
		log, userRepo, mailer, cfg.DecisionTokenSecret,
		cfg.MailFrom, cfg.OwnerContactEmail, cfg.WebBaseURL,
	)
	// Ask Roger corpus ingester. Sidecar-adapted; nil sidecar
	// (e.g. dev without a live sidecar) still lets Admin surface
	// non-corpus routes since IngestCorpusText guards on a nil
	// ingester and returns Unavailable.
	var ingester *ingest.Ingester
	if sc != nil {
		ingester = ingest.NewIngester(
			log, userRepo, ingest.SidecarEmbed{Client: sc}, nil,
		)
	}
	activityHandler := handlers.NewActivity(log, userRepo, authHandler)
	// JD scorer reuses the sidecar's embedder. Skipped when the
	// sidecar isn't dialled (rare — dev only) so /jd-upload still
	// stores submissions even without scoring wired.
	var jdScorer *jd.Scorer
	if sc != nil {
		// The requirement-judgment assessor only makes sense with a
		// real LLM behind the sidecar. The stub provider produces
		// schema-valid but meaningless output, so on stub the retrieval
		// pre-score stays the gate unless LLM_ALLOW_STUB is set (CI and
		// end-to-end exercises of the pipeline).
		var assessor *jd.Assessor
		var writer *jd.ResumeWriter
		provider := ""
		if h, err := sc.Health(ctx); err == nil {
			provider = h.LlmProvider
		}
		if provider != "" && (cfg.LLMAllowStub || !strings.HasPrefix(provider, "stub")) {
			gateway := llm.SidecarLLM{Client: sc}
			assessor = jd.NewAssessor(log, userRepo, ingest.SidecarEmbed{Client: sc}, gateway, cfg.LLMMonthlyCallCap, cfg.LLMNumCtx)
			writer = jd.NewResumeWriter(log, userRepo, gateway, cfg.LLMMonthlyCallCap,
				llm.SidecarRenderer{Client: sc}, cfg.ResumePDFOwnerPassword, cfg.LLMNumCtx)
			log.Info("jd assessor + résumé writer enabled",
				slog.String("llm_provider", provider),
				slog.Int("num_ctx", cfg.LLMNumCtx),
				slog.Bool("pdf", cfg.ResumePDFOwnerPassword != ""))
		} else {
			log.Info("jd assessor disabled; retrieval score is the gate", slog.String("llm_provider", provider))
		}
		jdScorer = jd.NewScorer(log, userRepo, ingest.SidecarEmbed{Client: sc}, assessor, writer, jd.NewBandsStore(log, userRepo, jd.DefaultBands(cfg.JDMatchThreshold)), cfg.JDPipelineTimeout)
		jdScorer.SetNotifier(jd.NewOwnerMailer(mailer, userRepo, cfg.MailFrom, cfg.OwnerContactEmail, cfg.WebBaseURL, log))
	}
	jdHandler := handlers.NewJd(log, userRepo, authHandler, jdScorer, cfg.JDPipelineTimeout)
	// Admin comes after the JD scorer so RescoreJd can reuse it.
	adminHandler := handlers.NewAdmin(
		log, userRepo, authHandler, decisionHandler, pool, readonlyPool, ingester,
		handlers.CorpusRoots{Public: cfg.CorpusRoot, Private: cfg.CorpusPrivateRoot},
		jdScorer, cfg.JDPipelineTimeout,
	)

	srv := server.New(cfg, log, server.Deps{
		Sidecar:  sc,
		DB:       pool,
		Auth:     authHandler,
		Member:   memberHandler,
		Contact:  contactHandler,
		Decision: decisionHandler,
		Admin:    adminHandler,
		Activity: activityHandler,
		Jd:       jdHandler,
	})

	// Expiry + auto-decline jobs run in-process; interval configurable so
	// tests can drive them quickly.
	expiry := handlers.NewExpiryJobs(log, userRepo, mailer, cfg.MailFrom, cfg.OwnerContactEmail)
	autoDecline := handlers.NewAutoDeclineJobs(log, userRepo, decisionHandler, cfg.PendingApprovalTTL)
	sched := scheduler.New(log,
		scheduler.Job{Name: "expiry-warn", Interval: cfg.ExpirySchedulerInterval, Run: expiry.WarnJob},
		scheduler.Job{Name: "expiry-cut", Interval: cfg.ExpirySchedulerInterval, Run: expiry.ExpireJob},
		scheduler.Job{Name: "auto-decline", Interval: cfg.ExpirySchedulerInterval, Run: autoDecline.Run},
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
