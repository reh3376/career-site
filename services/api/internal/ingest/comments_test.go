package ingest

import (
	"strings"
	"testing"
)

// The leak these tests exist to prevent: a scrubbed corpus copy
// carries an HTML comment saying what was removed, and that note was
// chunked, embedded, retrievable, and used as the document title.
func TestStripHTMLComments(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "the real case: multi-line scrubbing note above the heading",
			in: "<!-- Scrubbed copy for corpus ingestion.\n" +
				"Three passages were removed: the ownership structure,\n" +
				"a vendor named in a procurement disagreement, and a\n" +
				"statement that a former employer's customers lost trust. -->\n" +
				"\n# General Interview Prep\n\nBody text.\n",
			want: "# General Interview Prep\n\nBody text.",
		},
		{
			name: "single line comment",
			in:   "<!-- note -->\n# Title\n",
			want: "# Title",
		},
		{
			name: "comment at end of a content line keeps the content",
			in:   "Revenue grew 40%. <!-- source: internal deck -->\n",
			want: "Revenue grew 40%.",
		},
		{
			name: "two comments on one line",
			in:   "a <!-- x --> b <!-- y --> c\n",
			want: "a  b  c",
		},
		{
			name: "no comments is unchanged",
			in:   "# Title\n\nPlain body.",
			want: "# Title\n\nPlain body.",
		},
		{
			name: "comment inside a fenced block survives, because READMEs show examples",
			in:   "# Title\n\n```html\n<!-- this is how you comment -->\n```\n\nAfter.",
			want: "# Title\n\n```html\n<!-- this is how you comment -->\n```\n\nAfter.",
		},
		{
			name: "tilde fences too",
			in:   "~~~\n<!-- kept -->\n~~~",
			want: "~~~\n<!-- kept -->\n~~~",
		},
		{
			name: "a comment outside a fence is stripped even when the doc also has fences",
			in:   "<!-- gone -->\n# T\n\n```\n<!-- kept -->\n```",
			want: "# T\n\n```\n<!-- kept -->\n```",
		},
		{
			name: "unterminated comment strips to the end rather than leaking the tail",
			in:   "# Title\n\n<!-- oops forgot to close\nsecret passage here\nmore secrets",
			want: "# Title",
		},
		{
			name: "a fence marker inside a comment is not a fence",
			in:   "<!--\n```\nstill commented\n```\n-->\nAfter.",
			want: "After.",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := stripHTMLComments(tc.in); got != tc.want {
				t.Errorf("stripHTMLComments()\n got: %q\nwant: %q", got, tc.want)
			}
		})
	}
}

// The title is derived from the body, so a stripped comment must not
// be able to become one. This is the half of the defect that would
// have shown up in a citation label on a member's screen.
func TestTitleIsNotTakenFromAComment(t *testing.T) {
	body := stripHTMLComments("<!-- Scrubbed copy for corpus ingestion. The original,\n" +
		"reh-interview-prep-general.md, is unchanged. -->\n\n" +
		"# General Interview Prep — Roger E. Henley II\n\nBody.")

	got := firstNonEmptyLine(body)
	want := "General Interview Prep — Roger E. Henley II"
	if got != want {
		t.Errorf("firstNonEmptyLine() = %q, want %q", got, want)
	}
	if strings.Contains(got, "<!--") || strings.Contains(got, "Scrubbed") {
		t.Errorf("title still carries the scrubbing note: %q", got)
	}
}

// A document that is nothing but a comment must fail loudly rather
// than ingest as an empty shell.
func TestCommentOnlyDocumentIsEmpty(t *testing.T) {
	if got := stripHTMLComments("<!-- just a note -->"); got != "" {
		t.Errorf("want empty so IngestText returns ErrEmpty, got %q", got)
	}
}
