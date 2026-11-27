package users

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/reh3376/career-site/services/api/internal/dtreport"
)

// The report's SQL, executed against Postgres for the same reason the
// rest of the decision test's is (see decisiontest_db_test.go): a query
// inside a string literal is checked by nothing until it runs.

const dtReportTestVersion = "v4-m6000-q25000-r20000"

// dtReportRun takes a whole sitting through the write path and closes
// it. correct is how many of the thirty are answered correctly.
func dtReportRun(t *testing.T, r *Repo, ctx context.Context, visitor, itemSet, version string, synthetic bool, correct int, wants bool) *DTSession {
	t.Helper()
	intake := DTIntake{}
	if wants {
		intake = DTIntake{DisplayName: "report test", Email: visitor + "@example.test", WantsResults: true}
	}
	sess, err := r.StartDecisionTest(ctx, intake,
		DTConditions{AudioMode: "sound", DeviceClass: "phone", TapCheckPassed: true, BaselineRTMs: 300, BaselineRTSDMs: 40, TimerVisible: true},
		synthetic, visitor, nil, version, itemSet, "key-1", true)
	if err != nil {
		t.Fatalf("StartDecisionTest: %v", err)
	}
	items, err := r.DTScoredItems(ctx)
	if err != nil {
		t.Fatalf("DTScoredItems: %v", err)
	}
	if len(items) < DTQuestionCount {
		t.Fatalf("item bank holds %d scored items, need %d", len(items), DTQuestionCount)
	}
	for bn := 1; bn <= DTBlockCount; bn++ {
		block, err := r.DTBuildBlock(ctx, bn, items[(bn-1)*DTQuestionsPerBlock:bn*DTQuestionsPerBlock])
		if err != nil {
			t.Fatalf("DTBuildBlock(%d): %v", bn, err)
		}
		if err := r.DTRecordBlockDigits(ctx, sess.ID, block); err != nil {
			t.Fatalf("DTRecordBlockDigits(%d): %v", bn, err)
		}
		for i := range DTQuestionsPerBlock {
			pos := (bn-1)*DTQuestionsPerBlock + i + 1
			item := items[pos-1]
			chosen := item.LureIndex
			if pos <= correct {
				chosen = item.CorrectIndex
			}
			if err := r.DTSaveAnswer(ctx, sess.ID, bn, pos, chosen, 9000+pos*100, 70, item); err != nil {
				t.Fatalf("DTSaveAnswer(%d): %v", pos, err)
			}
		}
		if _, _, err := r.DTSaveRecall(ctx, sess.ID, bn, block.Digits, 8000); err != nil {
			t.Fatalf("DTSaveRecall(%d): %v", bn, err)
		}
	}
	if _, _, err := r.DTFinish(ctx, sess.ID, "encode"); err != nil {
		t.Fatalf("DTFinish: %v", err)
	}
	return sess
}

// dtReportCleanup removes a test's runs. Real runs are never deleted
// (FR-DT-11); these are not real, and several of them are deliberately
// not flagged synthetic, so they are taken out again rather than left
// in a shared database for another package's test to count.
func dtReportCleanup(t *testing.T, r *Repo, itemSets ...string) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		for _, set := range itemSets {
			// The draw has no cascade, by design: a real run's questions
			// outlive everything. So it goes first.
			if _, err := r.pool.Exec(ctx, `
				DELETE FROM dt_session_items i USING dt_sessions s
				 WHERE i.session_id = s.id AND s.item_set_version = $1`, set); err != nil {
				t.Logf("cleanup %s draw: %v", set, err)
			}
			if _, err := r.pool.Exec(ctx, `
				WITH gone AS (DELETE FROM dt_sessions WHERE item_set_version = $1 RETURNING participant_id)
				DELETE FROM dt_participants p USING gone g
				 WHERE p.id = g.participant_id
				   AND NOT EXISTS (SELECT 1 FROM dt_sessions s WHERE s.participant_id = p.id
				                      AND s.item_set_version <> $1)`, set); err != nil {
				t.Logf("cleanup %s: %v", set, err)
			}
		}
	})
}

