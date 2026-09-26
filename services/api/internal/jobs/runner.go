// Package jobs is a small in-process runner for admin operations that
// outlive an RPC: corpus reindexes and embed sweeps take minutes on the
// production box's CPU, longer than the proxy and the api's write
// timeout allow a response to wait. RunJob starts one and returns an
// id; GetJob reports progress until it finishes. State lives in
// memory: a restart forgets running jobs, which is acceptable because
// every job here is idempotent and can simply be started again.
package jobs

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"sync"
	"time"
)

// Status mirrors the proto JobStatus vocabulary.
type Status string

const (
	StatusQueued    Status = "queued"
	StatusRunning   Status = "running"
	StatusSucceeded Status = "succeeded"
	StatusFailed    Status = "failed"
)

// Job is one run.
type Job struct {
	ID         string
	Kind       string
	Status     Status
	Progress   int32 // percent, 0 when the job does not report it
	Summary    string
	StartedAt  time.Time
	FinishedAt *time.Time
	// Events is every progress report the job made, in order.
	//
	// Summary holds only the latest, because each report overwrites it,
	// and for a job measured in hours that is the least useful line: an
	// evaluation says "posting 4 of 9: scoring" having already discarded
	// that the first three took 28, 31 and 26 minutes. The pace is what
	// separates a healthy run from one that is merely alive, and it was
	// being thrown away as it arrived.
	Events []Event
}

// Event is one progress report, kept with the time it arrived so the
// gaps between reports can be read. The gap is the measurement; the
// text is only how the job describes what it was doing.
type Event struct {
	At       time.Time
	Progress int32
	Summary  string
}

// MaxEvents bounds the history per job. An evaluation runs about four
// hours and reports a few times per posting, so this is generous. The
// cap exists because a job reporting in a tight loop should not be able
// to exhaust memory on a box that also has to run the model.
const MaxEvents = 500

// Report lets a job publish progress while it runs.
type Report func(pct int32, summary string)

// Fn is the body of a job. It returns the final summary.
type Fn func(ctx context.Context, report Report) (summary string, err error)

// ErrAlreadyRunning is returned when a job of the same kind is running.
var ErrAlreadyRunning = errors.New("a job of this kind is already running")

// Runner keeps jobs in memory and runs them one goroutine each.
type Runner struct {
	log     *slog.Logger
	mu      sync.Mutex
	jobs    map[string]*Job
	retain  time.Duration
	timeout time.Duration
}

// New makes a runner. Finished jobs are forgotten after retain; a job
// that runs longer than timeout is cancelled and marked failed.
func New(log *slog.Logger, retain, timeout time.Duration) *Runner {
	if retain <= 0 {
		retain = 24 * time.Hour
	}
	if timeout <= 0 {
		timeout = 2 * time.Hour
	}
	return &Runner{log: log, jobs: map[string]*Job{}, retain: retain, timeout: timeout}
}

// Start begins a job of kind, refusing a second concurrent one of the
// same kind (reindexing the same directory twice at once has no use
// and doubles the load on the embedder).
func (r *Runner) Start(kind string, fn Fn) (*Job, error) {
	return r.StartWithin(kind, 0, fn)
}

// StartWithin is Start with its own deadline, for a job whose honest
// duration is not the default.
//
// The default of two hours suits work that is bounded by the size of
// the corpus. An evaluation is bounded by the language model instead:
// it scores every posting in the golden set through the real pipeline,
// one judge call per requirement, and on this box a single posting
// takes the better part of an hour. A five-posting set ran into the
// two-hour ceiling on 2026-09-23 and lost two hours of work three
// postings in, which is the whole reason this exists.
func (r *Runner) StartWithin(kind string, timeout time.Duration, fn Fn) (*Job, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pruneLocked()
	for _, j := range r.jobs {
		if j.Kind == kind && (j.Status == StatusQueued || j.Status == StatusRunning) {
			return nil, ErrAlreadyRunning
		}
	}
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		return nil, err
	}
	j := &Job{ID: hex.EncodeToString(b[:]), Kind: kind, Status: StatusQueued, StartedAt: time.Now().UTC()}
	r.jobs[j.ID] = j
	if timeout <= 0 {
		timeout = r.timeout
	}
	go r.run(j, timeout, fn)
	return snapshot(j), nil
}

