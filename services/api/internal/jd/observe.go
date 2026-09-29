package jd

import "context"

// An observer watches a single scoring run's stage changes as they
// happen, without changing where they are recorded.
//
// Every submission already reports its stage: "checking the posting",
// "judging requirement 7 of 14", "writing the tailored résumé". The
// scorer writes those to jd_submissions.progress_stage, which is what
// the member sees on their own review.
//
// An evaluation is nine of those runs inside one job, and the job
// reported once per posting. So /admin/ops sat on "scoring blue-origin
// (5 of 9)" for forty minutes while the pipeline underneath moved
// through extraction, fourteen judgments and a résumé. The information
// existed; nothing carried it up to the job.
//
// It travels on the context rather than in a signature because it is a
// property of one call, not of the scorer, which is shared by every
// member submission. A field would be shared mutable state and would
// leak one caller's observer into another's run. This is the same
// reason corpusscope is passed this way.
type observer func(pct int32, stage string)

type observerKey struct{}

// withObserver returns a context whose scoring run reports its stages
// to fn. Only the evaluator sets one; a member's submission carries no
// observer and the scorer's behaviour is identical either way.
func withObserver(ctx context.Context, fn observer) context.Context {
	if fn == nil {
		return ctx
	}
	return context.WithValue(ctx, observerKey{}, fn)
}

// observerFrom returns the observer on ctx, or nil. Callers invoke the
// result through notify rather than testing it.
func observerFrom(ctx context.Context) observer {
	fn, _ := ctx.Value(observerKey{}).(observer)
	return fn
}

// notify reports a stage if anyone is watching. A no-op on the member
// path, which is every submission but an evaluation's.
func (o observer) notify(pct int32, stage string) {
	if o == nil {
		return
	}
	o(pct, stage)
}
