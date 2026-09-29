package jd

import (
	"strings"
	"sync"
	"time"
)

// phaseTimer answers "where did the time go" for one pipeline run.
//
// jd_runs already records queued_ms and duration_ms, which separate
// waiting from working but say nothing about what the work was. A
// posting that takes fifty minutes and one that takes four hours look
// the same in that record, and the only way to tell them apart has
// been to read llm_usage timestamps and subtract. That does not work
// for retrieval, which makes no model call and so leaves no row at
// all, and it does not work at all for a run that failed, which is the
// run most worth understanding.
//
// The stages are already reported: the pipeline calls progress at every
// boundary so the member can watch their review. This listens to the
// same reports and accumulates the time between them, so nothing new
// has to be threaded through the pipeline and a phase cannot be
// forgotten at a boundary that already reports.
//
// A report marks the START of a stage, so the time attributed to a
// phase runs from its own report to the next one. That is the same
// convention the job timeline uses (docs/evaluation.md).
type phaseTimer struct {
	mu   sync.Mutex
	last time.Time
	cur  string
	ms   map[string]int64
}

func newPhaseTimer(start time.Time) *phaseTimer {
	return &phaseTimer{last: start, cur: phaseSetup, ms: map[string]int64{}}
}

// enter closes the running phase and starts the one this stage belongs
// to. Consecutive stages in the same phase, such as the fourteen
// judging reports, accumulate into one entry rather than fourteen.
func (p *phaseTimer) enter(stage string) {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked(time.Now())
	p.cur = phaseFor(stage)
}

// stop closes the final phase. Safe to call more than once; the second
// call adds nothing, which matters because it runs from a defer that
// also fires on the failure path.
func (p *phaseTimer) stop() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.closeLocked(time.Now())
	p.cur = ""
}

func (p *phaseTimer) closeLocked(now time.Time) {
	if p.cur != "" && !p.last.IsZero() {
		if d := now.Sub(p.last).Milliseconds(); d > 0 {
			p.ms[p.cur] += d
		}
	}
	p.last = now
}

// snapshot copies what has been measured so far. Safe to call while the
// run is still going, which is how a failed run keeps its timings: the
// deferred close reads it after the error has already returned.
func (p *phaseTimer) snapshot() map[string]int64 {
	if p == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.ms) == 0 {
		return nil
	}
	out := make(map[string]int64, len(p.ms))
	for k, v := range p.ms {
		out[k] = v
	}
	return out
}

// The phases. Deliberately coarse: these are the questions actually
// asked of a slow run, and a finer split would be answered from
// llm_usage anyway.
const (
	phaseSetup        = "setup"
	phasePostingCheck = "posting_check"
	phaseExtract      = "extract"
	phaseRetrieval    = "retrieval"
	phaseJudge        = "judge"
	phaseScore        = "score"
	phaseResume       = "resume"
	phaseRender       = "render"
	phaseOther        = "other"
)

// stagePhases maps the stage text the pipeline reports to its phase, by
// prefix. Keep it beside the calls that produce these strings:
// Scorer.ScoreAndPersist and Assessor.Assess.
//
// A reworded stage falls to "other" rather than being folded into a
// neighbour, so the mapping going stale shows up as an "other" bucket
// in the data instead of quietly inflating whichever phase happened to
// be adjacent.
var stagePhases = []struct {
	prefix string
	phase  string
}{
	{"starting", phaseSetup},
	{"queued", phaseSetup},
	{"checking the posting", phasePostingCheck},
	{"reading the posting", phaseExtract},
	{"requirements found", phaseRetrieval}, // "12 requirements found; gathering evidence"
	{"gathering evidence", phaseRetrieval},
	{"judging requirement", phaseJudge},
	{"computing the score", phaseScore},
	{"writing the tailored", phaseResume},
	{"rendering the", phaseRender},
}

func phaseFor(stage string) string {
	s := strings.ToLower(strings.TrimSpace(stage))
	for _, m := range stagePhases {
		if strings.HasPrefix(s, m.prefix) || strings.Contains(s, m.prefix) {
			return m.phase
		}
	}
	return phaseOther
}
