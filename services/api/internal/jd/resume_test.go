package jd

import (
	"strings"
	"testing"
)

func TestRenderMarkdown_structureAndNoEmDash(t *testing.T) {
	r := &Resume{
		Headline: "Principal Engineer, Industrial AI",
		Summary:  "Thirty years " + tidy("across control rooms — and plants."),
		Competencies: []Sourced{
			{Text: "Rockwell ControlLogix", Sources: []int64{1}},
		},
		Experience: []Experience{{
			Role: "Director of Engineering", Organisation: "Whiskey House", Dates: "2022 to present",
			Bullets: []Sourced{{Text: "Built the hub-and-spoke platform.", Sources: []int64{2}}},
		}},
		Education: []Sourced{{Text: "BS Applied Mathematics, WVU", Sources: []int64{3}}},
	}
	md := RenderMarkdown(r)
	for _, want := range []string{
		"# Roger E. Henley II", "## Summary", "## Core competencies",
		"### Director of Engineering, Whiskey House (2022 to present)",
		"- Built the hub-and-spoke platform.", "## Education and credentials",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown missing %q", want)
		}
	}
	if strings.Contains(md, "—") {
		t.Error("em dash leaked into rendered résumé")
	}
}

func TestTidy_replacesEmDashes(t *testing.T) {
	if got := tidy("  a — b—c &mdash; d "); got != "a, b, c ,  d" {
		t.Fatalf("got %q", got)
	}
}
