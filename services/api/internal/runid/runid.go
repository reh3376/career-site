// Package runid carries which pipeline run the current work belongs to.
//
// A JD assessment fans out into one requirement-extraction call, one
// model call per requirement, and one résumé call, each writing rows to
// llm_usage and decision_log from a different place in the code. Those
// rows are only useful if they can be tied back to the run that made
// them, and threading an id through every signature between the scorer
// and the gateway would touch a dozen functions to carry one value that
// none of them read.
//
// So it rides the context, like the tenant does. The rule is the same:
// the writer asks, nobody hardcodes. An empty id is legitimate and
// means "not part of a run", which is what a call from outside the JD
// pipeline looks like; the column is nullable for exactly that reason.
package runid

import (
	"context"
	"crypto/rand"
	"encoding/hex"
)

type contextKey struct{}

// New mints a run id. It is a UUID because the column is one; the
// format matters more than the version.
func New() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("runid: crypto/rand unavailable: " + err.Error())
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// With returns a context bound to a run. The scorer calls this once,
// after the run row exists, and passes the result down the pipeline.
func With(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, contextKey{}, id)
}

// FromContext returns the run this work belongs to, or "" when it is
// not part of one. Writers store the empty string as NULL.
func FromContext(ctx context.Context) string {
	if id, ok := ctx.Value(contextKey{}).(string); ok {
		return id
	}
	return ""
}
