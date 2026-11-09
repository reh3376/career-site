package hiring

import (
	"slices"
	"testing"
)

func TestInferType(t *testing.T) {
	cases := []struct {
		text    string
		parties []string
		want    string
	}{
		{"Bachelor's degree in Electrical Engineering or equivalent experience", nil, "credential"},
		{"5+ years of PLC programming experience", nil, "experience"},
		{"Strong communication skills and a collaborative attitude", nil, "behavioral"},
		{"Experience with Rockwell Studio 5000 and Ignition SCADA", nil, "tool"},
		{"Background in food and beverage manufacturing", nil, "domain"},
		{"Established partnership with Rockwell Automation", []string{"Rockwell Automation"}, "relationship"},
		{"Ability to travel up to 25%", nil, "constraint"},
		{"Track record of delivering capital projects on budget", nil, "outcome"},
		{"Root-cause analysis of process upsets", nil, "skill"},
		// A credential that also names years is a credential.
		{"Master's degree and 3 years in the field", nil, "credential"},
		// A behavioral adjective wins over everything: it must never be judged.
		{"Self-starter with 10 years of SQL", nil, "behavioral"},
	}
	for _, c := range cases {
		if got := InferType(c.text, c.parties); got != c.want {
			t.Errorf("InferType(%q) = %q, want %q", c.text, got, c.want)
		}
	}
}

// Every value the heuristic can return must be a registered
// requirement_type, or the writer would refuse it and fall back to
// skill, silently.
func TestInferTypeReturnsOnlyRegisteredCodes(t *testing.T) {
	texts := []string{
		"degree", "5 years", "team player", "python", "manufacturing", "partnership with", "travel",
		"delivered", "anything else at all", "",
	}
	for _, s := range texts {
		got := InferType(s, nil)
		if !slices.Contains(RequirementTypes, got) {
			t.Errorf("InferType(%q) = %q, which is not in vocabulary 1.0", s, got)
		}
	}
	if got := InferType("x", []string{"Acme"}); !slices.Contains(RequirementTypes, got) {
		t.Errorf("InferType with parties = %q, not registered", got)
	}
}