func (r *Runner) run(j *Job, timeout time.Duration, fn Fn) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	r.set(j, func(x *Job) { x.Status = StatusRunning; x.StartedAt = time.Now().UTC() })
	report := func(pct int32, summary string) {
		r.set(j, func(x *Job) {
			if pct >= 0 && pct <= 100 {
				x.Progress = pct
			}
			if summary != "" {
				x.Summary = summary
			}
			// Append rather than replace. When the cap is reached the
			// oldest go first: a long run's recent history is what is
			// being read, and dropping the tail instead would leave the
			// start of a run visible and the part you are watching gone.
			x.Events = append(x.Events, Event{At: time.Now().UTC(), Progress: pct, Summary: summary})
			if len(x.Events) > MaxEvents {
				x.Events = x.Events[len(x.Events)-MaxEvents:]
			}
		})
	}
	summary, err := fn(ctx, report)
	now := time.Now().UTC()
	r.set(j, func(x *Job) {
		x.FinishedAt = &now
		if err != nil {
			x.Status = StatusFailed
			x.Summary = err.Error()
			// The ending belongs in the timeline too. A history that
			// stops at the last progress report leaves the reader
			// guessing whether the job finished or stopped being
			// observed, which is exactly the question it is there to
			// answer.
			x.Events = append(x.Events, Event{At: now, Progress: x.Progress, Summary: "failed: " + err.Error()})
			return
		}
		x.Status = StatusSucceeded
		x.Progress = 100
		if summary != "" {
			x.Summary = summary
		}
		x.Events = append(x.Events, Event{At: now, Progress: 100, Summary: summary})
	})
	if err != nil {
		r.log.Warn("job failed", slog.String("job", j.ID), slog.String("kind", j.Kind), slog.String("error", err.Error()))
		return
	}
	r.log.Info("job finished", slog.String("job", j.ID), slog.String("kind", j.Kind), slog.Duration("took", now.Sub(j.StartedAt)))
}

func (r *Runner) set(j *Job, mut func(*Job)) {
	r.mu.Lock()
	defer r.mu.Unlock()
	mut(j)
}

// Get returns a copy of a job, or false.
func (r *Runner) Get(id string) (*Job, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	j, ok := r.jobs[id]
	if !ok {
		return nil, false
	}
	return snapshot(j), true
}

// Recent lists jobs newest first, for an admin surface.
func (r *Runner) Recent(limit int) []*Job {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]*Job, 0, len(r.jobs))
	for _, j := range r.jobs {
		out = append(out, snapshot(j))
	}
	for i := 1; i < len(out); i++ {
		for k := i; k > 0 && out[k].StartedAt.After(out[k-1].StartedAt); k-- {
			out[k], out[k-1] = out[k-1], out[k]
		}
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}

func (r *Runner) pruneLocked() {
	cut := time.Now().Add(-r.retain)
	for id, j := range r.jobs {
		if j.FinishedAt != nil && j.FinishedAt.Before(cut) {
			delete(r.jobs, id)
		}
	}
}

func snapshot(j *Job) *Job {
	c := *j
	if j.FinishedAt != nil {
		t := *j.FinishedAt
		c.FinishedAt = &t
	}
	// Events must be copied, not aliased. `c := *j` copies the slice
	// header, so the caller would share a backing array that the job's
	// own goroutine keeps appending to: a read while the runner appends
	// is a data race, and a re-slice after the cap is reached would
	// shift the caller's view under it. Everything else here is a value.
	if j.Events != nil {
		c.Events = make([]Event, len(j.Events))
		copy(c.Events, j.Events)
	}
	return &c
}
