// Package scheduler runs recurring background jobs in the API process.
// Phase 1 owns the whitelist/expiry pair; later phases add GitHub sync,
// digests, backups, and retention.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

type Job struct {
	Name     string
	Interval time.Duration
	// Run is called with a per-tick context bounded by a per-job deadline.
	Run func(ctx context.Context) error
}

type Scheduler struct {
	log      *slog.Logger
	jobs     []Job
	deadline time.Duration
	wg       sync.WaitGroup
	cancel   context.CancelFunc
}

// New builds a scheduler with a per-tick job deadline (default 5 minutes).
func New(log *slog.Logger, jobs ...Job) *Scheduler {
	return &Scheduler{
		log:      log,
		jobs:     jobs,
		deadline: 5 * time.Minute,
	}
}

// Start launches every registered job. Returns immediately; jobs run in
// background goroutines until Stop is called.
func (s *Scheduler) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel

	for _, j := range s.jobs {
		s.wg.Add(1)
		go s.runJob(ctx, j)
	}
	s.log.Info("scheduler started", slog.Int("jobs", len(s.jobs)))
}

// Stop signals every job goroutine to exit and waits for them.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) runJob(ctx context.Context, j Job) {
	defer s.wg.Done()
	ticker := time.NewTicker(j.Interval)
	defer ticker.Stop()

	fire := func() {
		tickCtx, cancel := context.WithTimeout(ctx, s.deadline)
		defer cancel()
		start := time.Now()
		if err := j.Run(tickCtx); err != nil {
			s.log.Warn("job error",
				slog.String("job", j.Name),
				slog.Duration("elapsed", time.Since(start)),
				slog.String("error", err.Error()),
			)
			return
		}
		s.log.Debug("job ok",
			slog.String("job", j.Name),
			slog.Duration("elapsed", time.Since(start)),
		)
	}

	// Fire once at start so a job that runs infrequently still runs on
	// boot rather than waiting a full interval.
	fire()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			fire()
		}
	}
}
