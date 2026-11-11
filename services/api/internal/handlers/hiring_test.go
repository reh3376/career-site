package handlers

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"testing"

	"connectrpc.com/connect"
	"github.com/jackc/pgx/v5/pgxpool"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/corpusscope"
	"github.com/reh3376/career-site/services/api/internal/hiring"
	"github.com/reh3376/career-site/services/api/internal/users"
)

func TestLooksLikeUUID(t *testing.T) {
	for id, want := range map[string]bool{
		"00000000-0000-0000-0000-000000000001": true,
		"A1B2C3D4-e5f6-7890-abcd-ef0123456789": true,
		"":                                     false,
		"12":                                   false,
		"00000000-0000-0000-0000-00000000000g": false,
		"00000000000000000000000000000000000x": false,
		"'; DROP TABLE requirements; --      ": false,
	} {
		if got := looksLikeUUID(id); got != want {
			t.Errorf("looksLikeUUID(%q) = %v, want %v", id, got, want)
		}
	}
}

// A patch carries only what was set, and a zero threshold is no
// threshold.
func TestPatchFromProto(t *testing.T) {
	text, class := "Lead front-end loading", "ideal"
	weight := int32(3)
	zero, ten := 0.0, 10.0
	patch, fields := patchFromProto(&v1.RequirementPatch{
		Text: &text, RequirementClass: &class, Weight: &weight,
		Thresholds:   &v1.Thresholds{MinYears: &ten, RecencyYears: &zero, MinScope: &v1.ScopeThreshold{Unit: " sites ", Value: 4}},
		Alternatives: &v1.StringList{},
	})
	if patch.Text == nil || *patch.Text != text || patch.Class == nil || *patch.Class != "ideal" || patch.Weight == nil || *patch.Weight != 3 {
		t.Errorf("scalars: %+v", patch)
	}
	if patch.Type != nil || patch.EvidenceStandard != nil || patch.EquivalenceNotes != nil || patch.Equivalency != nil || patch.NamedParties != nil {
		t.Errorf("fields that were not sent are set: %+v", patch)
	}
	if th := patch.Thresholds; th == nil || th.MinYears == nil || *th.MinYears != 10 || th.RecencyYears != nil || th.MinScope == nil || th.MinScope.Unit != "sites" {
		t.Errorf("thresholds: %+v", patch.Thresholds)
	}
	if patch.Alternatives == nil || len(*patch.Alternatives) != 0 {
		t.Errorf("a present, empty list must clear the alternatives: %v", patch.Alternatives)
	}
	if got := strings.Join(fields, ","); got != "text,class,weight,thresholds,alternatives" {
		t.Errorf("fields = %s", got)
	}
}

// A re-judge reads what the posting's last run read. The first live
// run of step 5 narrowed an owner's posting from the whole corpus to
// the public part, because the run listing did not carry the scope.
func TestRejudgeScopeFollowsTheLastRun(t *testing.T) {
	cases := []struct {
		name string
		runs []users.JdRun
		want corpusscope.Scope
	}{
		{"no runs", nil, corpusscope.Public},
		{"the owner's posting", []users.JdRun{{RetrievalScope: "all"}}, corpusscope.All},
		{"a member's posting", []users.JdRun{{RetrievalScope: "public"}}, corpusscope.Public},
		{"newest first: a member's posting an admin later re-scored", []users.JdRun{{RetrievalScope: "all"}, {RetrievalScope: "public"}}, corpusscope.All},
		{"a run from before the scope was recorded is skipped", []users.JdRun{{RetrievalScope: ""}, {RetrievalScope: "all"}}, corpusscope.All},
		{"nothing says", []users.JdRun{{RetrievalScope: ""}}, corpusscope.Public},
	}
	for _, c := range cases {
		if got := rejudgeScope(c.runs); got != c.want {
			t.Errorf("%s: scope %v, want %v", c.name, got, c.want)
		}
	}
}

func hiringStack(t *testing.T) (*Hiring, *pgxpool.Pool, context.Context) {
	t.Helper()
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		if os.Getenv("CI") != "" {
			t.Fatal("TEST_DATABASE_URL is unset in CI: the database-backed tests would skip silently")
		}
		t.Skip("TEST_DATABASE_URL not set; skipping the database-backed path")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	repo := users.New(pool)
	return NewHiring(log, repo, hiring.NewRepo(pool), hiring.NewSettingsStore(log, repo), nil), pool, ctx
}

