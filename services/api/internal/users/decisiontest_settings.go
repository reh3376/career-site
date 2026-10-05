package users

import (
	"context"
	"encoding/json"
	"fmt"
)

// Timings for the decision test, owner-editable rather than compiled in.
//
// The first two live runs each moved these, and each move cost a build
// and a deploy to change one number. They belong in the database for the
// same reason the JD fit bands do: they are a judgement the owner makes
// from watching people take it, not a property of the code.
//
// # The version is derived, deliberately
//
// Changing a timing changes the instrument. A question answered in 20
// seconds and the same question answered in 25 are not the same
// measurement, and pooling them would be the quiet kind of mistake that
// never announces itself.
//
// So InstrumentVersion() is computed FROM the timings rather than
// maintained alongside them. It is impossible to change a timing and
// forget to change the version, because there is no separate version to
// forget.

// DTSettingsKey is the app_settings row these live in.
const DTSettingsKey = "decision_test_timings"

// DTSettings is what the owner can change.
type DTSettings struct {
	// How long the number to hold is shown. Two seconds was the original
	// specification and proved unreadable; raised to five after the first
	// live run and six after the second.
	MemoriseMs int `json:"memorise_ms"`
	// Hard limit per question, covering reading, deciding and rating.
	// Twenty by arithmetic, twenty-five by the owner having taken it.
	QuestionMs int `json:"question_ms"`
	// Limit on entering the number at the end of a block.
	RecallMs int `json:"recall_ms"`
}

// DTDefaultSettings are the values as of the owner's second live run.
func DTDefaultSettings() DTSettings {
	return DTSettings{MemoriseMs: 6000, QuestionMs: 25000, RecallMs: 20000}
}

// InstrumentVersion identifies the instrument these timings describe.
//
// Derived rather than stored, so a timing change cannot be made without
// the data recording that it happened.
func (s DTSettings) InstrumentVersion() string {
	return fmt.Sprintf("v2-m%d-q%d-r%d", s.MemoriseMs, s.QuestionMs, s.RecallMs)
}

// sane clamps values that would break the instrument rather than tune
// it. A question limit under five seconds is not a hard test, it is an
// unanswerable one, and a memorise window under one second shows nothing.
func (s DTSettings) sane() DTSettings {
	clamp := func(v, lo, hi, def int) int {
		if v < lo || v > hi {
			return def
		}
		return v
	}
	d := DTDefaultSettings()
	return DTSettings{
		MemoriseMs: clamp(s.MemoriseMs, 1000, 30000, d.MemoriseMs),
		QuestionMs: clamp(s.QuestionMs, 5000, 120000, d.QuestionMs),
		RecallMs:   clamp(s.RecallMs, 5000, 120000, d.RecallMs),
	}
}

// DTGetSettings reads the timings, falling back to the defaults. A
// missing or unreadable row is not an error: the test must still run.
func (r *Repo) DTGetSettings(ctx context.Context) DTSettings {
	raw, ok, err := r.GetSetting(ctx, DTSettingsKey)
	if err != nil || !ok {
		return DTDefaultSettings()
	}
	var s DTSettings
	if json.Unmarshal(raw, &s) != nil {
		return DTDefaultSettings()
	}
	return s.sane()
}

// DTSetSettings stores the timings, clamped.
func (r *Repo) DTSetSettings(ctx context.Context, s DTSettings, by int64) error {
	v, err := json.Marshal(s.sane())
	if err != nil {
		return fmt.Errorf("decision test settings: %w", err)
	}
	if err := r.SetSetting(ctx, DTSettingsKey, v, by); err != nil {
		return fmt.Errorf("decision test settings: %w", err)
	}
	return nil
}

// DTSessionsOnVersion counts the runs already recorded under one
// instrument version.
//
// Used by the admin surface to show what a timing change costs: every
// session before it belongs to a different instrument, and the two
// cannot honestly be pooled.
func (r *Repo) DTSessionsOnVersion(ctx context.Context, version string) (int, error) {
	var n int
	err := r.pool.QueryRow(ctx,
		`SELECT count(*) FROM dt_sessions
		  WHERE instrument_version = $1 AND NOT is_synthetic`, version).Scan(&n)
	if err != nil {
		return 0, fmt.Errorf("decision test: count sessions on %s: %w", version, err)
	}
	return n, nil
}
