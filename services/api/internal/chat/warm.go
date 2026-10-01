package chat

import (
	"context"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/reh3376/career-site/services/api/internal/llm"
	"github.com/reh3376/career-site/services/api/internal/prompts"
	"github.com/reh3376/career-site/services/api/internal/tenant"
	"github.com/reh3376/career-site/services/api/internal/users"
)

// Keeping the prompt prefix in Ollama's KV cache.
//
// Every chat prompt begins with the same two things: the persona system
// prompt and the career facts sheet. Together they are about two
// thousand tokens, and on this box prompt evaluation runs at roughly 29
// tokens a second, so evaluating them costs over a minute.
//
// It only costs that once. llama.cpp keeps the KV cache for the tokens
// it last evaluated and reuses the longest matching prefix, so a second
// request starting with the same bytes skips straight to the part that
// differs. Measured on production, same prefix, different questions:
//
//	cold                        2435 tokens   prompt_eval  82.5 s
//	after a warm-up call        2435 tokens   prompt_eval   0.8 s
//	next question, still warm   2434 tokens   prompt_eval   0.7 s
//
// So this sends the prefix on a timer with num_predict 1, which makes
// Ollama evaluate it and return without writing an answer. When the
// cache is already warm that call costs 0.5 s, so the loop is nearly
// free and pays the full price only when something evicted it.
//
// # What evicts it
//
// OLLAMA_NUM_PARALLEL is 1, so there is one cache slot. A JD evaluation
// uses a different system prompt and takes the slot, so chat goes cold
// while one runs and warms again afterwards. That is survivable and
// worth knowing: during an eval, answers are slow.
//
// # Why not simply retrieve less
//
// Because the facts sheet is what makes the answers right. Retrieval is
// unreliable for tenure, titles, dates and credentials, which is most
// of what anyone asks. The choice is not between fast and slow, it is
// between warming a cache and answering from the wrong documents.

// WarmSource supplies the facts block. Separated so the warmer and the
// answer path cannot drift: both ask the same thing for the same bytes.
type WarmSource interface {
	ListChunksByKind(ctx context.Context, sourceKind string, limit int) ([]users.CorpusHit, error)
}

// BusySource reports whether something else needs the one cache slot.
type BusySource interface {
	JDRunInProgress(ctx context.Context) (bool, error)
}

// Facts loads and renders the career facts sheet.
//
// Cached in memory with a short life. The text has to be byte-identical
// across calls or the prefix breaks, and a database round trip per
// question would be a second chance per question for it to come back in
// a different order. The TTL is what lets a corpus reindex take effect
// without a restart.
type Facts struct {
	Source WarmSource
	Log    *slog.Logger
	// TTL is how long a loaded sheet is reused. Zero means five
	// minutes, which matches the embedding job's interval, so a
	// reindexed sheet is in use within one cycle.
	TTL time.Duration

	mu     sync.Mutex
	text   string
	loaded time.Time
}

// Text returns the facts block, loading it if the cached copy is stale.
//
// An error returns the empty string rather than failing: an assistant
// answering without the facts sheet is worse than one that was, and far
// better than one that refuses to answer at all.
func (f *Facts) Text(ctx context.Context) string {
	if f == nil || f.Source == nil {
		return ""
	}
	ttl := f.TTL
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	if f.text != "" && time.Since(f.loaded) < ttl {
		return f.text
	}

	// The tenant rides the context, and a timer-driven caller has none.
	chunks, err := f.Source.ListChunksByKind(
		tenant.WithID(ctx, tenant.Default), prompts.ProfileSourceKind, 8)
	if err != nil {
		f.log().Warn("career facts sheet unavailable; answering without it",
			slog.String("error", err.Error()))
		return f.text // whatever was loaded last, possibly empty
	}

	// Ordered by chunk index, which ListChunksByKind already does, and
	// joined the same way every time. Any instability here is a cache
	// miss on every question, which would look like the warming simply
	// not working.
	var b strings.Builder
	for i, c := range chunks {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(strings.TrimSpace(c.Chunk.Text))
	}
	f.text = b.String()
	f.loaded = time.Now()
	return f.text
}

func (f *Facts) log() *slog.Logger {
	if f.Log != nil {
		return f.Log
	}
	return slog.Default()
}

// Warmer keeps the prefix in the cache.
type Warmer struct {
	Model Generator
	Facts *Facts
	// Busy is asked before every tick. Nil warms unconditionally.
	Busy BusySource
	Log  *slog.Logger
}

// Run sends the prefix and asks for one token.
//
// num_predict 1 rather than 0 because a model asked for nothing can
// return before the prompt is evaluated, which would warm nothing. One
// token costs an eighth of a second and guarantees the work was done.
func (w *Warmer) Run(ctx context.Context) error {
	if w == nil || w.Model == nil {
		return nil
	}
	// Stand aside for a JD run.
	//
	// There is one KV cache slot, so a run and the chat prefix cannot
	// both be cached. Warming through a run would evaluate two thousand
	// tokens, be evicted by the run's next call, and do it again on the
	// next tick: minutes of CPU spent on a cache nothing will read,
	// taken from the run that is actually working. Chat is slow during
	// an evaluation either way; this stops it also making the
	// evaluation slower.
	if w.Busy != nil {
		if busy, err := w.Busy.JDRunInProgress(ctx); err == nil && busy {
			w.log().Debug("prefix warming paused; a JD run holds the cache slot")
			return nil
		}
	}

	facts := w.Facts.Text(ctx)
	if facts == "" {
		// Nothing stable to warm beyond the system prompt, which the
		// next real question will cache anyway.
		return nil
	}

	started := time.Now()
	_, err := w.Model.Generate(ctx, llm.Request{
		System:      prompts.AskRogerPersona.System,
		User:        prompts.AskPrefix(facts),
		MaxTokens:   1,
		Temperature: 0,
	})
	if err != nil {
		// Not fatal and not an error the owner needs woken for: the
		// next question pays full price and still answers.
		w.log().Warn("prompt prefix not warmed", slog.String("error", err.Error()))
		return nil
	}

	// Logged at info with the elapsed time because that number is the
	// diagnosis: a tick that takes half a second means the cache held,
	// and one that takes a minute means something evicted it, which on
	// this box means a JD evaluation was running.
	w.log().Info("prompt prefix warmed", slog.Duration("elapsed", time.Since(started)))
	return nil
}

func (w *Warmer) log() *slog.Logger {
	if w.Log != nil {
		return w.Log
	}
	return slog.Default()
}
