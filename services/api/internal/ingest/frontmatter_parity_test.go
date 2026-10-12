package ingest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// One fixture, four parsers. See testdata/frontmatter/README.md.
//
// This package's parseFrontMatter carried the comment "the same shape
// the TypeScript articles library expects (kept in sync by convention)".
// They had drifted. This is the mechanism that replaces the convention;
// the TypeScript half of it is apps/web/src/lib/frontmatter-parity.test.ts
// and reads the same two files.

func fixtureDir(t *testing.T) string {
	t.Helper()
	// internal/ingest -> services/api -> repo root
	dir, err := filepath.Abs(filepath.Join("..", "..", "..", "..", "testdata", "frontmatter"))
	if err != nil {
		t.Fatalf("resolve the fixture directory: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "fixture.md")); err != nil {
		t.Fatalf("fixture not found at %s: %v", dir, err)
	}
	return dir
}

func TestFrontMatterMatchesTheSharedFixture(t *testing.T) {
	dir := fixtureDir(t)

	raw, err := os.ReadFile(filepath.Join(dir, "fixture.md"))
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	wantJSON, err := os.ReadFile(filepath.Join(dir, "expected.json"))
	if err != nil {
		t.Fatalf("read expected: %v", err)
	}
	var want map[string]string
	if err := json.Unmarshal(wantJSON, &want); err != nil {
		t.Fatalf("parse expected.json: %v", err)
	}
	if len(want) == 0 {
		t.Fatal("expected.json is empty, so this test asserts nothing")
	}

	got, body := parseFrontMatter(string(raw))

	for k, w := range want {
		g, ok := got[k]
		if !ok {
			t.Errorf("key %q missing from the parsed map", k)
			continue
		}
		if g != w {
			t.Errorf("key %q = %q, the shared fixture says %q", k, g, w)
		}
	}
	for k := range got {
		if _, ok := want[k]; !ok {
			t.Errorf("key %q was parsed but is not in expected.json: either this "+
				"parser gained a behaviour the other three lack, or the fixture "+
				"needs updating in all four places at once", k)
		}
	}
	if !strings.HasPrefix(body, "\nBody text") {
		t.Errorf("body was not returned with the front matter stripped: %.40q", body)
	}
}

// CRLF is its own case rather than part of the fixture, because a file
// with mixed line endings does not survive a checkout reliably. All
// four parsers must treat it the same way, and articles.ts did not
// until 2026-10-12.
func TestFrontMatterNormalisesCRLF(t *testing.T) {
	raw := "---\r\ntitle: \"carriage returns\"\r\n---\r\nbody\r\n"
	got, _ := parseFrontMatter(raw)
	if got["title"] != "carriage returns" {
		t.Fatalf("title = %q, want %q", got["title"], "carriage returns")
	}
}
