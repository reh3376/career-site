package jd

import (
	"strings"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/prompts"
)

// The posting wording that started this. Extraction compressed it to
// "Develop front-end development for major investments", the judge read
// that as web development, and a capital projects requirement was
// scored against the wrong field entirely.
const postingFELParagraph = `Responsibilities include the front-end development of future major
investments, including scope definition, alternatives analysis, and
basis of design, working with operations and maintenance to establish
project success criteria.`

func TestQuoteAppearsIn(t *testing.T) {
	cases := []struct {
		name  string
		quote string
		want  bool
	}{
		{
			name:  "verbatim span is accepted",
			quote: "front-end development of future major investments, including scope definition",
			want:  true,
		},
		{
			name:  "re-wrapped span is accepted, because only whitespace changed",
			quote: "front-end development of future major\n  investments,   including scope definition",
			want:  true,
		},
		{
			name:  "case differences are forgiven",
			quote: "FRONT-END DEVELOPMENT OF FUTURE MAJOR INVESTMENTS",
			want:  true,
		},
		{
			name: "a plausible paraphrase is rejected: this is the failure the quote exists to prevent, " +
				"so accepting a composed quote would defeat the point",
			quote: "front-end development for major investments",
			want:  false,
		},
		{
			name:  "phrases that are apart in the posting must not be stitched together",
			quote: "front-end development of future major investments, working with operations",
			want:  false,
		},
		{
			name:  "wholly invented text is rejected",
			quote: "five years of React and TypeScript experience",
			want:  false,
		},
		{
			name:  "empty is rejected rather than trivially matching",
			quote: "",
			want:  false,
		},
		{
			name:  "whitespace only is rejected",
			quote: "   \n\t ",
			want:  false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := quoteAppearsIn(tc.quote, postingFELParagraph); got != tc.want {
				t.Fatalf("quoteAppearsIn(%q) = %v, want %v", tc.quote, got, tc.want)
			}
		})
	}
}

func TestValidateRequirementsDropsUnverifiedQuote(t *testing.T) {
	in := []prompts.Requirement{
		{
			ID:          "r1",
			Text:        "Develop front-end development for major investments",
			SourceQuote: "front-end development of future major investments, including scope definition",
			Category:    "must",
			Weight:      3,
		},
		{
			ID:          "r2",
			Text:        "Establish project success criteria with operations",
			SourceQuote: "a quote the model composed rather than copied",
			Category:    "must",
			Weight:      2,
		},
		{
			ID:       "r3",
			Text:     "No quote offered at all",
			Category: "nice",
			Weight:   1,
		},
	}

	out, err := validateRequirements(in, postingFELParagraph)
	if err != nil {
		t.Fatalf("validateRequirements: %v", err)
	}
	if len(out) != 3 {
		t.Fatalf("got %d requirements, want 3", len(out))
	}

	if out[0].SourceQuote == "" {
		t.Error("r1: a quote that is genuinely in the posting was dropped")
	}
	if out[1].SourceQuote != "" {
		t.Errorf("r2: an invented quote survived validation: %q", out[1].SourceQuote)
	}
	if out[2].SourceQuote != "" {
		t.Errorf("r3: a quote appeared from nowhere: %q", out[2].SourceQuote)
	}

	// Dropping the quote must not drop the requirement. A bad quote
	// costs the judge the posting's wording, which is where it started;
	// losing the requirement would cost it the whole ask.
	for i, r := range out {
		if r.Text == "" {
			t.Errorf("requirement %d lost its text", i)
		}
	}
}

// The judge prompt must actually show the quote, or verifying it
// achieves nothing.
func TestRenderJudgeUserIncludesVerifiedQuote(t *testing.T) {
	reqs := []prompts.Requirement{{
		ID:          "r1",
		Text:        "Develop front-end development for major investments",
		SourceQuote: "front-end development of future major investments, including scope definition",
		Category:    "must",
		Weight:      3,
	}}
	rendered := prompts.RenderJudgeUser(reqs, nil, nil)

	if !strings.Contains(rendered, "<posting_says>") {
		t.Fatal("the judge prompt does not carry the posting's own words")
	}
	if !strings.Contains(rendered, "including scope definition") {
		t.Error("the disambiguating words were not rendered")
	}

	// And a requirement without a verified quote must render no element
	// at all, rather than an empty one that reads as "the posting said
	// nothing".
	reqs[0].SourceQuote = ""
	if strings.Contains(prompts.RenderJudgeUser(reqs, nil, nil), "<posting_says>") {
		t.Error("an empty posting_says element was rendered")
	}
}
