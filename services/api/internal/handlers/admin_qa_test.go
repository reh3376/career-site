package handlers

import (
	"os"
	"strings"
	"testing"

	v1 "github.com/reh3376/career-site/services/api/gen/career/v1"
)

// The bank's promise is that an entry is served word for word. A field
// silently cut to fit would be served in its cut form, which breaks
// that promise in the one way nobody would notice until a visitor read
// half a sentence.
func TestOverlongFieldsAreRefusedNotTruncated(t *testing.T) {
	long := strings.Repeat("x", 9000)
	if _, err := qaEntryFromRequest("q", long, nil, nil, false, true); err == nil {
		t.Error("an over-long answer was accepted")
	}
	if _, err := qaEntryFromRequest(strings.Repeat("q", 600), "a", nil, nil, false, true); err == nil {
		t.Error("an over-long question was accepted")
	}
}

func TestEmptyFieldsAreRefused(t *testing.T) {
	if _, err := qaEntryFromRequest("   ", "an answer", nil, nil, false, true); err == nil {
		t.Error("an empty question was accepted")
	}
	if _, err := qaEntryFromRequest("a question", "  ", nil, nil, false, true); err == nil {
		t.Error("an empty answer was accepted")
	}
}

// A source on a bank answer is a link to this site's own page. An
// absolute URL in that field would render as a site path and send the
// reader somewhere else, which is the shape of an open redirect built
// out of a content field.
func TestASourcePathMustPointAtThisSite(t *testing.T) {
	for _, bad := range []string{
		"https://example.test/page",
		"//example.test/page",
		`/\example.test`,
		"javascript:alert(1)",
		"about",
	} {
		_, err := qaEntryFromRequest("q", "a",
			[]*v1.QaSource{{Title: "T", Path: bad}}, nil, false, true)
		if err == nil {
			t.Errorf("source path %q was accepted", bad)
		}
	}
	got, err := qaEntryFromRequest("q", "a",
		[]*v1.QaSource{{Title: "About", Path: "/about"}}, nil, false, true)
	if err != nil {
		t.Fatalf("a site path was refused: %v", err)
	}
	if len(got.Sources) != 1 || got.Sources[0].Path != "/about" {
		t.Errorf("sources = %+v", got.Sources)
	}
}

// A source with a title and no link is how a private-corpus source is
// shown (FR-CHAT-04), so it has to survive.
func TestASourceMayHaveATitleAndNoLink(t *testing.T) {
	got, err := qaEntryFromRequest("q", "a",
		[]*v1.QaSource{{Title: "An unpublished note"}, {Title: "", Path: ""}}, nil, false, true)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if len(got.Sources) != 1 {
		t.Fatalf("sources = %d, want the empty one dropped and the titled one kept: %+v",
			len(got.Sources), got.Sources)
	}
	if got.Sources[0].Title != "An unpublished note" || got.Sources[0].Path != "" {
		t.Errorf("source = %+v", got.Sources[0])
	}
}

func TestTagsAreNormalisedAndBounded(t *testing.T) {
	var many []string
	for i := 0; i < 30; i++ {
		many = append(many, "Tag")
	}
	got, err := qaEntryFromRequest("q", "a", nil, append([]string{" Logistics ", "", "  "}, many...), false, true)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if len(got.Tags) > qaTagsMax {
		t.Errorf("tags = %d, over the cap of %d", len(got.Tags), qaTagsMax)
	}
	if got.Tags[0] != "logistics" {
		t.Errorf("tags[0] = %q, want trimmed and lowercased", got.Tags[0])
	}
}

// covers_restricted and enabled are the two flags with consequences:
// together they are the owner deciding the assistant may speak about
// compensation or references. They must survive exactly as sent.
func TestTheConsequentialFlagsAreCarriedThrough(t *testing.T) {
	got, err := qaEntryFromRequest("what are your salary expectations?",
		"Better discussed directly.", nil, nil, true, true)
	if err != nil {
		t.Fatalf("refused: %v", err)
	}
	if !got.CoversRestricted || !got.Enabled {
		t.Errorf("flags lost: covers_restricted=%v enabled=%v", got.CoversRestricted, got.Enabled)
	}
}

// Every public content directory must be reachable by the reindex.
//
// The public reindex walks publicCorpusSubdirs rather than the
// directory listing, so a folder that is not named there is never read.
// The site guide was added under content/other, reindexed, and silently
// ignored: the job reported "kinds article" and ingested nothing, and
// the assistant still could not answer questions about the site.
//
// This walks the committed content tree and fails on any directory that
// has no entry, so the next one is caught before it is deployed.
func TestEveryPublicContentDirectoryIsReindexable(t *testing.T) {
	const root = "../../../../apps/web/content"
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("content tree not present: %v", err)
	}
	mapped := map[string]bool{}
	for _, dir := range publicCorpusSubdirs {
		mapped[dir] = true
	}
	for _, e := range entries {
		if !e.IsDir() || mapped[e.Name()] {
			continue
		}
		// photos holds images, which the walker does not read.
		if e.Name() == "photos" {
			continue
		}
		t.Errorf("apps/web/content/%s has no entry in publicCorpusSubdirs, "+
			"so the public reindex will never read it", e.Name())
	}
}
