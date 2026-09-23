package jd

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"

	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// PostingVerdict is what the gatekeeper decided about the submitted
// text.
type PostingVerdict struct {
	IsPosting bool   `json:"is_posting"`
	Kind      string `json:"kind"`
	Reason    string `json:"reason"`
}

// minPostingRunes is the length below which no text describes a role.
//
// The submission that prompted all this was 365 characters. A genuine
// posting can be short, so this is set well below anything plausible
// and exists only to save a model call on input that is obviously not a
// posting: a pasted line, an empty box, a URL.
const minPostingRunes = 180

// CheckPosting decides whether the text is a job posting, before the
// expensive stages run.
//
// It fails open. A classifier that is down, slow, or confused must not
// stop a real posting from being assessed: the cost of wrongly refusing
// a hiring manager's posting is far higher than the cost of scoring
// something odd, so every error path here returns "carry on".
//
// The verdict is logged like any other decision, which means it can be
// reviewed and graded in the console, and its agreement with the owner
// measured. A gatekeeper nobody can audit is how a system quietly
// starts refusing real work.
func (a *Assessor) CheckPosting(ctx context.Context, submissionID int64, text string) PostingVerdict {
	ok := PostingVerdict{IsPosting: true, Kind: "job_posting"}
	if a == nil {
		return ok
	}
	trimmed := strings.TrimSpace(text)
	if v, ok := tooShortToBeAPosting(trimmed); ok {
		a.logPostingCheck(ctx, submissionID, text, v, callResult{Model: "code"}, "")
		return v
	}
	if err := a.checkCapN(ctx, 1); err != nil {
		a.log.Warn("posting check skipped, call cap", slog.Int64("jd_id", submissionID))
		return ok
	}

	var out PostingVerdict
	user := prompts.RenderPostingCheckUser(trimmed)
	res, err := a.call(ctx, submissionID, prompts.PostingCheck, user, 300, &out)
	if err != nil {
		// Fails open, loudly. The run continues and the log says why the
		// gate did not run, so a silently missing check is visible.
		a.log.Warn("posting check failed, continuing",
			slog.Int64("jd_id", submissionID), slog.String("error", err.Error()))
		return ok
	}
	out.Kind = strings.ToLower(strings.TrimSpace(out.Kind))
	out.Reason = strings.TrimSpace(out.Reason)
	if out.Reason == "" {
		out.Reason = "This does not look like a job posting."
	}
	a.logPostingCheck(ctx, submissionID, text, out, res, user)
	if !out.IsPosting {
		a.log.Info("jd: not a posting",
			slog.Int64("jd_id", submissionID), slog.String("kind", out.Kind))
	}
	return out
}

// logPostingCheck records the gatekeeper's decision beside every other
// decision the pipeline makes.
func (a *Assessor) logPostingCheck(
	ctx context.Context, submissionID int64, text string,
	v PostingVerdict, res callResult, userPrompt string,
) {
	input, _ := json.Marshal(map[string]any{
		"text_head": prompts.CapRunes(strings.TrimSpace(text), 2000),
		"runes":     len([]rune(strings.TrimSpace(text))),
	})
	output, _ := json.Marshal(v)
	model := res.Model
	if model == "" {
		model = "unknown"
	}
	promptText := ""
	if userPrompt != "" {
		promptText = prompts.PostingCheck.System + "\n\n" + userPrompt
	}
	if err := a.users.InsertDecisions(context.WithoutCancel(ctx), []users.Decision{{
		Kind:          "jd_posting_check",
		RefKind:       "jd_submission",
		RefID:         submissionID,
		Model:         model,
		PromptID:      prompts.PostingCheck.ID,
		PromptVersion: prompts.PostingCheck.Version,
		NumCtx:        a.numCtx,
		Input:         input,
		Output:        output,
		PromptText:    promptText,
		ResponseText:  res.Text,
		PromptTokens:  res.PromptTokens,
		CompletionTok: res.CompletionTokens,
		LatencyMs:     res.LatencyMs,
	}}); err != nil {
		a.log.Warn("posting check not logged",
			slog.Int64("jd_id", submissionID), slog.String("error", err.Error()))
	}
}

// tooShortToBeAPosting is the part of the check that needs no model.
// Kept pure so the threshold is testable, and deliberately generous:
// its job is to save a call on an empty box or a stray line, not to
// second-guess a terse posting.
func tooShortToBeAPosting(trimmed string) (PostingVerdict, bool) {
	if len([]rune(trimmed)) >= minPostingRunes {
		return PostingVerdict{}, false
	}
	return PostingVerdict{
		IsPosting: false,
		Kind:      "fragment",
		Reason:    "This is too short to describe a role. Paste the whole posting, including what the job involves and what it asks for.",
	}, true
}

// NotAPostingMessage is what the submitter is told. It says what the
// text looked like and what to do, and never implies they did
// something wrong: the commonest cause is pasting the wrong clipboard.
func NotAPostingMessage(v PostingVerdict) string {
	reason := v.Reason
	if reason == "" {
		reason = "This does not look like a job posting."
	}
	return fmt.Sprintf("%s Nothing was scored, because a score computed from text that is not a posting would mean nothing. Paste the full job description and submit again.", reason)
}
