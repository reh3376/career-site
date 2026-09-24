package jd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// SettingJdSubmissionLimit is the app_settings key.
const SettingJdSubmissionLimit = "jd_submission_limit"

// MaxSubmissionLimit is the largest value the admin surface accepts.
// Not a capacity claim, just a guard against a typo turning the cap off
// by making it enormous.
const MaxSubmissionLimit = 100

// submissionLimit is the stored shape. A struct rather than a bare int
// so a window length or a per-role limit can be added later without a
// migration or a second key.
type submissionLimit struct {
	Limit int `json:"limit"`
}

// LimitStore serves how many postings one member may submit per rolling
// window, with a short cache so an admin edit takes effect within
// seconds and without a restart.
//
// It exists because the number is an operational judgment, not a
// constant. What it should be depends on how long a review takes on the
// box of the day, and that changes when the model changes, when the
// corpus grows, or when the box does. A value that lives only in an
// environment variable has to be edited over ssh and takes a deploy to
// apply, which is how a limit ends up wrong for months.
type LimitStore struct {
	log      *slog.Logger
	users    *users.Repo
	fallback int
	mu       sync.Mutex
	cached   int
	loadedAt time.Time
}

const limitCacheTTL = 15 * time.Second

// NewLimitStore wires the store. The fallback comes from configuration
// and is used until a setting exists, which it then seeds, so the value
// in force after the first read is visible in the database rather than
// only in the environment.
func NewLimitStore(log *slog.Logger, repo *users.Repo, fallback int) *LimitStore {
	if fallback < 0 {
		fallback = 0
	}
	return &LimitStore{log: log, users: repo, fallback: fallback}
}

// Get returns the limit in force. Zero means no cap.
func (s *LimitStore) Get(ctx context.Context) int {
	if s == nil {
		return 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.loadedAt.After(time.Time{}) && time.Since(s.loadedAt) < limitCacheTTL {
		return s.cached
	}
	n, err := s.load(ctx)
	if err != nil {
		// Fall back to the configured value rather than to "no limit". A
		// cap that disappears when the settings table is unreadable is
		// not a cap.
		s.log.Warn("jd submission limit: using fallback",
			slog.Int("fallback", s.fallback), slog.String("error", err.Error()))
		n = s.fallback
	}
	s.cached, s.loadedAt = n, time.Now()
	return n
}

func (s *LimitStore) load(ctx context.Context) (int, error) {
	raw, ok, err := s.users.GetSetting(ctx, SettingJdSubmissionLimit)
	if err != nil {
		return 0, err
	}
	if !ok {
		seed, _ := json.Marshal(submissionLimit{Limit: s.fallback})
		if err := s.users.SetSetting(ctx, SettingJdSubmissionLimit, seed, 0); err != nil {
			return 0, err
		}
		return s.fallback, nil
	}
	var v submissionLimit
	if err := json.Unmarshal(raw, &v); err != nil {
		return 0, fmt.Errorf("decode jd submission limit: %w", err)
	}
	if err := validateLimit(v.Limit); err != nil {
		return 0, err
	}
	return v.Limit, nil
}

// Set validates, stores and applies a new limit at once.
func (s *LimitStore) Set(ctx context.Context, limit int, updatedBy int64) error {
	if err := validateLimit(limit); err != nil {
		return err
	}
	raw, _ := json.Marshal(submissionLimit{Limit: limit})
	if err := s.users.SetSetting(ctx, SettingJdSubmissionLimit, raw, updatedBy); err != nil {
		return err
	}
	s.mu.Lock()
	s.cached, s.loadedAt = limit, time.Now()
	s.mu.Unlock()
	return nil
}

func validateLimit(n int) error {
	if n < 0 {
		return fmt.Errorf("the limit cannot be negative")
	}
	if n > MaxSubmissionLimit {
		return fmt.Errorf("the limit cannot exceed %d", MaxSubmissionLimit)
	}
	return nil
}
