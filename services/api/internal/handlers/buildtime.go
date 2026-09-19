package handlers

import "time"

// parseBuildTime accepts RFC3339 or the sentinel "unknown"; returns nil for
// anything unparseable so callers can omit the field cleanly.
func parseBuildTime(s string) *time.Time {
	if s == "" || s == "unknown" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}
