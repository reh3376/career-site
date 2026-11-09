package hiring

import "testing"

func TestRegistryResolve(t *testing.T) {
	r := NewRegistry([]Code{
		{Vocab: VocabBasis, Code: "direct", IntroducedIn: "1.0", SortOrder: 1},
		{Vocab: VocabBasis, Code: "vocabulary", IntroducedIn: "1.0", SortOrder: 2},
		{Vocab: VocabBasis, Code: "newer", IntroducedIn: "1.2", SortOrder: 3},
		{Vocab: VocabBasis, Code: "gone", IntroducedIn: "1.0", RetiredIn: "1.1", ReplacedBy: "vocabulary", SortOrder: 4},
		{Vocab: "", Code: "ignored"},
	})
	cases := []struct {
		code, at string
		want     string
		known    bool
	}{
		{"direct", "1.0", "direct", true},
		{"newer", "1.0", Other, false}, // introduced later than the row's vocabulary
		{"newer", "1.2", "newer", true},
		{"gone", "1.0", "gone", true},    // live at 1.0
		{"gone", "1.1", Other, false},    // retired at 1.1
		{"unheard", "1.0", Other, false}, // never existed
	}
	for _, c := range cases {
		got, known := r.Resolve(VocabBasis, c.code, c.at)
		if got != c.want || known != c.known {
			t.Errorf("Resolve(%q at %s) = %q,%v; want %q,%v", c.code, c.at, got, known, c.want, c.known)
		}
	}
	if !r.Known(VocabBasis, "gone") || r.Known(VocabBasis, "unheard") || r.Known("other_vocab", "direct") {
		t.Error("Known: a retired code is still known; an unheard one or another vocabulary is not")
	}
	live := r.Codes(VocabBasis)
	if len(live) != 3 || live[0].Code != "direct" || live[2].Code != "newer" {
		t.Errorf("Codes: want direct, vocabulary, newer in order; got %+v", live)
	}
	var nilReg *Registry
	if got, ok := nilReg.Resolve(VocabBasis, "direct", "1.0"); got != Other || ok {
		t.Error("a nil registry must resolve everything to other")
	}
}