// The requirement editor's RPCs against Postgres: list by posting,
// edit with the right refusals, release, and the settings. No scorer
// is wired, so a release releases and says no run began.
func TestHiringServiceAgainstPostgres(t *testing.T) {
	h, pool, ctx := hiringStack(t)

	var adminID, submissionID int64
	if err := pool.QueryRow(ctx, `
    INSERT INTO users (email, name, status, role)
    VALUES ('hm-' || gen_random_uuid()::text || '@example.test', 'Test Manager', 'active', 'admin') RETURNING id`).Scan(&adminID); err != nil {
		t.Fatalf("make admin: %v", err)
	}
	if err := pool.QueryRow(ctx, `
    INSERT INTO jd_submissions (source_kind, jd_text, jd_hash, text_head, status)
    VALUES ('paste', 'Senior Manager, Capital Projects. 10+ years of engineering. Front-end development of major investments.', gen_random_uuid()::text::bytea, 'Capital Projects', 'ready')
    RETURNING id`).Scan(&submissionID); err != nil {
		t.Fatalf("make submission: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM roles WHERE submission_id = $1`, submissionID)
		_, _ = pool.Exec(ctx, `DELETE FROM jd_submissions WHERE id = $1`, submissionID)
		_, _ = pool.Exec(ctx, `DELETE FROM app_settings WHERE key = $1`, hiring.SettingKey)
		_, _ = pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, adminID)
	})
	ctx = WithCaller(ctx, &users.User{ID: adminID, Role: users.RoleAdmin})
	sid := strconv.FormatInt(submissionID, 10)

	// A posting with no rows yet is an answer, with the vocabularies an
	// editor needs.
	empty, err := h.ListRequirements(ctx, connect.NewRequest(&v1.ListRequirementsRequest{SubmissionId: sid}))
	if err != nil {
		t.Fatalf("ListRequirements before any rows: %v", err)
	}
	if empty.Msg.RoleId != "" || len(empty.Msg.Requirements) != 0 || len(empty.Msg.Types) != 9 || len(empty.Msg.Classes) != 3 ||
		len(empty.Msg.ScopeUnits) != 8 || len(empty.Msg.CredentialLevels) != 6 || len(empty.Msg.RelationshipKinds) != 5 {
		t.Errorf("empty listing: role=%q reqs=%d types=%d classes=%d units=%d levels=%d kinds=%d",
			empty.Msg.RoleId, len(empty.Msg.Requirements), len(empty.Msg.Types), len(empty.Msg.Classes),
			len(empty.Msg.ScopeUnits), len(empty.Msg.CredentialLevels), len(empty.Msg.RelationshipKinds))
	}
	for _, bad := range []*v1.ListRequirementsRequest{{}, {SubmissionId: sid, RoleId: "00000000-0000-0000-0000-000000000001"}, {RoleId: "not-a-uuid"}, {SubmissionId: "abc"}} {
		if _, err := h.ListRequirements(ctx, connect.NewRequest(bad)); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("ListRequirements(%v): code %v, want invalid_argument", bad, connect.CodeOf(err))
		}
	}

	// The extraction, written as rows.
	w := hiring.NewWriter(slog.New(slog.NewTextHandler(io.Discard, nil)), h.repo)
	prepared, err := w.Prepare(ctx, submissionID, "Senior Manager, Capital Projects", []hiring.RequirementInput{
		{LocalID: "r1", Text: "Front-end development of major investments", SourceQuote: "Front-end development of major investments", Category: "must", Weight: 3, Type: "skill"},
		{LocalID: "r2", Text: "10+ years of engineering experience", SourceQuote: "10+ years of engineering", Category: "must", Weight: 2, Type: "experience"},
	}, hiring.Provenance{PromptID: "jd_requirements", PromptFingerprint: "v5:abcd", Model: "test-model"})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	listed, err := h.ListRequirements(ctx, connect.NewRequest(&v1.ListRequirementsRequest{SubmissionId: sid}))
	if err != nil || listed.Msg.RoleId != prepared.RoleID || len(listed.Msg.Requirements) != 2 || listed.Msg.HumanEdited || listed.Msg.PendingDrafts != 0 {
		t.Fatalf("listing after extraction: %+v (err %v)", listed, err)
	}
	r1, r2 := listed.Msg.Requirements[0], listed.Msg.Requirements[1]
	if r1.LocalId != "r1" || r1.Status != v1.RequirementStatus_REQUIREMENT_STATUS_ACTIVE || r1.RequirementClass != "required" || r1.TypeSource != "model" ||
		r1.EvidenceStandard == "" || r1.ReleasedByPerson || r1.ReleasedAt == nil || !r1.Included || r1.InclusionReason != "" {
		t.Errorf("r1 as listed: %+v", r1)
	}
	if r2.Included || !strings.Contains(r2.InclusionReason, "Not included") || r2.Thresholds.GetMinYears() != 10 {
		t.Errorf("r2 is a bare years item and must say it is not included: included=%v reason=%q thresholds=%+v", r2.Included, r2.InclusionReason, r2.Thresholds)
	}

	// Edits and their refusals.
	text, typ := "Lead front-end loading of major capital investments: scope definition and basis of design", "outcome"
	patch := &v1.RequirementPatch{Text: &text, Type: &typ}
	for name, c := range map[string]struct {
		req  *v1.UpdateRequirementRequest
		code connect.Code
	}{
		"no reason":       {&v1.UpdateRequirementRequest{RequirementId: r1.Id, Patch: patch}, connect.CodeInvalidArgument},
		"no patch":        {&v1.UpdateRequirementRequest{RequirementId: r1.Id, EditReason: "x"}, connect.CodeInvalidArgument},
		"a malformed id":  {&v1.UpdateRequirementRequest{RequirementId: "nope", Patch: patch, EditReason: "x"}, connect.CodeInvalidArgument},
		"an unknown id":   {&v1.UpdateRequirementRequest{RequirementId: "00000000-0000-0000-0000-0000000000aa", Patch: patch, EditReason: "x"}, connect.CodeNotFound},
		"nothing changed": {&v1.UpdateRequirementRequest{RequirementId: r1.Id, Patch: &v1.RequirementPatch{Text: &r1.Text}, EditReason: "x"}, connect.CodeInvalidArgument},
		"an unknown type": {&v1.UpdateRequirementRequest{RequirementId: r1.Id, Patch: &v1.RequirementPatch{Type: strPtr("wizardry")}, EditReason: "x"}, connect.CodeInvalidArgument},
	} {
		if _, err := h.UpdateRequirement(ctx, connect.NewRequest(c.req)); connect.CodeOf(err) != c.code {
			t.Errorf("%s: code %v (%v), want %v", name, connect.CodeOf(err), err, c.code)
		}
	}
	edited, err := h.UpdateRequirement(ctx, connect.NewRequest(&v1.UpdateRequirementRequest{
		RequirementId: r1.Id, Patch: patch, EditReason: "The posting means the definition phase of a capital project.",
	}))
	if err != nil {
		t.Fatalf("UpdateRequirement: %v", err)
	}
	d := edited.Msg.Requirement
	if d.Version != 2 || d.Status != v1.RequirementStatus_REQUIREMENT_STATUS_DRAFT || d.Supersedes != r1.Id || d.Text != text || d.Type != "outcome" ||
		d.EditedByDisplay != "Test Manager" || d.EditedAt == nil || d.EditReason == "" || d.ReleasedAt != nil {
		t.Errorf("the draft: %+v", d)
	}
	if _, err := h.UpdateRequirement(ctx, connect.NewRequest(&v1.UpdateRequirementRequest{RequirementId: r1.Id, Patch: &v1.RequirementPatch{Weight: weightPtr(1)}, EditReason: "x"})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("editing the released version beside a draft: code %v, want failed_precondition", connect.CodeOf(err))
	}
	// Defining the equivalency brings the years item into the numbers.
	eq := "A graduate engineering degree counts as two of the ten years."
	included, err := h.UpdateRequirement(ctx, connect.NewRequest(&v1.UpdateRequirementRequest{
		RequirementId: r2.Id, Patch: &v1.RequirementPatch{Equivalency: &eq}, EditReason: "Defining what stands in for the years.",
	}))
	if err != nil {
		t.Fatalf("defining the equivalency: %v", err)
	}
	if q := included.Msg.Requirement; !q.Included || !strings.Contains(q.InclusionReason, "Included") {
		t.Errorf("r2 with an equivalency: %+v", q)
	}

	pending, _ := h.ListRequirements(ctx, connect.NewRequest(&v1.ListRequirementsRequest{RoleId: prepared.RoleID}))
	if pending.Msg.PendingDrafts != 2 || !pending.Msg.HumanEdited || len(pending.Msg.Requirements) != 4 {
		t.Errorf("listing with drafts pending: drafts=%d edited=%v rows=%d", pending.Msg.PendingDrafts, pending.Msg.HumanEdited, len(pending.Msg.Requirements))
	}

	// The release. A posting mid-run refuses it.
	if _, err := pool.Exec(ctx, `UPDATE jd_submissions SET status = 'scoring' WHERE id = $1`, submissionID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ReleaseRequirements(ctx, connect.NewRequest(&v1.ReleaseRequirementsRequest{RoleId: prepared.RoleID})); connect.CodeOf(err) != connect.CodeFailedPrecondition {
		t.Errorf("release while scoring: code %v, want failed_precondition", connect.CodeOf(err))
	}
	if _, err := pool.Exec(ctx, `UPDATE jd_submissions SET status = 'ready' WHERE id = $1`, submissionID); err != nil {
		t.Fatal(err)
	}
	if _, err := h.ReleaseRequirements(ctx, connect.NewRequest(&v1.ReleaseRequirementsRequest{RoleId: "00000000-0000-0000-0000-0000000000aa"})); connect.CodeOf(err) != connect.CodeNotFound {
		t.Errorf("release of an unknown role: code %v, want not_found", connect.CodeOf(err))
	}
	rel, err := h.ReleaseRequirements(ctx, connect.NewRequest(&v1.ReleaseRequirementsRequest{RoleId: prepared.RoleID}))
	if err != nil {
		t.Fatalf("ReleaseRequirements: %v", err)
	}
	if len(rel.Msg.Released) != 2 || rel.Msg.RunStarted || rel.Msg.SubmissionId != sid {
		t.Errorf("release: released=%d run_started=%v submission=%s", len(rel.Msg.Released), rel.Msg.RunStarted, rel.Msg.SubmissionId)
	}
	for _, q := range rel.Msg.Released {
		if q.Status != v1.RequirementStatus_REQUIREMENT_STATUS_ACTIVE || !q.ReleasedByPerson {
			t.Errorf("released %s: %+v", q.LocalId, q)
		}
	}
	after, _ := h.ListRequirements(ctx, connect.NewRequest(&v1.ListRequirementsRequest{RoleId: prepared.RoleID, IncludeRetired: true}))
	statuses := map[v1.RequirementStatus]int{}
	for _, q := range after.Msg.Requirements {
		statuses[q.Status]++
	}
	if statuses[v1.RequirementStatus_REQUIREMENT_STATUS_ACTIVE] != 2 || statuses[v1.RequirementStatus_REQUIREMENT_STATUS_RETIRED] != 2 || after.Msg.PendingDrafts != 0 || !after.Msg.HumanEdited {
		t.Errorf("after the release: %v, drafts=%d, edited=%v", statuses, after.Msg.PendingDrafts, after.Msg.HumanEdited)
	}

	// The settings.
	got, err := h.GetHiringSettings(ctx, connect.NewRequest(&v1.GetHiringSettingsRequest{}))
	if err != nil {
		t.Fatalf("GetHiringSettings: %v", err)
	}
	if got.Msg.Settings.PreferredFraction != hiring.DefaultPreferredFraction {
		t.Errorf("default settings: %+v", got.Msg.Settings)
	}
	for _, bad := range []float64{0, -0.1, 1.5} {
		if _, err := h.SetHiringSettings(ctx, connect.NewRequest(&v1.SetHiringSettingsRequest{Settings: &v1.HiringSettings{PreferredFraction: bad}})); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("a preferred fraction of %v: code %v, want invalid_argument", bad, connect.CodeOf(err))
		}
	}
	if _, err := h.SetHiringSettings(ctx, connect.NewRequest(&v1.SetHiringSettingsRequest{Settings: &v1.HiringSettings{PreferredFraction: 0.5}})); err != nil {
		t.Fatalf("SetHiringSettings: %v", err)
	}
	if got, _ := h.GetHiringSettings(ctx, connect.NewRequest(&v1.GetHiringSettingsRequest{})); got.Msg.Settings.PreferredFraction != 0.5 {
		t.Errorf("settings after the change: %+v", got.Msg.Settings)
	}
}

func strPtr(s string) *string  { return &s }
func weightPtr(n int32) *int32 { return &n }
