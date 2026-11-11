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
}

// DefaultSettings is what a fresh install runs with.
func DefaultSettings() Settings {
	return Settings{PreferredFraction: DefaultPreferredFraction}
}

// ErrInvalidSettings is returned for values outside their range.
var ErrInvalidSettings = errors.New("preferred fraction must be greater than 0 and at most 1")

// Validate checks the ranges.
func (s Settings) Validate() error {
	if !(s.PreferredFraction > 0 && s.PreferredFraction <= 1) {
		return ErrInvalidSettings
	}
	return nil
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
