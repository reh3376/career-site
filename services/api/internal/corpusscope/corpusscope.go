// Package corpusscope carries how much of the corpus a request may
// retrieve from.
//
// Most of the corpus is private. Of the documents in production, the
// large majority are `corpus_only`: client work, material under NDA,
// and personal records that exist so the owner's own reviews can draw
// on his whole history. Five are public.
//
// Retrieval had no visibility predicate at all, so a job description
// submitted by a member drove similarity search across all of it, and
// the retrieved text entered the judge's context and the résumé
// generator. The only thing standing between that text and the
// submitter was a line in the judge prompt asking the model not to
// quote private chunks. A submitted posting is an instruction channel
// aimed at that same model, so the prompt is the wrong place for the
// control. This package moves it into SQL.
//
// The default is Public and that is deliberate. A scope that has to be
// asked for cannot be forgotten into being permissive: code that
// neglects to widen the scope retrieves less than it could, which is a
// bug someone notices, rather than more than it should, which is a
// leak nobody notices.
package corpusscope

import "context"

// Scope is how much of the corpus retrieval may see.
type Scope int

const (
	// Public restricts retrieval to documents marked public. This is
	// the zero value, so a context that was never marked gets it.
	Public Scope = iota
	// All lifts the restriction. It belongs to work the owner starts
	// for himself: his own submissions, and evaluations of the golden
	// set, which exist to measure what he gets rather than what a
	// visitor gets.
	All
)

type contextKey struct{}

// With returns a context carrying the scope.
func With(ctx context.Context, s Scope) context.Context {
	return context.WithValue(ctx, contextKey{}, s)
}

// FromContext returns the scope in force, or Public.
func FromContext(ctx context.Context) Scope {
	if ctx == nil {
		return Public
	}
	if s, ok := ctx.Value(contextKey{}).(Scope); ok {
		return s
	}
	return Public
}

// AllowsPrivate reports whether private documents may be retrieved.
func AllowsPrivate(ctx context.Context) bool {
	return FromContext(ctx) == All
}
