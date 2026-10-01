package users

import (
	"context"
	"testing"
)

// Every query in the dropdown must run against the real schema.
//
// They are fixed SQL over tables that migrations move, so a rename
// turns one into a runtime error that nobody sees until an admin picks
// it. Cheap to catch here, against a database with the full migration
// history.
func TestEveryAdminQueryRuns(t *testing.T) {
	r, ctx := chatRepo(t)
	list := ListAdminQueries()
	if len(list) == 0 {
		t.Fatal("no admin queries registered")
	}
	for _, q := range list {
		t.Run(q.ID, func(t *testing.T) {
			got, value, err := r.RunAdminQuery(ctx, q.ID)
			if err != nil {
				t.Fatalf("%s: %v", q.ID, err)
			}
			if got.Label == "" || q.Detail == "" {
				t.Errorf("%s has no label or no detail; the dropdown needs both", q.ID)
			}
			if value == "" {
				t.Errorf("%s returned nothing", q.ID)
			}
		})
	}
}

// An id outside the list is refused, not interpolated.
func TestAnUnknownAdminQueryIsRefused(t *testing.T) {
	r, ctx := chatRepo(t)
	for _, bad := range []string{"", "members; DROP TABLE users", "../../etc", "unknown"} {
		if _, _, err := r.RunAdminQuery(ctx, bad); err == nil {
			t.Errorf("RunAdminQuery(%q) was accepted", bad)
		}
	}
}

// The list handed to a caller must not carry the SQL.
func TestTheListDoesNotLeakSQL(t *testing.T) {
	for _, q := range ListAdminQueries() {
		if q.sql != "" {
			t.Errorf("%s carries its SQL out of the package", q.ID)
		}
	}
}

var _ = context.Background
