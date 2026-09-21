package ingest

import (
	"strings"
	"testing"
)

func TestChunkText_shortInputStaysAsOneChunk(t *testing.T) {
	text := "Short paragraph.\n\nAnother short paragraph."
	got := ChunkText(text, Options{})
	if len(got) != 1 {
		t.Fatalf("want 1 chunk, got %d", len(got))
	}
	if !strings.Contains(got[0].Text, "Short paragraph.") {
		t.Errorf("chunk missing content: %q", got[0].Text)
	}
	if got[0].Index != 0 {
		t.Errorf("index=0 expected, got %d", got[0].Index)
	}
	if got[0].TokenCount < 1 {
		t.Errorf("token count should be at least 1, got %d", got[0].TokenCount)
	}
}

func TestChunkText_splitsLongInput(t *testing.T) {
	// Build ~9000 runes as ten 900-rune paragraphs. Default cap is
	// 3000 → expect 4-ish chunks with overlap.
	para := strings.Repeat("word ", 180) // ~900 runes
	var b strings.Builder
	for range 10 {
		b.WriteString(para)
		b.WriteString("\n\n")
	}
	got := ChunkText(b.String(), Options{})
	if len(got) < 3 {
		t.Fatalf("want ≥3 chunks, got %d", len(got))
	}
	for i, c := range got {
		if c.Index != int32(i) {
			t.Errorf("chunk[%d].Index = %d, want %d", i, c.Index, i)
		}
		if runeLen(c.Text) > 3200 {
			t.Errorf("chunk[%d] over cap: %d runes", i, runeLen(c.Text))
		}
	}
}

func TestChunkText_paragraphOverCapSentenceSplits(t *testing.T) {
	// One 4000-rune paragraph with sentences. Should sentence-split.
	sentence := strings.Repeat("word ", 40) + "end. " // ~205 runes ending in "end."
	text := strings.Repeat(sentence, 20)              // ~4100 runes, one paragraph
	got := ChunkText(text, Options{})
	if len(got) < 2 {
		t.Fatalf("want ≥2 chunks from long paragraph, got %d", len(got))
	}
}

func TestChunkText_overlapPreservesTail(t *testing.T) {
	// Two big paragraphs that force a chunk boundary.
	p1 := strings.Repeat("alpha ", 300) // 1800 runes
	p2 := strings.Repeat("beta ", 300)  // 1500 runes
	text := p1 + "\n\n" + p2
	got := ChunkText(text, Options{MaxRunes: 2000, OverlapRunes: 300})
	if len(got) < 2 {
		t.Fatalf("want ≥2 chunks, got %d", len(got))
	}
	// Second chunk should start with some tail of the first (the
	// overlap seed) — look for "alpha" appearing in chunk 2.
	if !strings.Contains(got[1].Text, "alpha") {
		t.Errorf("chunk 2 missing overlap tail; text=%q", got[1].Text)
	}
}

func TestChunkText_emptyInputReturnsNil(t *testing.T) {
	if got := ChunkText("", Options{}); got != nil {
		t.Fatalf("want nil, got %v", got)
	}
	if got := ChunkText("   \n\n\n  ", Options{}); got != nil {
		t.Fatalf("want nil for whitespace, got %v", got)
	}
}

func TestChunkText_trailingShardFolded(t *testing.T) {
	// One long paragraph plus a tiny trailing paragraph — the shard
	// should merge into the previous chunk instead of standing alone.
	big := strings.Repeat("word ", 500)
	tiny := "postscript."
	text := big + "\n\n" + tiny
	got := ChunkText(text, Options{MaxRunes: 1500, OverlapRunes: 200, MinRunes: 200})
	last := got[len(got)-1]
	if !strings.Contains(last.Text, "postscript.") {
		t.Errorf("trailing shard not folded; last chunk=%q", last.Text)
	}
}
