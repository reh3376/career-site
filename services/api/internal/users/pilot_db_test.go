package users

import (
	"testing"
)

// Every write the pilot performs, against a real database. The reason
// is the one decisiontest_db_test.go gives: a column count or a type
// inference in SQL is invisible to go build, go vet and gofmt, and the
// only thing that catches it is running the statement.
//
// Skipped without TEST_DATABASE_URL, fails in CI without it, exactly as
// dtTestRepo does.
func TestPilotWritePathAgainstPostgres(t *testing.T) {
	r, ctx := dtTestRepo(t)

	// A participant to own the session. Cleanup takes the session,
	// answers, events and comments with it through the cascades.
	var userID int64
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO users (email, name, status, role)
    VALUES ('pilot-' || gen_random_uuid()::text || '@example.test', 'Pilot Tester', 'active', 'member')
    RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("make user: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

	const version = "test-v0.0"
	if err := r.EnsurePilotInstrument(ctx, version, []byte(`{"version":"test-v0.0","sections":[]}`)); err != nil {
		t.Fatalf("EnsurePilotInstrument: %v", err)
	}
	// A second call with a different body must not replace the stored
	// copy: what a past session answered is what stays.
	if err := r.EnsurePilotInstrument(ctx, version, []byte(`{"version":"test-v0.0","changed":true}`)); err != nil {
		t.Fatalf("EnsurePilotInstrument again: %v", err)
	}
	var stored string
	if err := r.pool.QueryRow(ctx, `SELECT definition::text FROM pilot_instruments WHERE version = $1`, version).Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if stored == `{"version": "test-v0.0", "changed": true}` {
		t.Fatal("a second EnsurePilotInstrument replaced the fielded definition")
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM pilot_instruments WHERE version = $1`, version) })

	if _, err := r.GetPilotSession(ctx, userID, version); err != ErrNotFound {
		t.Fatalf("GetPilotSession before start: err = %v, want ErrNotFound", err)
	}
	s1, err := r.StartPilotSession(ctx, userID, version, true)
	if err != nil {
		t.Fatalf("StartPilotSession: %v", err)
	}
	s2, err := r.StartPilotSession(ctx, userID, version, false)
	if err != nil {
		t.Fatalf("StartPilotSession again: %v", err)
	}
	if s1.ID != s2.ID || !s2.RecordingOK {
		t.Fatalf("a second start must resume the same session with its first preference: %+v vs %+v", s1, s2)
	}
	if s1.Status != "consented" || s1.RoleBranch != "" {
		t.Fatalf("fresh session: %+v", s1)
	}

	// Section A: open, answer, submit, route.
	if err := r.RecordPilotSectionEvent(ctx, s1.ID, "A", "opened"); err != nil {
		t.Fatal(err)
	}
	answers := []PilotAnswer{
		{ItemID: "A1", ValueJSON: `"Hiring manager (I own the role and decide)"`},
		{ItemID: "A2", ValueJSON: `"3 to 5"`, Flagged: true},
	}
	if err := r.UpsertPilotAnswers(ctx, s1.ID, "A", answers); err != nil {
		t.Fatalf("UpsertPilotAnswers: %v", err)
	}
	// Resubmitting replaces the value and clears the flag.
	if err := r.UpsertPilotAnswers(ctx, s1.ID, "A", []PilotAnswer{{ItemID: "A2", ValueJSON: `"6 to 15"`}}); err != nil {
		t.Fatalf("UpsertPilotAnswers again: %v", err)
	}
	got, err := r.ListPilotAnswers(ctx, s1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("answers = %d, want 2", len(got))
	}
	for _, a := range got {
		if a.ItemID == "A2" && (a.ValueJSON != `"6 to 15"` || a.Flagged) {
			t.Fatalf("A2 was not replaced: %+v", a)
		}
	}
	if err := r.SetPilotRoleBranch(ctx, s1.ID, "hm"); err != nil {
		t.Fatal(err)
	}
	if err := r.RecordPilotSectionEvent(ctx, s1.ID, "A", "submitted"); err != nil {
		t.Fatal(err)
	}
	done, err := r.SubmittedPilotSections(ctx, s1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !done["A"] || done["B"] {
		t.Fatalf("submitted sections = %v", done)
	}
	events, err := r.ListPilotSectionEvents(ctx, s1.ID)
	if err != nil || len(events) != 2 {
		t.Fatalf("events = %v, %v", events, err)
	}

	// The pilot-only closers ride on the session.
	minutes := int32(11)
	if err := r.MarkPilotSurveyFinished(ctx, s1.ID, &minutes, "C3"); err != nil {
		t.Fatal(err)
	}
	// A second mark with nothing new keeps the first instant and values.
	if err := r.MarkPilotSurveyFinished(ctx, s1.ID, nil, ""); err != nil {
		t.Fatal(err)
	}
	s3, err := r.GetPilotSessionByID(ctx, s1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if s3.RoleBranch != "hm" || s3.SurveyFinishedAt == nil || s3.SelfReportedMinutes == nil || *s3.SelfReportedMinutes != 11 || s3.MostWantedToSkip != "C3" {
		t.Fatalf("after survey: %+v", s3)
	}

	// Comments: upsert, replace, remove.
	c, err := r.UpsertPilotComment(ctx, s1.ID, "record_row", "r6", "would check this one first")
	if err != nil || c == nil || c.Body == "" {
		t.Fatalf("UpsertPilotComment: %v %v", c, err)
	}
	if _, err := r.UpsertPilotComment(ctx, s1.ID, "record_row", "r6", "changed my mind"); err != nil {
		t.Fatal(err)
	}
	if _, err := r.UpsertPilotComment(ctx, s1.ID, "workflow_node", "C-12", "we have no hold state here"); err != nil {
		t.Fatal(err)
	}
	removed, err := r.UpsertPilotComment(ctx, s1.ID, "workflow_node", "C-12", "")
	if err != nil || removed != nil {
		t.Fatalf("remove: %v %v", removed, err)
	}
	comments, err := r.ListPilotComments(ctx, s1.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(comments) != 1 || comments[0].Body != "changed my mind" {
		t.Fatalf("comments = %+v", comments)
	}

	if err := r.MarkPilotPart(ctx, s1.ID, "record"); err != nil {
		t.Fatal(err)
	}
	if err := r.MarkPilotPart(ctx, s1.ID, "nowhere"); err == nil {
		t.Fatal("MarkPilotPart accepted an unknown part")
	}
	if err := r.FinishPilotSession(ctx, s1.ID); err != nil {
		t.Fatal(err)
	}

	list, err := r.ListPilotSessionSummaries(ctx)
	if err != nil {
		t.Fatalf("ListPilotSessionSummaries: %v", err)
	}
	var mine *PilotSessionSummary
	for i := range list {
		if list[i].Session.ID == s1.ID {
			mine = &list[i]
		}
	}
	if mine == nil {
		t.Fatal("the session is missing from the admin list")
	}
	if mine.Session.Status != "finished" || mine.Session.RecordFinishedAt == nil || mine.CommentCount != 1 || mine.FlagCount != 0 || mine.Name != "Pilot Tester" || mine.FirstOpened == nil || mine.LastSubmit == nil {
		t.Fatalf("summary: %+v", mine)
	}
}

// The owner's own runs on the deployed build are removed with
// DeletePilotSession (FSD FR-PLT-15). The cascades in 00065 are what
// make that one statement rather than four, and a cascade declared in
// SQL is as invisible to go build as a column count is, so it is run.
func TestDeletePilotSessionCascades(t *testing.T) {
	r, ctx := dtTestRepo(t)

	var userID int64
	if err := r.pool.QueryRow(ctx, `
    INSERT INTO users (email, name, status, role)
    VALUES ('pilot-del-' || gen_random_uuid()::text || '@example.test', 'Pilot Deleter', 'active', 'member')
    RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("make user: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) })

	const version = "test-del-v0.0"
	if err := r.EnsurePilotInstrument(ctx, version, []byte(`{"version":"test-del-v0.0","sections":[]}`)); err != nil {
		t.Fatalf("EnsurePilotInstrument: %v", err)
	}
	t.Cleanup(func() { _, _ = r.pool.Exec(ctx, `DELETE FROM pilot_instruments WHERE version = $1`, version) })

	s, err := r.StartPilotSession(ctx, userID, version, false)
	if err != nil {
		t.Fatalf("StartPilotSession: %v", err)
	}
	if err := r.UpsertPilotAnswers(ctx, s.ID, "A", []PilotAnswer{{ItemID: "A1", ValueJSON: `"recruiter"`}}); err != nil {
		t.Fatalf("UpsertPilotAnswers: %v", err)
	}
	if err := r.RecordPilotSectionEvent(ctx, s.ID, "A", "opened"); err != nil {
		t.Fatalf("RecordPilotSectionEvent: %v", err)
	}
	if _, err := r.UpsertPilotComment(ctx, s.ID, "record_row", "r1", "a comment"); err != nil {
		t.Fatalf("UpsertPilotComment: %v", err)
	}

	deleted, err := r.DeletePilotSession(ctx, s.ID)
	if err != nil || !deleted {
		t.Fatalf("DeletePilotSession: deleted=%v err=%v", deleted, err)
	}
	for _, table := range []string{"pilot_sessions", "pilot_answers", "pilot_section_events", "pilot_comments"} {
		var n int
		col := "session_id"
		if table == "pilot_sessions" {
			col = "id"
		}
		if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM `+table+` WHERE `+col+` = $1::uuid`, s.ID).Scan(&n); err != nil {
			t.Fatalf("count %s: %v", table, err)
		}
		if n != 0 {
			t.Errorf("%s still holds %d row(s) for the deleted session; the cascade did not run", table, n)
		}
	}
	// A second delete finds nothing and says so, which is what lets the
	// handler answer "no such session" rather than "deleted".
	if again, err := r.DeletePilotSession(ctx, s.ID); err != nil || again {
		t.Fatalf("second DeletePilotSession: deleted=%v err=%v, want false and nil", again, err)
	}
}
