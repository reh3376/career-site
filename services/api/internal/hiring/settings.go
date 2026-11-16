package hiring

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"
)

// Settings are the framework's adjustable values: today the share of
// its weight a preferred requirement counts at (FR-HF-23, "adjustable
// in the admin console"). One row in app_settings, read with a short
// cache so an edit takes effect on the next run without a restart, in
// the manner of the fit bands.
type Settings struct {
	// PreferredFraction is the share of its weight a preferred item, and
	// an evidenced ideal one, counts at. Greater than 0, at most 1.
	PreferredFraction float64 `json:"preferred_fraction"`
	// ClarificationAuto turns on the clarification loop's unprompted
	// half (FR-HF-22, FR-HF-24): a run that leaves a requirement
	// unresolved on a lexical basis or an unproven relationship asks the
	// candidate about it without waiting to be asked, and carries no fit
	// until the questions are closed. Off, a run is given its fit as it
	// always was and questions are asked only when the candidate asks
	// for them. It ships off, so the first review that waits on the
	// candidate is one the owner chose to start.
	ClarificationAuto bool `json:"clarification_auto"`
	// ClarificationDays is how long a held review waits for the
	// candidate before the open questions are closed unanswered and the
	// fit is issued from what is evidenced (owner's decision,
	// 2026-11-15: three days). 1 to 14.
	ClarificationDays int `json:"clarification_days"`
}

// DefaultClarificationDays is the time limit unless the setting says
// otherwise.
const DefaultClarificationDays = 3

// DefaultSettings is what a fresh install runs with.
func DefaultSettings() Settings {
	return Settings{PreferredFraction: DefaultPreferredFraction, ClarificationDays: DefaultClarificationDays}
}

// ErrInvalidSettings is returned for values outside their range.
var ErrInvalidSettings = errors.New("preferred fraction must be greater than 0 and at most 1")

// ErrInvalidClarificationDays is returned for a time limit outside its
// range.
var ErrInvalidClarificationDays = errors.New("the time limit for the candidate's answers must be between 1 and 14 days")

// Validate checks the ranges.
func (s Settings) Validate() error {
	if !(s.PreferredFraction > 0 && s.PreferredFraction <= 1) {
		return ErrInvalidSettings
	}
	if s.ClarificationDays < 1 || s.ClarificationDays > 14 {
		return ErrInvalidClarificationDays
	}
	return nil
}

// ClarificationLimit is how long a held review waits.
func (s Settings) ClarificationLimit() time.Duration {
	days := s.ClarificationDays
	if days < 1 {
		days = DefaultClarificationDays
	}
	return time.Duration(days) * 24 * time.Hour
}

// SettingKey is the app_settings key.
const SettingKey = "hiring_settings"

// SettingsBackend is the part of the settings table the store needs;
// users.Repo satisfies it.
type SettingsBackend interface {
	GetSetting(ctx context.Context, key string) (json.RawMessage, bool, error)
	SetSetting(ctx context.Context, key string, value json.RawMessage, updatedBy int64) error
}

// SettingsStore serves the current settings.
type SettingsStore struct {
	log      *slog.Logger
	backend  SettingsBackend
	mu       sync.Mutex
	cached   Settings
	loadedAt time.Time
}

const settingsCacheTTL = 15 * time.Second

// NewSettingsStore wires the store.
func NewSettingsStore(log *slog.Logger, backend SettingsBackend) *SettingsStore {
	return &SettingsStore{log: log, backend: backend}
}

// Get returns the settings in force; the defaults when none are stored
// or the stored value cannot be used.
func (s *SettingsStore) Get(ctx context.Context) Settings {
	if s == nil || s.backend == nil {
		return DefaultSettings()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loadedAt.IsZero() && time.Since(s.loadedAt) < settingsCacheTTL {
		return s.cached
	}
	v, err := s.load(ctx)
	if err != nil {
		if s.log != nil {
			s.log.Warn("hiring settings: using defaults", slog.String("error", err.Error()))
		}
		v = DefaultSettings()
	}
	s.cached, s.loadedAt = v, time.Now()
	return v
}

func (s *SettingsStore) load(ctx context.Context) (Settings, error) {
	raw, ok, err := s.backend.GetSetting(ctx, SettingKey)
	if err != nil {
		return Settings{}, err
	}
	if !ok {
		return DefaultSettings(), nil
	}
	var v Settings
	if err := json.Unmarshal(raw, &v); err != nil {
		return Settings{}, fmt.Errorf("decode hiring settings: %w", err)
	}
	// A row stored before the time limit existed has none: it runs with
	// the default, not with zero days.
	if v.ClarificationDays == 0 {
		v.ClarificationDays = DefaultClarificationDays
	}
	if err := v.Validate(); err != nil {
		return Settings{}, err
	}
	return v, nil
}

// Set validates, stores and applies new settings at once.
func (s *SettingsStore) Set(ctx context.Context, v Settings, updatedBy int64) error {
	if s == nil || s.backend == nil {
		return errors.New("hiring settings: no store")
	}
	if err := v.Validate(); err != nil {
		return err
	}
	raw, _ := json.Marshal(v)
	if err := s.backend.SetSetting(ctx, SettingKey, raw, updatedBy); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached, s.loadedAt = v, time.Now()
	s.mu.Unlock()
	return nil
}
