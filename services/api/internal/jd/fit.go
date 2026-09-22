package jd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// Fit categories tell the submitter, and the owner, what a score means
// in plain words. Owner's wording (2026-09-22): very strong, strong,
// possible, weak, very weak. The band edges are an owner-editable
// setting (/admin/jd), stored in app_settings under SettingFitBands;
// the "strong" floor is the résumé gate itself, so a tailored résumé
// exists exactly when a review is strong or better.
const (
	FitVeryStrong = "very_strong"
	FitStrong     = "strong"
	FitPossible   = "possible"
	FitWeak       = "weak"
	FitVeryWeak   = "very_weak"
)

// SettingFitBands is the app_settings key.
const SettingFitBands = "jd_fit_bands"

// Bands are the lower edges of each category. Scores below Weak are
// very weak.
type Bands struct {
	VeryStrong float64 `json:"very_strong"`
	Strong     float64 `json:"strong"` // the résumé gate
	Possible   float64 `json:"possible"`
	Weak       float64 `json:"weak"`
}

// DefaultBands seeds the setting from the configured gate.
func DefaultBands(gate float64) Bands {
	if gate <= 0 || gate > 1 {
		gate = DefaultMatchThreshold
	}
	b := Bands{VeryStrong: 0.85, Strong: gate, Possible: 0.55, Weak: 0.35}
	if b.VeryStrong <= gate {
		b.VeryStrong = gate + 0.10
	}
	if b.Possible >= gate {
		b.Possible = gate - 0.15
	}
	if b.Weak >= b.Possible {
		b.Weak = b.Possible - 0.20
	}
	return b
}

// ErrInvalidBands is returned for bands out of order or range.
var ErrInvalidBands = errors.New("bands must satisfy 0 < weak < possible < strong < very strong <= 1")

// Validate checks 0 < weak < possible < strong < very_strong <= 1.
func (b Bands) Validate() error {
	if !(b.Weak > 0 && b.Weak < b.Possible && b.Possible < b.Strong && b.Strong < b.VeryStrong && b.VeryStrong <= 1) {
		return ErrInvalidBands
	}
	return nil
}

// Category maps a score to its category.
func (b Bands) Category(score float64) string {
	switch {
	case score >= b.VeryStrong:
		return FitVeryStrong
	case score >= b.Strong:
		return FitStrong
	case score >= b.Possible:
		return FitPossible
	case score >= b.Weak:
		return FitWeak
	default:
		return FitVeryWeak
	}
}

// FitLabel is the human wording for a category.
func FitLabel(cat string) string {
	switch cat {
	case FitVeryStrong:
		return "very strong"
	case FitStrong:
		return "strong"
	case FitPossible:
		return "possible"
	case FitWeak:
		return "weak"
	case FitVeryWeak:
		return "very weak"
	}
	return ""
}

// BandsStore serves the current bands with a short cache, so the
// pipeline, the result RPCs and the emails all read the same numbers
// and an admin edit takes effect within seconds without a restart.
type BandsStore struct {
	log      *slog.Logger
	users    *users.Repo
	fallback Bands
	mu       sync.Mutex
	cached   Bands
	loadedAt time.Time
}

const bandsCacheTTL = 15 * time.Second

// NewBandsStore wires the store; fallback is used until a setting
// exists (and seeds it the first time it is read).
func NewBandsStore(log *slog.Logger, repo *users.Repo, fallback Bands) *BandsStore {
	return &BandsStore{log: log, users: repo, fallback: fallback}
}

// Get returns the current bands.
func (s *BandsStore) Get(ctx context.Context) Bands {
	if s == nil {
		return DefaultBands(0)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if time.Since(s.loadedAt) < bandsCacheTTL && s.loadedAt.After(time.Time{}) {
		return s.cached
	}
	b, err := s.load(ctx)
	if err != nil {
		s.log.Warn("fit bands: using fallback", slog.String("error", err.Error()))
		b = s.fallback
	}
	s.cached, s.loadedAt = b, time.Now()
	return b
}

func (s *BandsStore) load(ctx context.Context) (Bands, error) {
	raw, ok, err := s.users.GetSetting(ctx, SettingFitBands)
	if err != nil {
		return Bands{}, err
	}
	if !ok {
		seed, _ := json.Marshal(s.fallback)
		if err := s.users.SetSetting(ctx, SettingFitBands, seed, 0); err != nil {
			return Bands{}, err
		}
		return s.fallback, nil
	}
	var b Bands
	if err := json.Unmarshal(raw, &b); err != nil {
		return Bands{}, fmt.Errorf("decode fit bands: %w", err)
	}
	if err := b.Validate(); err != nil {
		return Bands{}, err
	}
	return b, nil
}

// Set validates, stores and applies new bands at once.
func (s *BandsStore) Set(ctx context.Context, b Bands, updatedBy int64) error {
	if err := b.Validate(); err != nil {
		return err
	}
	raw, _ := json.Marshal(b)
	if err := s.users.SetSetting(ctx, SettingFitBands, raw, updatedBy); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached, s.loadedAt = b, time.Now()
	s.mu.Unlock()
	return nil
}
