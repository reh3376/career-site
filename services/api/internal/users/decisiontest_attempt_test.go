package users

import "testing"

func ptr(n int) *int { return &n }

// Three identities can disagree about how many prior sittings exist,
// and combining them is where a labelling variable acquires a bias.
//
// The owner rejected an earlier version that stored a single number
// taken from whichever source reported the most: "we dont get to cherry
// pick data sets in that manner". He was right for a sharper reason
// than I gave at the time. It collapsed three observations into one at
// write time and discarded the components, and taking the maximum
// inflates the figure whenever the sources disagree, which is not
// random noise: it tracks how much identity a participant handed over,
// so people who gave an email would read as repeaters more often than
// people who did not.
//
// What survives is a yes-or-no summary, and an OR is the complete
// answer to a yes-or-no question rather than a choice among numbers.

func TestIsRepeatIsAnOrAcrossSourcesNotASelection(t *testing.T) {
	tests := []struct {
		name                   string
		account, email, cookie *int
		wantRepeat             bool
		wantSource             string
	}{
		{
			name:       "nothing to ask: not a repeat, and no source for the claim",
			wantRepeat: false, wantSource: "",
		},
		{
			name:    "every source agrees there is no prior sitting",
			account: ptr(0), email: ptr(0), cookie: ptr(0),
			wantRepeat: false, wantSource: "",
		},
		{
			// The case a single number would have hidden. Somebody sat
			// it anonymously and later signed up: the account is new,
			// the browser is not. A prior sitting demonstrably exists,
			// so the honest answer to "is this a repeat" is yes.
			name:    "account says no, cookie says yes: still a repeat",
			account: ptr(0), cookie: ptr(1),
			wantRepeat: true, wantSource: "cookie",
		},
		{
			// The reverse: cleared storage, same account.
			name:    "cookie says no, account says yes: still a repeat",
			account: ptr(2), cookie: ptr(0),
			wantRepeat: true, wantSource: "account",
		},
		{
			// Both saw one. The source names the better evidence rather
			// than whichever was checked last.
			name:    "both agree: the more reliable source is named",
			account: ptr(1), email: ptr(1), cookie: ptr(3),
			wantRepeat: true, wantSource: "account",
		},
		{
			name:       "only a cookie exists and it saw one",
			cookie:     ptr(4),
			wantRepeat: true, wantSource: "cookie",
		},
		{
			// An email with no account, which is the commonest shape for
			// a volunteer who wants their results.
			name:  "email only",
			email: ptr(1), cookie: ptr(0),
			wantRepeat: true, wantSource: "email",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotRepeat, gotSource := dtAnyPrior(tc.account, tc.email, tc.cookie)
			if gotRepeat != tc.wantRepeat {
				t.Errorf("dtAnyPrior() repeat = %v, want %v", gotRepeat, tc.wantRepeat)
			}
			if gotSource != tc.wantSource {
				t.Errorf("dtAnyPrior() source = %q, want %q", gotSource, tc.wantSource)
			}
		})
	}
}

// nil and zero are different facts and the code must never conflate
// them. Zero is an identity reporting no prior sitting, which is
// positive evidence of a first attempt. nil is that identity not
// existing, or a failed count, which is silence. A reader who treats
// silence as evidence of a first attempt will undercount repeats among
// exactly the participants who gave the least identity away.
func TestNilIsNotZero(t *testing.T) {
	// All nil: no evidence either way, and no source to cite.
	if repeat, src := dtAnyPrior(nil, nil, nil); repeat || src != "" {
		t.Errorf("no sources gave repeat=%v source=%q, want false and empty", repeat, src)
	}
	// All zero: evidence of a first sitting, still not a repeat, but the
	// difference from the case above is real and lives in the stored
	// counts rather than in this boolean.
	if repeat, _ := dtAnyPrior(ptr(0), ptr(0), ptr(0)); repeat {
		t.Error("three sources reporting zero priors read as a repeat")
	}
}
