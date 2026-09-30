package handlers

import (
	"strings"
	"testing"

	"github.com/reh3376/career-site/services/api/internal/users"
)

// History is what lets a follow-up like "and where was that?" resolve.
// Getting it wrong is not a crash: it is the assistant answering a
// question that was never asked, because a stray question paired itself
// with the wrong answer.
func TestTurnsFromPairsCompleteExchangesOnly(t *testing.T) {
	msgs := []users.ChatMessage{
		{Role: "user", Text: "what did you do at Joy Global?"},
		{Role: "assistant", Text: "I ran the controls programme."},
		{Role: "user", Text: "for how long?"},
		{Role: "assistant", Text: "About ten years."},
		// The question just stored, with no answer yet. It is already
		// the prompt's <question>; carrying it in history as well would
		// show it to the model twice.
		{Role: "user", Text: "and where was that?"},
	}
	turns := turnsFrom(msgs)
	if len(turns) != 2 {
		t.Fatalf("turns = %d, want 2 complete exchanges: %+v", len(turns), turns)
	}
	if turns[0].Question != "what did you do at Joy Global?" || turns[0].Answer != "I ran the controls programme." {
		t.Errorf("first turn mispaired: %+v", turns[0])
	}
	if turns[1].Question != "for how long?" || turns[1].Answer != "About ten years." {
		t.Errorf("second turn mispaired: %+v", turns[1])
	}
}

// An answer with no question before it must not attach itself to an
// older question. The disclosure and an owner's escalation reply both
// arrive that way.
func TestTurnsFromIgnoresAnAnswerWithNoQuestion(t *testing.T) {
	msgs := []users.ChatMessage{
		{Role: "assistant", Text: "I am an AI assistant."},
		{Role: "assistant", Text: "Roger here, replying directly."},
		{Role: "user", Text: "thanks, one more thing"},
		{Role: "assistant", Text: "Go on."},
	}
	turns := turnsFrom(msgs)
	if len(turns) != 1 {
		t.Fatalf("turns = %d, want 1: %+v", len(turns), turns)
	}
	if turns[0].Question != "thanks, one more thing" {
		t.Errorf("wrong question kept: %q", turns[0].Question)
	}
}

// An owner reply is a real turn in the thread (FR-CHAT-10), so it
// answers the question before it.
func TestTurnsFromTreatsAnOwnerReplyAsAnAnswer(t *testing.T) {
	turns := turnsFrom([]users.ChatMessage{
		{Role: "user", Text: "can I ask you directly?"},
		{Role: "owner", Text: "Of course. Roger."},
	})
	if len(turns) != 1 || turns[0].Answer != "Of course. Roger." {
		t.Errorf("owner reply not paired: %+v", turns)
	}
}

func TestTitleFromShortensWithoutCuttingAWord(t *testing.T) {
	long := "What experience do you have building unified namespace architectures " +
		"for multi-site manufacturing operations with MQTT and Ignition?"
	got := titleFrom(long)
	if len([]rune(got)) > 74 {
		t.Errorf("title is %d runes, too long for a list: %q", len([]rune(got)), got)
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("a truncated title should say so: %q", got)
	}
	// The cut lands on a space, so the last word is whole.
	trimmed := strings.TrimSuffix(got, "...")
	if strings.HasSuffix(trimmed, " ") {
		t.Errorf("title ends on a space: %q", got)
	}
	if !strings.HasPrefix(long, trimmed) {
		t.Errorf("title is not a prefix of the question: %q", got)
	}
}

func TestTitleFromKeepsAShortQuestionWhole(t *testing.T) {
	if got := titleFrom("  What does   MDEMG do? "); got != "What does MDEMG do?" {
		t.Errorf("title = %q, want the question tidied but intact", got)
	}
}

// FR-CHAT-02: the disclosure must say it is an AI, say where answers
// come from, and link the explanation.
func TestDisclosureSaysWhatItIsAndLinksTheExplanation(t *testing.T) {
	for _, want := range []string{"AI assistant", "his own records", "/how-ask-roger-works"} {
		if !strings.Contains(DisclosureText, want) {
			t.Errorf("the disclosure no longer mentions %q: %q", want, DisclosureText)
		}
	}
	if strings.Contains(DisclosureText, "—") {
		t.Error("em dash in user-visible copy")
	}
}

func TestParseIDRejectsNonsense(t *testing.T) {
	for _, s := range []string{"", "abc", "0", "-3", "12x"} {
		if _, err := parseID(s); err == nil {
			t.Errorf("parseID(%q) accepted", s)
		}
	}
	if id, err := parseID(" 42 "); err != nil || id != 42 {
		t.Errorf("parseID(\" 42 \") = %d, %v", id, err)
	}
}
