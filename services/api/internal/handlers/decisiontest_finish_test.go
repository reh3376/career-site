package handlers

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// FinishSession must not resolve its session through session().
//
// This reads source, which is crude, and it is the check that was
// missing. The fault it guards cannot be seen any other way without a
// database and a wired-up handler, and it cost every participant's
// answer to the debrief question.
//
// What happened: the run screen closes the run BEFORE showing the
// debrief, so that a participant who shuts the tab on that question is
// recorded as having completed thirty answers rather than abandoned
// them. session() refuses any run whose status is not `running`, on the
// no-resume rule. So the debrief's FinishSession call was refused with
// FailedPrecondition every time, the client caught it on the grounds
// that "only the strategy is lost", and the strategy was lost on every
// single run that went through a browser.
//
// Nothing failed. The column, the proto field, the handler allowlist
// and three buttons all existed and agreed with each other. The only
// symptom was an empty text column, which is indistinguishable from a
// question nobody chose to answer.
//
// Two rules, and they pull against each other, which is why writing one
// broke the other:
//
//   - A closed run cannot be ADDED TO. No answer, no recall, no block.
//     Resuming would poison the timing control the instrument depends
//     on, because a question answered after a four-minute interruption
//     is not the same question and nothing in the data would say so.
//   - A closed run CAN still say which method was used. The debrief
//     carries no answer, no timing and no item, and the honesty of it
//     is not a function of when it was typed.
func TestFinishSessionAcceptsAClosedRun(t *testing.T) {
	b, err := os.ReadFile("decisiontest.go")
	if err != nil {
		t.Fatalf("read the decision test handler: %v", err)
	}
	src := string(b)

	body := funcBody(src, "func (h *DecisionTest) FinishSession(")
	if body == "" {
		t.Fatal("could not find FinishSession: this test has drifted and is " +
			"asserting nothing, which is the same failure mode it exists for")
	}
	if !strings.Contains(body, "h.sessionToFinish(ctx,") {
		t.Error("FinishSession does not resolve its handle with sessionToFinish. " +
			"If it uses session(), every debrief answer is refused with " +
			"FailedPrecondition, because the run screen closes the run before " +
			"it asks the question")
	}
	if strings.Contains(body, "h.session(ctx,") {
		t.Error("FinishSession calls session(), which refuses any run that is " +
			"not running. The run is already completed by the time the debrief " +
			"is answered")
	}

	// And the other half: the ordinary RPCs must keep the strict guard,
	// or the fix to one bug becomes a resume path.
	for _, name := range []string{
		"func (h *DecisionTest) GetBlock(",
		"func (h *DecisionTest) SubmitAnswer(",
		"func (h *DecisionTest) SubmitRecall(",
	} {
		fb := funcBody(src, name)
		if fb == "" {
			t.Errorf("could not find %s", name)
			continue
		}
		if !strings.Contains(fb, "h.session(ctx,") {
			t.Errorf("%s no longer uses the strict session() guard: a closed run "+
				"could be added to, and there is no resume (FSD 5b)", name)
		}
	}
}

// funcBody returns the text of the function whose declaration starts
// with `decl`, up to the closing brace in column one.
func funcBody(src, decl string) string {
	i := strings.Index(src, decl)
	if i < 0 {
		return ""
	}
	rest := src[i:]
	if end := strings.Index(rest, "\n}\n"); end >= 0 {
		return rest[:end]
	}
	return rest
}

// A prior count of zero must survive serialisation.
//
// This is the bug the live pass found and no other check could have.
// The three prior-sitting counts were plain `int32`, and under proto3
// implicit presence a scalar equal to its default is omitted from the
// JSON entirely. So an identity that existed and saw no earlier
// sittings arrived at the browser as `undefined`, indistinguishable
// from an identity that never existed at all.
//
// That is the one distinction the three columns exist to preserve. The
// console read it back as "no account" for an account that was there,
// which is a confident false statement about a participant's history.
//
// Everything passed: the SQL was right, the Go was right, the page
// rendered, the types checked. The fault lived entirely in the wire
// format, between two correct halves.
func TestAZeroPriorCountSurvivesTheWire(t *testing.T) {
	zero := 0
	run := toProtoRun(users.DTRun{
		SessionKey:     "wire-test",
		PriorByAccount: &zero,
		// Email and cookie left nil: no such link.
	})

	if run.PriorByAccount == nil {
		t.Fatal("prior_by_account is nil for an account that saw zero prior " +
			"sittings: the field needs explicit presence, or the zero is " +
			"dropped and reads as 'no account'")
	}
	if got := *run.PriorByAccount; got != 0 {
		t.Errorf("prior_by_account = %d, want 0", got)
	}
	// And the genuinely absent ones must stay absent, or the fix would
	// have invented links that were never there.
	if run.PriorByEmail != nil {
		t.Errorf("prior_by_email = %d, want absent", *run.PriorByEmail)
	}
	if run.PriorByCookie != nil {
		t.Errorf("prior_by_cookie = %d, want absent", *run.PriorByCookie)
	}

	// The JSON is where it actually broke, so check the JSON.
	b, err := protojson.Marshal(run)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var m map[string]any
	if err := json.Unmarshal(b, &m); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, ok := m["priorByAccount"]; !ok {
		t.Error("priorByAccount is missing from the JSON although it is zero: " +
			"the browser cannot tell that from an account that never existed")
	}
	if _, ok := m["priorByEmail"]; ok {
		t.Error("priorByEmail is present in the JSON although there was no such link")
	}
}
