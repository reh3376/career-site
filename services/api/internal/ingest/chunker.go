// Package ingest turns source text (markdown, plain text) into
// corpus chunks + embeddings and writes them to the DB.
//
// The chunker is factored behind a `Chunker` interface so a
// heading-aware v2 can drop in later without touching ingest
// callers. Splitting rules for v1 are simple by design (paragraph
// first, sentence fallback, hard-cap by rune count) so a change
// has predictable ripple effects at retrieval time.
package ingest

import (
	"strings"
	"unicode"
)

// Chunk is one retrieval-sized slice of a source document.
type Chunk struct {
	Text       string
	TokenCount int32
	Index      int32
}

// Chunker splits source text into retrieval-sized chunks. v1 is
// paragraph-first; a v2 heading-aware splitter can implement the
// same interface. Callers should hold this type, not the concrete
// `ChunkText` function, so the swap is a one-line change.
type Chunker interface {
	Chunk(text string) []Chunk
	// Name reports a stable identifier for the chunker (e.g.
	// "paragraph.v1"). Persisted alongside chunks so a future
	// migration can tell which generation produced which rows.
	Name() string
}

// ParagraphChunker is the v1 implementation. Options are captured
// at construction so the same instance can be reused across calls.
type ParagraphChunker struct {
	Opts Options
}

// NewParagraphChunker returns a Chunker with the given options
// (defaults applied for zero fields).
func NewParagraphChunker(opts Options) *ParagraphChunker {
	return &ParagraphChunker{Opts: opts.withDefaults()}
}

func (c *ParagraphChunker) Chunk(text string) []Chunk { return ChunkText(text, c.Opts) }
func (c *ParagraphChunker) Name() string              { return "paragraph.v1" }

// Options controls how the chunker splits. Zero-valued fields fall
// back to defaults that suit article-length prose:
//   - MaxRunes: 3000 (≈ 600 tokens on English prose at ~5 chars/token)
//   - OverlapRunes: 450 (~15% overlap so a paragraph split near a
//     boundary still returns a coherent chunk from at least one side)
//   - MinRunes: 200 (a shard smaller than this gets folded into its
//     previous chunk instead of standing alone)
type Options struct {
	MaxRunes     int
	OverlapRunes int
	MinRunes     int
}

func (o Options) withDefaults() Options {
	if o.MaxRunes <= 0 {
		o.MaxRunes = 3000
	}
	if o.OverlapRunes < 0 {
		o.OverlapRunes = 0
	}
	if o.OverlapRunes >= o.MaxRunes {
		o.OverlapRunes = o.MaxRunes / 5 // never overlap more than 20%
	}
	if o.MinRunes <= 0 {
		o.MinRunes = 200
	}
	return o
}

// Chunk splits `text` into overlapping chunks. The algorithm:
//  1. Normalise line endings and trim.
//  2. Split on blank-line paragraph boundaries.
//  3. Greedy-pack paragraphs into a rolling buffer up to MaxRunes.
//  4. When the buffer reaches MaxRunes, emit a chunk and keep the
//     tail (OverlapRunes runes) as the seed of the next chunk.
//  5. Any single paragraph longer than MaxRunes is sentence-split;
//     a paragraph longer than MaxRunes with no sentence breaks is
//     hard-split at MaxRunes.
//  6. A trailing shard shorter than MinRunes is appended to the
//     previous chunk instead of emitted on its own.
//
// The TokenCount field is a rough estimate: ceil(len(text) / 4) —
// matches the rule of thumb English tokenisers use and is close
// enough for prompt-budget accounting. Real tokenisation happens at
// LLM call time.
func ChunkText(text string, opts Options) []Chunk {
	opts = opts.withDefaults()
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.TrimSpace(text)
	if text == "" {
		return nil
	}

	paragraphs := splitParagraphs(text)
	// Expand any paragraph over the cap into sentences (or hard-split).
	expanded := make([]string, 0, len(paragraphs))
	for _, p := range paragraphs {
		if runeLen(p) <= opts.MaxRunes {
			expanded = append(expanded, p)
			continue
		}
		expanded = append(expanded, splitSentences(p, opts.MaxRunes)...)
	}

	var (
		out    []Chunk
		buf    strings.Builder
		bufLen int
		emit   = func() {
			s := strings.TrimSpace(buf.String())
			if s == "" {
				return
			}
			out = append(out, Chunk{Text: s, TokenCount: estimateTokens(s)})
			buf.Reset()
			bufLen = 0
		}
	)

	for _, p := range expanded {
		// If adding this paragraph would exceed MaxRunes, emit
		// what's in the buffer, seed the next with the overlap tail.
		if bufLen+runeLen(p)+2 > opts.MaxRunes && bufLen > 0 {
			tail := runeTail(buf.String(), opts.OverlapRunes)
			emit()
			if tail != "" {
				buf.WriteString(tail)
				buf.WriteString("\n\n")
				bufLen = runeLen(tail) + 2
			}
		}
		if bufLen > 0 {
			buf.WriteString("\n\n")
			bufLen += 2
		}
		buf.WriteString(p)
		bufLen += runeLen(p)
	}
	emit()

	// Fold a small trailing shard into its predecessor.
	if len(out) >= 2 && runeLen(out[len(out)-1].Text) < opts.MinRunes {
		last := out[len(out)-1]
		merged := out[len(out)-2].Text + "\n\n" + last.Text
		out = out[:len(out)-1]
		out[len(out)-1].Text = merged
		out[len(out)-1].TokenCount = estimateTokens(merged)
	}

	for i := range out {
		out[i].Index = int32(i)
	}
	return out
}

func splitParagraphs(text string) []string {
	// Any run of blank lines (2+ newlines) marks a paragraph break.
	raw := strings.Split(text, "\n\n")
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// splitSentences splits a too-long paragraph on sentence-ending
// punctuation followed by whitespace. If a sentence is still over
// max, it hard-splits by rune count.
func splitSentences(paragraph string, max int) []string {
	var out []string
	var buf strings.Builder
	bufLen := 0
	runes := []rune(paragraph)
	flush := func() {
		s := strings.TrimSpace(buf.String())
		if s != "" {
			out = append(out, s)
		}
		buf.Reset()
		bufLen = 0
	}
	for i, r := range runes {
		buf.WriteRune(r)
		bufLen++
		isSentenceEnd := (r == '.' || r == '?' || r == '!') &&
			(i+1 >= len(runes) || unicode.IsSpace(runes[i+1]))
		if isSentenceEnd && bufLen >= max/2 {
			flush()
		} else if bufLen >= max {
			flush()
		}
	}
	flush()
	return out
}

func runeLen(s string) int {
	n := 0
	for range s {
		n++
	}
	return n
}

// runeTail returns the last `n` runes of `s`, cutting at a word
// boundary if there's one within the last 40 runes so the seed of
// the next chunk doesn't start mid-word.
func runeTail(s string, n int) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	start := len(runes) - n
	// Try to snap to a whitespace boundary within 40 runes forward.
	for i := start; i < start+40 && i < len(runes); i++ {
		if unicode.IsSpace(runes[i]) {
			start = i + 1
			break
		}
	}
	return strings.TrimSpace(string(runes[start:]))
}

// estimateTokens is the "chars/4" rule of thumb English tokenisers
// use. Off by a factor for other languages, close enough for prompt
// budgeting.
func estimateTokens(s string) int32 {
	// Ceil division so a 3-char string counts as at least 1 token.
	n := (runeLen(s) + 3) / 4
	if n < 1 {
		return 1
	}
	return int32(n)
}
