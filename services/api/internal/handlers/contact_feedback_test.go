package handlers

import (
	"context"
	"os"
	"strings"
	"testing"

	"connectrpc.com/connect"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
	"github.com/reh3376/career-site/services/api/internal/turnstile"
)

// A decision-test participant may write without identifying themselves.
//
// The test tells participants every field is optional and that the
// measurement does not need their name, and the thanks page repeats it
// one paragraph above the feedback box. Requiring a name there would
// mean a participant who took that at its word has to invent one before
// they can say the test was hard.
//
// The relaxation is narrow on purpose, so each test below pins one edge
// of it: who may omit a name, who may not, what is still validated when
// given, and that the bot gate did not move.

func feedbackReq(name, email, token string) *connect.Request[v1.SubmitContactRequest] {
	r := connect.NewRequest(&v1.SubmitContactRequest{
		Subject:        "decision test",
		Message:        "the fourth block was brutal",
		Category:       v1.SupportCategory_SUPPORT_CATEGORY_DECISION_TEST_FEEDBACK,
		Name:           name,
		Email:          email,
		TurnstileToken: token,
	})
	r.Header().Set("X-Forwarded-For", "203.0.113.7")
	return r
}

// reachedTheRepository is blockedAtTheGate inverted. Past validation the
// handler reaches the nil repository and panics, so a panic is how this
// package observes "the submission was accepted" without a database.
func reachedTheRepository(t *testing.T, what string) {
	t.Helper()
	if r := recover(); r == nil {
		t.Fatalf("%s was refused before it reached the repository, so the "+
			"handler rejected it during validation", what)
	}
}

func TestDecisionTestFeedbackIsAcceptedWithoutANameOrEmail(t *testing.T) {
	defer reachedTheRepository(t, "unsigned decision-test feedback")
	h := newGateContact(turnstile.Noop{})
	_, _ = h.SubmitContact(context.Background(), feedbackReq("", "", "any-token"))
}

// The relaxation is per category, not a general loosening: every other
// anonymous sender still has to say who they are.
func TestAnonymousContactStillRequiresANameOutsideThatCategory(t *testing.T) {
	h := newGateContact(turnstile.Noop{})
	r := feedbackReq("", "", "any-token")
	r.Msg.Category = v1.SupportCategory_SUPPORT_CATEGORY_GENERAL_QUESTION

	_, err := h.SubmitContact(context.Background(), r)
	if err == nil {
		t.Fatal("an anonymous general question with no name or email was accepted")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want invalid_argument", got)
	}
}

// An address that is given is still checked. Feedback carrying a
// mistyped address is worse than feedback carrying none: it reads as a
// note someone is waiting on a reply to, and the reply bounces.
func TestDecisionTestFeedbackStillValidatesAnAddressThatIsGiven(t *testing.T) {
	h := newGateContact(turnstile.Noop{})

	_, err := h.SubmitContact(context.Background(),
		feedbackReq("A Participant", "not-an-address", "any-token"))
	if err == nil {
		t.Fatal("feedback with a malformed email address was accepted")
	}
	if got := connect.CodeOf(err); got != connect.CodeInvalidArgument {
		t.Fatalf("code = %v, want invalid_argument", got)
	}
}

// This relaxes who may write, never whether a bot may. If the gate ever
// moves behind the identity check, this is the test that says so.
func TestDecisionTestFeedbackStillPassesTheBotGate(t *testing.T) {
	defer blockedAtTheGate(t)
	fake := &fakeVerifier{verdict: turnstile.Verdict{OK: false, ErrorCodes: []string{"invalid-input-response"}}}
	h := newGateContact(fake)

	_, err := h.SubmitContact(context.Background(), feedbackReq("", "", "bad-token"))
	if err == nil {
		t.Fatal("unsigned feedback with a rejected Turnstile token was accepted")
	}
	if got := connect.CodeOf(err); got != connect.CodeFailedPrecondition {
		t.Fatalf("code = %v, want failed_precondition", got)
	}
	if fake.calls != 1 {
		t.Fatalf("verifier called %d times, want 1", fake.calls)
	}
}

// The run must never be folded into the auth note.
//
// It was, for one deploy. The template renders any non-empty `Auth` as
// "Signed in: yes (...)", so appending the run to it made every
// anonymous decision-test note report the sender as signed in, which is
// the opposite of true. Both halves were right in isolation and the
// first real email was what found it, so the shape is pinned here.
func TestTheRunIsNotFoldedIntoTheSignedInNote(t *testing.T) {
	tmpl := mustReadTemplate(t)
	if !strings.Contains(tmpl, "{{if .Run}}") {
		t.Fatal("the notification template has no separate Run field, so the " +
			"run can only be carried by overloading another one")
	}
	// The row the run belongs in must not be the row that answers
	// "was this sender signed in".
	auth := strings.Index(tmpl, "Signed in")
	run := strings.Index(tmpl, "{{if .Run}}")
	if auth == -1 || run == -1 || auth == run {
		t.Fatalf("Signed in at %d, Run at %d: expected two distinct rows", auth, run)
	}
}

func mustReadTemplate(t *testing.T) string {
	t.Helper()
	b, err := os.ReadFile("../email/templates/support_notification.txt.tmpl")
	if err != nil {
		t.Fatalf("read template: %v", err)
	}
	return string(b)
}
