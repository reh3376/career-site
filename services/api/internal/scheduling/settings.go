package scheduling

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// SettingKey is the app_settings key holding the owner's availability.
//
// Stored rather than compiled in because every value here is a
// decision the owner will change: the hours he is willing to be
// interrupted, how far ahead someone may book, how many meetings he
// will take in a day. A redeploy is the wrong way to say "not next
// Tuesday".
const SettingKey = "scheduler_settings"

// DefaultSettings is the owner's stated configuration, used until a
// setting exists and as the fallback when one cannot be read.
//
// His words, 2026-09-29: Tuesday, Wednesday and Thursday, 09:00 to
// 12:00 and 14:00 to 16:00, meetings of 15, 30 or 45 minutes with 15
// minutes of clearance between them, in America/New_York.
func DefaultSettings() Settings {
	return Settings{
		Zone:        "America/New_York",
		Durations:   []int{15, 30, 45},
		GapMins:     15,
		StepMins:    15,
		MaxPerDay:   3,
		LeadHours:   24,
		HorizonDays: 21,
		Windows: []Window{
			{Weekday: time.Tuesday, StartMins: 9 * 60, EndMins: 12 * 60},
			{Weekday: time.Tuesday, StartMins: 14 * 60, EndMins: 16 * 60},
			{Weekday: time.Wednesday, StartMins: 9 * 60, EndMins: 12 * 60},
			{Weekday: time.Wednesday, StartMins: 14 * 60, EndMins: 16 * 60},
			{Weekday: time.Thursday, StartMins: 9 * 60, EndMins: 12 * 60},
			{Weekday: time.Thursday, StartMins: 14 * 60, EndMins: 16 * 60},
		},
	}
}

// SettingsStore serves the current settings with a short cache, so the
// availability RPC, the booking re-check and the admin surface all read
// the same configuration and an edit takes effect within seconds
// without a restart. Same shape as jd.BandsStore, for the same reason.
//
// Named for what it holds because Store in this package is already the
// booking repository.
type SettingsStore struct {
	log      *slog.Logger
	users    *users.Repo
	fallback Settings
	mu       sync.Mutex
	cached   Settings
	loadedAt time.Time
}

const settingsCacheTTL = 15 * time.Second

// NewSettingsStore wires the store. The fallback is used until a
// setting exists, and seeds it the first time it is read.
func NewSettingsStore(log *slog.Logger, repo *users.Repo, fallback Settings) *SettingsStore {
	if err := fallback.Validate(); err != nil {
		// A fallback that cannot produce slots fails closed and
		// silently: the calendar would simply be empty forever, which
		// reads to a member as "never available" rather than as a bug.
		log.Warn("scheduler: the fallback settings are invalid, using the defaults",
			slog.String("error", err.Error()))
		fallback = DefaultSettings()
	}
	return &SettingsStore{log: log, users: repo, fallback: fallback}
}

// Get returns the current settings, never a zero value.
func (s *SettingsStore) Get(ctx context.Context) Settings {
	if s == nil {
		return DefaultSettings()
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.loadedAt.IsZero() && time.Since(s.loadedAt) < settingsCacheTTL {
		return s.cached
	}
	cfg, err := s.load(ctx)
	if err != nil {
		s.log.Warn("scheduler: using fallback settings", slog.String("error", err.Error()))
		cfg = s.fallback
	}
	s.cached, s.loadedAt = cfg, time.Now()
	return cfg
}

func (s *SettingsStore) load(ctx context.Context) (Settings, error) {
	raw, ok, err := s.users.GetSetting(ctx, SettingKey)
	if err != nil {
		return Settings{}, err
	}
	if !ok {
		seed, mErr := json.Marshal(s.fallback)
		if mErr != nil {
			return Settings{}, fmt.Errorf("seed scheduler settings: %w", mErr)
		}
		if err := s.users.SetSetting(ctx, SettingKey, seed, 0); err != nil {
			return Settings{}, fmt.Errorf("seed scheduler settings: %w", err)
		}
		return s.fallback, nil
	}
	var cfg Settings
	if err := json.Unmarshal(raw, &cfg); err != nil {
		return Settings{}, fmt.Errorf("decode scheduler settings: %w", err)
	}
	// A stored value can be invalid: written by an older build, or
	// edited by hand. Refusing it is better than computing a calendar
	// from settings nothing ever checked.
	if err := cfg.Validate(); err != nil {
		return Settings{}, fmt.Errorf("stored scheduler settings are invalid: %w", err)
	}
	return cfg, nil
}

// Put validates and writes the settings, then refreshes the cache so
// the next read is the new value rather than up to a TTL of the old.
func (s *SettingsStore) Put(ctx context.Context, cfg Settings, updatedBy int64) error {
	if err := cfg.Validate(); err != nil {
		return err
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("encode scheduler settings: %w", err)
	}
	if err := s.users.SetSetting(ctx, SettingKey, body, updatedBy); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached, s.loadedAt = cfg, time.Now()
	s.mu.Unlock()
	return nil
}
