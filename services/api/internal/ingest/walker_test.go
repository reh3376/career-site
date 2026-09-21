package ingest

import (
	"testing"

	"github.com/reh3376/career-site/services/api/internal/users"
)

func TestParseFrontMatter_extractsKeysAndStripsBlock(t *testing.T) {
	raw := "---\ntitle: \"Emergence by Design\"\nvisibility: corpus_only\ndate: 2026-09-21\n---\n# Heading\n\nBody text.\n"
	meta, body := parseFrontMatter(raw)
	if meta["title"] != "Emergence by Design" {
		t.Fatalf("title = %q", meta["title"])
	}
	if meta["visibility"] != "corpus_only" {
		t.Fatalf("visibility = %q", meta["visibility"])
	}
	if body != "# Heading\n\nBody text.\n" {
		t.Fatalf("body = %q", body)
	}
}

func TestParseFrontMatter_noBlockIsIdentity(t *testing.T) {
	raw := "# Just a heading\n\nText.\n"
	meta, body := parseFrontMatter(raw)
	if len(meta) != 0 || body != raw {
		t.Fatalf("expected identity, got meta=%v body=%q", meta, body)
	}
}

func TestParseFrontMatter_unterminatedBlockIsIdentity(t *testing.T) {
	raw := "---\ntitle: x\nno terminator here\n"
	meta, body := parseFrontMatter(raw)
	if len(meta) != 0 || body != raw {
		t.Fatalf("expected identity, got meta=%v body=%q", meta, body)
	}
}

func TestEffectiveVisibility_privateNeverLoosens(t *testing.T) {
	got := effectiveVisibility(users.VisibilityCorpusOnly, "public")
	if got != users.VisibilityCorpusOnly {
		t.Fatalf("private root must stay corpus_only, got %q", got)
	}
}

func TestEffectiveVisibility_publicCanTighten(t *testing.T) {
	if got := effectiveVisibility(users.VisibilityPublic, "corpus_only"); got != users.VisibilityCorpusOnly {
		t.Fatalf("expected corpus_only, got %q", got)
	}
	if got := effectiveVisibility(users.VisibilityPublic, ""); got != users.VisibilityPublic {
		t.Fatalf("expected public, got %q", got)
	}
	if got := effectiveVisibility(users.VisibilityPublic, "bogus"); got != users.VisibilityPublic {
		t.Fatalf("unknown override must be ignored, got %q", got)
	}
}

func TestIsTextSource(t *testing.T) {
	for name, want := range map[string]bool{
		"notes.md": true, "NOTES.MD": true, "a.txt": true,
		"deck.pptx": false, "scan.pdf": false, "README": false,
	} {
		if got := isTextSource(name); got != want {
			t.Errorf("%s: got %v want %v", name, got, want)
		}
	}
}

func TestKnownKinds_containsCoreSet(t *testing.T) {
	for _, k := range []string{"article", "talk", "worksheet", "post_mortem", "interview_prep", "resume"} {
		if !IsKnownKind(k) {
			t.Errorf("%s should be a known kind", k)
		}
	}
	if IsKnownKind("drawings") {
		t.Error("drawings must not be a known kind")
	}
}
