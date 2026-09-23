package jd

import (
	"strings"
	"testing"
)

// The length guard exists to save a model call on input that obviously
// describes no role. It must never be the thing that refuses a real
// posting, so these cases are about where the line sits.
func TestTooShortToBeAPosting(t *testing.T) {
	t.Parallel()

	t.Run("an empty box", func(t *testing.T) {
		t.Parallel()
		if _, short := tooShortToBeAPosting(""); !short {
			t.Fatal("empty text should not reach the model")
		}
	})

	t.Run("a pasted URL", func(t *testing.T) {
		t.Parallel()
		if _, short := tooShortToBeAPosting("https://example.com/careers/12345"); !short {
			t.Fatal("a bare link describes no role")
		}
	})

	// The submission that prompted the whole check: 365 characters of a
	// job-search worksheet. It is over the guard, on purpose. The guard
	// is not meant to catch this; the model is, because the difference
	// between this and a short posting is meaning, not length.
	t.Run("the worksheet that started this goes to the model", func(t *testing.T) {
		t.Parallel()
		worksheet := "4. Boeing — Search & Apply\n    Search: Artificial Intelligence, Machine Learning, " +
			"Generative AI, Autonomy, Data Science.\n5. Northrop Grumman — Search & Apply\n    " +
			"Search: AI/ML, Machine Learning, Autonomy, Artificial Intelligence, Principal Software Engineer."
		if _, short := tooShortToBeAPosting(worksheet); short {
			t.Fatalf("length %d should pass the guard and be judged on meaning", len([]rune(worksheet)))
		}
	})

	// The expensive mistake would be refusing a genuine posting that
	// happens to be terse, so the threshold must sit below one.
	t.Run("a short but real posting passes", func(t *testing.T) {
		t.Parallel()
		posting := "Controls Engineer, Louisville KY. You will program and commission PLCs and " +
			"SCADA for a 24/7 bottling line, support start-ups, and troubleshoot on the floor. " +
			"Requires 5 years in food and beverage manufacturing and an engineering degree."
		if _, short := tooShortToBeAPosting(posting); short {
			t.Fatalf("a real posting of %d runes was refused by the guard", len([]rune(posting)))
		}
	})

	t.Run("whitespace does not pad a fragment past the guard", func(t *testing.T) {
		t.Parallel()
		padded := strings.TrimSpace("   AI/ML roles   " + strings.Repeat(" ", 400))
		if _, short := tooShortToBeAPosting(padded); !short {
			t.Fatal("spaces are not content")
		}
	})
}

func TestNotAPostingMessage(t *testing.T) {
	t.Parallel()

	got := NotAPostingMessage(PostingVerdict{
		IsPosting: false,
		Kind:      "search_terms",
		Reason:    "This looks like a list of search terms rather than a job posting.",
	})
	for _, want := range []string{"search terms", "Nothing was scored", "submit again"} {
		if !strings.Contains(got, want) {
			t.Fatalf("message should mention %q: %s", want, got)
		}
	}

	// A model that returns an empty reason must not produce a message
	// that says nothing at all.
	if got := NotAPostingMessage(PostingVerdict{IsPosting: false}); !strings.Contains(got, "does not look like a job posting") {
		t.Fatalf("empty reason should still explain itself: %s", got)
	}
}
