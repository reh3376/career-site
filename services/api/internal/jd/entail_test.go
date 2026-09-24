package jd

import "testing"

// The property that matters most is the one that costs a true line when
// it is wrong. These tests are weighted accordingly: the cases proving
// a good line survives outnumber the cases proving a bad one is caught.

func TestNumberNotInSourceIsUnsupported(t *testing.T) {
	line := "Cut unplanned downtime by 40% across three plants"
	src := "Led reliability work across the plants, reducing unplanned downtime materially."
	got := checkSupport(line, src)
	if len(got.Numbers) != 1 || got.Numbers[0] != "40" {
		t.Fatalf("expected 40 to be unsupported, got %+v", got)
	}
}

func TestNumberInSourceIsSupported(t *testing.T) {
	line := "Cut unplanned downtime by 40% across the fleet"
	src := "The programme cut unplanned downtime 40% over two years."
	if got := checkSupport(line, src); got.any() {
		t.Fatalf("expected no issues, got %+v", got)
	}
}

func TestThousandsSeparatorIsTheSameClaim(t *testing.T) {
	line := "Brought 1,200 tags under management"
	src := "Migrated 1200 tags into the historian."
	if got := checkSupport(line, src); got.any() {
		t.Fatalf("1,200 and 1200 are one claim, got %+v", got)
	}
}

func TestVersionDigitsAreNotClaims(t *testing.T) {
	// A part number is not a quantity. If these registered as claims,
	// every line naming real hardware would be dropped.
	line := "Commissioned S7-1500 PLCs and ControlLogix racks"
	src := "Commissioned Siemens and Rockwell controllers on the line."
	got := checkSupport(line, src)
	for _, n := range got.Numbers {
		if n == "7" || n == "1500" {
			t.Fatalf("part numbers must not count as claims, got %+v", got)
		}
	}
}

func TestOrganisationNotInSourceIsUnsupported(t *testing.T) {
	line := "Delivered the control system for Blue Origin"
	src := "Delivered control systems across mining and chemical sites."
	got := checkSupport(line, src)
	if len(got.Parties) == 0 {
		t.Fatalf("expected the named organisation to be unsupported, got %+v", got)
	}
}

func TestAcronymMatchesSpelledOutName(t *testing.T) {
	// The relationship rule already established that these are one
	// employer. The entailment check must not contradict it.
	line := "Ran the AWS migration"
	src := "Ran the Amazon Web Services migration for the plant historian."
	for _, p := range checkSupport(line, src).Parties {
		if p == "AWS" {
			t.Fatalf("AWS is supported by Amazon Web Services, got %+v", p)
		}
	}
}

func TestSpelledOutNameMatchesAcronymInSource(t *testing.T) {
	line := "Ran the Amazon Web Services migration"
	src := "Ran the AWS migration for the plant historian."
	for _, p := range checkSupport(line, src).Parties {
		if p == "Amazon Web Services" {
			t.Fatalf("the spelled-out name is supported by AWS in the source, got %+v", p)
		}
	}
}

func TestJobTitlesAreNotOrganisations(t *testing.T) {
	// The commonest false positive: a line that starts with a
	// capitalised verb or carries a title. None of this is a company.
	for _, line := range []string{
		"Led process control and automation for the division",
		"Director of Process Control reporting to the plant manager",
		"Designed Automation Systems for continuous casting",
	} {
		if got := checkSupport(line, "worked on control systems"); len(got.Parties) > 0 {
			t.Fatalf("%q produced a false organisation: %+v", line, got.Parties)
		}
	}
}

func TestProseWithoutSpecificsIsLeftAlone(t *testing.T) {
	line := "Led the team through a difficult migration and kept the plant running"
	if got := checkSupport(line, "ran plant systems"); got.any() {
		t.Fatalf("unfalsifiable prose must pass, got %+v", got)
	}
}

func TestEmptySourceReportsNothing(t *testing.T) {
	// A line with no usable sources is already refused upstream. Saying
	// everything is unsupported here would blur the two cases.
	if got := checkSupport("Cut downtime 40% at Acme Steel", ""); got.any() {
		t.Fatalf("an empty source reports nothing, got %+v", got)
	}
}
