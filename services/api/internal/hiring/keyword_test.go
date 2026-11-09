package hiring

import (
	"reflect"
	"testing"
)

func TestKeywordBaseline(t *testing.T) {
	cases := []struct {
		name      string
		req       string
		evidence  []string
		wantFound bool
		wantTerms []string
	}{
		{
			name:      "two stems match across inflections",
			req:       "Experience managing PLC programming projects",
			evidence:  []string{"Managed the PLC program for three plants."},
			wantFound: true,
			wantTerms: []string{"manag", "plc"},
		},
		{
			name:      "one of several terms is not enough",
			req:       "Bachelor's degree in electrical engineering",
			evidence:  []string{"Led the engineering team."},
			wantFound: false,
			wantTerms: []string{"engineer"},
		},
		{
			name:      "the lexical gap: same work, other words",
			req:       "Front-end development on capital projects",
			evidence:  []string{"Owned FEL-2 and FEL-3 for a $60M expansion."},
			wantFound: false,
			wantTerms: nil,
		},
		{
			name:      "a one-term requirement needs that one term",
			req:       "Ignition",
			evidence:  []string{"Built the Ignition SCADA layer."},
			wantFound: true,
			wantTerms: []string{"ignition"},
		},
		{
			name:      "no content terms, nothing found",
			req:       "Must have the ability to",
			evidence:  []string{"anything"},
			wantFound: false,
			wantTerms: nil,
		},
	}
	for _, c := range cases {
		found, terms := KeywordBaseline(c.req, c.evidence)
		if found != c.wantFound {
			t.Errorf("%s: found = %v, want %v (terms %v)", c.name, found, c.wantFound, terms)
		}
		if !reflect.DeepEqual(terms, c.wantTerms) {
			t.Errorf("%s: terms = %v, want %v", c.name, terms, c.wantTerms)
		}
	}
}

func TestStem(t *testing.T) {
	for in, want := range map[string]string{
		"managing": "manag", "managed": "manag", "manager": "manag", "management": "manag",
		"systems": "system", "utilities": "util", "utility": "util", "programming": "programm", "designed": "design",
		"plc": "plc", "scada": "scada",
	} {
		if got := stem(in); got != want {
			t.Errorf("stem(%q) = %q, want %q", in, got, want)
		}
	}
}