func TestTheReportReadsItsGroupAgainstPostgres(t *testing.T) {
	r, ctx := dtTestRepo(t)
	tag := fmt.Sprintf("%d", time.Now().UnixNano())
	set, otherSet := "report-test-"+tag, "report-test-other-"+tag
	dtReportCleanup(t, r, set, otherSet)

	subject := dtReportRun(t, r, ctx, "subject-"+tag, set, dtReportTestVersion, false, 12, true)
	first := dtReportRun(t, r, ctx, "first-"+tag, set, dtReportTestVersion, false, 10, false)
	dtReportRun(t, r, ctx, "second-"+tag, set, "v3-m6000-q25000-r20000", false, 20, false)
	// The same visitor again: a repeat sitting.
	dtReportRun(t, r, ctx, "first-"+tag, set, dtReportTestVersion, false, 25, false)
	aside := dtReportRun(t, r, ctx, "aside-"+tag, set, dtReportTestVersion, false, 5, false)
	if err := r.DTReviewSession(ctx, aside.PublicID, DTReview{Status: DTExcludingStatus}, nil); err != nil {
		t.Fatalf("DTReviewSession: %v", err)
	}
	// None of these belongs to the group: an agent's run, another pool,
	// other time limits, and a run that never finished.
	dtReportRun(t, r, ctx, "agent-"+tag, set, dtReportTestVersion, true, 30, false)
	dtReportRun(t, r, ctx, "pool-"+tag, otherSet, dtReportTestVersion, false, 30, false)
	dtReportRun(t, r, ctx, "timing-"+tag, set, "v4-m6000-q20000-r20000", false, 30, false)
	if _, err := r.StartDecisionTest(ctx, DTIntake{}, DTConditions{AudioMode: "sound", DeviceClass: "phone"},
		false, "running-"+tag, nil, dtReportTestVersion, set, "key-1", true); err != nil {
		t.Fatalf("StartDecisionTest: %v", err)
	}

	in, err := r.DTReportInput(ctx, subject.ID)
	if err != nil {
		t.Fatalf("DTReportInput: %v", err)
	}
	run := in.Run
	if run.Key != subject.PublicID || run.Device != "phone" || run.RecallStrategy != "encode" || !run.TimerVisible {
		t.Fatalf("run: %+v", run)
	}
	if run.AttemptNo == nil || *run.AttemptNo != 1 || run.Synthetic {
		t.Fatalf("a first, real sitting: attempt=%v synthetic=%v", run.AttemptNo, run.Synthetic)
	}
	if len(run.Answers) != DTQuestionCount || len(run.Recalls) != DTBlockCount {
		t.Fatalf("%d answers, %d recalls", len(run.Answers), len(run.Recalls))
	}
	correct := 0
	for i, a := range run.Answers {
		if a.Position != i+1 || a.Load == "" || a.Family == "" || a.ItemCode == "" {
			t.Fatalf("answer %d: %+v", i+1, a)
		}
		// The owner's copy needs the question and both answers; the
		// guessing figure needs to know how many were on offer.
		if a.Prompt == "" || a.ChosenText == "" || a.CorrectText == "" || a.Options < 2 {
			t.Fatalf("answer %d is missing its text or its options: %+v", i+1, a)
		}
		if a.Outcome == "correct" {
			correct++
			if a.ChosenText != a.CorrectText {
				t.Fatalf("answer %d is correct but the texts differ", i+1)
			}
		}
		if a.Confidence != 70 || a.LatencyMs == 0 {
			t.Fatalf("answer %d: confidence %d latency %d", i+1, a.Confidence, a.LatencyMs)
		}
	}
	if correct != 12 {
		t.Fatalf("%d correct, want 12", correct)
	}
	for _, rc := range run.Recalls {
		// The helper gives back the number as shown, which is exact in
		// the plain blocks and the rule not applied in the two that
		// transform it.
		want := "exact"
		if rc.Presented != rc.Expected {
			want = "untransformed"
		}
		if rc.Presented == "" || rc.Expected == "" || rc.Outcome != want || rc.Load == "" {
			t.Fatalf("recall: %+v, want %s", rc, want)
		}
	}
	if in.ConfidentThreshold != 70 || in.MinPeers != DTReportMinPeers || in.Email.Reading == "" {
		t.Fatalf("threshold %d, min peers %d, reading %q", in.ConfidentThreshold, in.MinPeers, in.Email.Reading)
	}

	// Four finished, real runs share the pool and the time limits. Which
	// of them is comparable is dtreport's to decide, so all four arrive,
	// each saying what it is.
	if len(in.Peers) != 4 {
		t.Fatalf("%d peers, want 4", len(in.Peers))
	}
	repeats, setAside, generations := 0, 0, map[int]int{}
	for _, p := range in.Peers {
		if p.Key == subject.PublicID {
			t.Fatal("the run is among its own peers")
		}
		if len(p.Answers) != DTQuestionCount || len(p.Recalls) != DTBlockCount {
			t.Fatalf("peer %s: %d answers, %d recalls", p.ShortID(), len(p.Answers), len(p.Recalls))
		}
		if p.AttemptNo == nil {
			t.Fatalf("peer %s has no sitting number", p.ShortID())
		}
		if *p.AttemptNo > 1 {
			repeats++
		}
		if !p.Usable {
			setAside++
			if p.Key != aside.PublicID {
				t.Fatalf("the wrong run was set aside: %s", p.ShortID())
			}
		}
		generations[p.Generation()]++
		// A peer's questions and answers are never read.
		if p.Answers[0].Prompt != "" || p.Answers[0].CorrectText != "" {
			t.Fatal("a peer arrived with question text")
		}
	}
	if repeats != 1 || setAside != 1 || generations[3] != 1 || generations[4] != 3 {
		t.Fatalf("repeats=%d set aside=%d generations=%v", repeats, setAside, generations)
	}

	// And the engine takes it from there: two comparable first sittings,
	// which is under the minimum, so the report carries no ranking.
	doc, f := dtreport.Build(*in)
	if f.Compared || f.Reason != dtreport.ReasonSmallGroup || f.Others != 2 || f.RepeatSittings != 1 {
		t.Fatalf("figures: %+v repeat=%d", f.Summary(), f.RepeatSittings)
	}
	if f.Score != 12 || len(doc.Blocks) == 0 {
		t.Fatalf("score %d", f.Score)
	}
	in.MinPeers = 2
	if _, f = dtreport.Build(*in); !f.Compared || f.N != 3 || f.ScorePlace.Rank != 2 {
		t.Fatalf("with a minimum of two the run ranks second of three: %+v", f.Summary())
	}

	// The same reading from the repeat's side: its own sitting number.
	_ = first
	var repeatID int64
	if err := r.pool.QueryRow(ctx, `
		SELECT s.id FROM dt_sessions s
		 WHERE s.item_set_version = $1
		   AND dt_attempt_no(s.prior_by_account, s.prior_by_email, s.prior_by_cookie) = 2`, set).Scan(&repeatID); err != nil {
		t.Fatalf("find the repeat: %v", err)
	}
	rin, err := r.DTReportInput(ctx, repeatID)
	if err != nil {
		t.Fatalf("DTReportInput(repeat): %v", err)
	}
	if _, f = dtreport.Build(*rin); f.Compared || f.Reason != dtreport.ReasonRepeat {
		t.Fatalf("a repeat sitting is not ranked: %+v", f.Summary())
	}
}

func TestAnUnfinishedRunHasNoReport(t *testing.T) {
	r, ctx := dtTestRepo(t)
	tag := fmt.Sprintf("%d", time.Now().UnixNano())
	set := "report-test-open-" + tag
	dtReportCleanup(t, r, set)

	sess, err := r.StartDecisionTest(ctx, DTIntake{}, DTConditions{AudioMode: "sound", DeviceClass: "phone"},
		true, "open-"+tag, nil, dtReportTestVersion, set, "key-1", true)
	if err != nil {
		t.Fatalf("StartDecisionTest: %v", err)
	}
	if _, err := r.DTReportInput(ctx, sess.ID); !errors.Is(err, ErrDTRunIncomplete) {
		t.Fatalf("a running session: %v", err)
	}
	// Closed with no answers, which a client can do: completed, and
	// still not a test anybody sat.
	if _, _, err := r.DTFinish(ctx, sess.ID, ""); err != nil {
		t.Fatalf("DTFinish: %v", err)
	}
	if _, err := r.DTReportInput(ctx, sess.ID); !errors.Is(err, ErrDTRunIncomplete) {
		t.Fatalf("a run closed without its answers: %v", err)
	}
}

func TestReportStorageAgainstPostgres(t *testing.T) {
	r, ctx := dtTestRepo(t)
	tag := fmt.Sprintf("%d", time.Now().UnixNano())
	set := "report-test-store-" + tag
	dtReportCleanup(t, r, set)
	sess := dtReportRun(t, r, ctx, "store-"+tag, set, dtReportTestVersion, true, 12, true)

	due := func(settle time.Duration) bool {
		t.Helper()
		ids, err := r.DTReportsDue(ctx, settle, 10000)
		if err != nil {
			t.Fatalf("DTReportsDue: %v", err)
		}
		for _, id := range ids {
			if id == sess.ID {
				return true
			}
		}
		return false
	}

	// A run that has only just closed is left alone, so the debrief
	// answer can arrive; once it has settled it is due.
	if due(time.Hour) {
		t.Fatal("a run that closed a moment ago is already due")
	}
	if !due(0) {
		t.Fatal("a finished run with no report is not due")
	}
	st, err := r.DTReportStateByID(ctx, sess.ID)
	if err != nil {
		t.Fatalf("DTReportStateByID: %v", err)
	}
	if st.Report != nil || st.Attempts != 0 || !st.CanSend || st.Status != "completed" || st.SessionKey != sess.PublicID {
		t.Fatalf("fresh state: %+v", st)
	}

	// Failures count, and a failed build waits before it is tried again.
	boom := errors.New("renderer unavailable")
	if err := r.DTReportFailed(ctx, sess.ID, boom, false); err != nil {
		t.Fatalf("DTReportFailed: %v", err)
	}
	if due(0) {
		t.Fatal("a build that failed a moment ago is retried at once")
	}
	if _, err := r.pool.Exec(ctx, `UPDATE dt_report_jobs SET last_attempt_at = now() - interval '10 minutes' WHERE session_id = $1`, sess.ID); err != nil {
		t.Fatal(err)
	}
	if !due(0) {
		t.Fatal("a failed build is not retried after its wait")
	}
	for range DTReportMaxAttempts - 1 {
		if err := r.DTReportFailed(ctx, sess.ID, boom, false); err != nil {
			t.Fatal(err)
		}
	}
	st, _ = r.DTReportStateByID(ctx, sess.ID)
	if !st.Exhausted() || st.Attempts != DTReportMaxAttempts || st.LastError != boom.Error() {
		t.Fatalf("after %d failures: %+v", DTReportMaxAttempts, st)
	}
	// Given up on, so nothing more to build; but the email it was
	// holding back is now owed, which keeps the run due until that is
	// settled.
	if !due(0) {
		t.Fatal("a run whose build was given up on still owes its email")
	}

	// The automatic email can be claimed once.
	if ok, err := r.DTResultEmailClaim(ctx, sess.ID); err != nil || !ok {
		t.Fatalf("first claim: %v %v", ok, err)
	}
	if ok, err := r.DTResultEmailClaim(ctx, sess.ID); err != nil || ok {
		t.Fatalf("second claim: %v %v", ok, err)
	}
	if due(0) {
		t.Fatal("a run with its build given up on and its email claimed is still due")
	}
	if err := r.DTResultEmailSettle(ctx, sess.ID, "sent", 0, nil); err != nil {
		t.Fatalf("DTResultEmailSettle: %v", err)
	}
	st, _ = r.DTReportStateByID(ctx, sess.ID)
	if st.EmailedAt == nil || st.EmailedBy != "system" || st.EmailedVersion != 0 {
		t.Fatalf("after the send: %+v", st)
	}

	// Builds are versions, kept.
	build := func(tag string) DTReportBuild {
		return DTReportBuild{Template: dtreport.TemplateVersion, Engine: "test",
			Summary:        dtreport.Summary{Template: dtreport.TemplateVersion, Compared: true, Others: 15, Score: 12, Total: 30, Rank: 5, GroupSize: 16, Percentile: 71.875},
			ParticipantPDF: []byte("%PDF-participant-" + tag), OwnerPDF: []byte("%PDF-owner-" + tag),
			ParticipantPages: 7, OwnerPages: 10, TriggeredBy: tag}
	}
	for want, who := range []string{"", "admin:7"} {
		v, err := r.DTReportSave(ctx, sess.ID, build(who))
		if err != nil || v != want+1 {
			t.Fatalf("DTReportSave: version %d, %v", v, err)
		}
	}
	st, err = r.DTReportStateByKey(ctx, sess.PublicID)
	if err != nil {
		t.Fatalf("DTReportStateByKey: %v", err)
	}
	rec := st.Report
	if rec == nil || rec.Version != 2 || rec.TriggeredBy != "admin:7" || !rec.Compared || rec.Others != 15 {
		t.Fatalf("latest: %+v", rec)
	}
	if rec.Summary.Rank != 5 || rec.Summary.GroupSize != 16 || rec.Summary.Percentile != 71.875 || rec.ParticipantPages != 7 || rec.OwnerPages != 10 {
		t.Fatalf("summary: %+v", rec.Summary)
	}
	if st.Exhausted() || due(0) {
		t.Fatal("a run with a report is neither given up on nor due")
	}
	var first string
	if err := r.pool.QueryRow(ctx, `SELECT triggered_by FROM dt_reports WHERE session_id = $1 AND version = 1`, sess.ID).Scan(&first); err != nil || first != "system" {
		t.Fatalf("version 1 kept, built by the system: %q %v", first, err)
	}
	for _, owner := range []bool{false, true} {
		pdf, v, err := r.DTReportPDF(ctx, sess.ID, owner)
		want := "%PDF-participant-admin:7"
		if owner {
			want = "%PDF-owner-admin:7"
		}
		if err != nil || v != 2 || string(pdf) != want {
			t.Fatalf("DTReportPDF(owner=%v): %q v%d %v", owner, pdf, v, err)
		}
	}

	// A send from the console is recorded, attributed, and becomes the
	// latest send.
	if err := r.DTResultEmailRecordAdmin(ctx, sess.ID, 0, 2, nil); err != nil {
		t.Fatalf("DTResultEmailRecordAdmin: %v", err)
	}
	if err := r.DTResultEmailRecordAdmin(ctx, sess.ID, 0, 2, boom); err != nil {
		t.Fatalf("DTResultEmailRecordAdmin(failed): %v", err)
	}
	st, _ = r.DTReportStateByID(ctx, sess.ID)
	if st.EmailedBy != "admin" || st.EmailedVersion != 2 {
		t.Fatalf("after the console send: by %q version %d", st.EmailedBy, st.EmailedVersion)
	}
	var system, admin int
	if err := r.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE source = 'system'), count(*) FILTER (WHERE source = 'admin')
		  FROM dt_result_emails WHERE session_id = $1`, sess.ID).Scan(&system, &admin); err != nil {
		t.Fatal(err)
	}
	if system != 1 || admin != 2 {
		t.Fatalf("%d system rows and %d admin rows", system, admin)
	}

	// A run nobody has asked about reads as having no report.
	if _, err := r.DTReportStateByKey(ctx, "00000000-0000-4000-8000-000000000000"); err == nil {
		t.Fatal("a run that does not exist has a report state")
	}
	if _, _, err := r.DTReportPDF(ctx, -1, false); err == nil {
		t.Fatal("a run with no report has a PDF")
	}
}

// Bumping the instrument's generation means deciding how its runs
// compare with the earlier ones. This fails until somebody has.
func TestTheReportKnowsTheCurrentGeneration(t *testing.T) {
	run := dtreport.Run{InstrumentVersion: DTDefaultSettings().InstrumentVersion()}
	if !dtreport.KnowsGeneration(run.Generation()) {
		t.Fatalf("generation %d (%s) has no era in dtreport: add it to eras and say what changed in generationNotes",
			run.Generation(), dtGeneration)
	}
	if run.Timings() == "" {
		t.Fatalf("the report cannot read the time limits out of %q", run.InstrumentVersion)
	}
}
