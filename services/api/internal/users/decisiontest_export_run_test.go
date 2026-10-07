package users

import (
	"encoding/csv"
	"strings"
	"testing"
	"time"
)

// The per-run download, checked on the distinctions that are easy to
// lose in a CSV and expensive to lose in a dataset.
//
// A spreadsheet has no concept of NULL. Every one of these columns has a
// value that means "not applicable" and a value that means zero, and
// writing the sentinel as a number turns it into a data point: -1
// chosen indices get averaged, -100 memory failures get charted, and a
// blank prior count becomes a claim that a link existed and saw nothing.
func TestTheRunExportKeepsAbsenceDistinctFromZero(t *testing.T) {
	zero := 0
	two := 3
	run := DTRun{
		SessionKey:     "11111111-2222-3333-4444-555555555555",
		Status:         "completed",
		StartedAt:      time.Date(2026, 10, 7, 9, 0, 0, 0, time.UTC),
		AttemptNo:      &two,
		AttemptSource:  "cookie",
		PriorByAccount: &zero, // the link existed and saw nothing
		PriorByEmail:   nil,   // there was no such link
		BaselineRTMs:   90,
	}
	blocks := []DTBlockSummary{
		{BlockNo: 1, Load: "d3", Correct: 5, Total: 6, PresentedDigits: "695",
			MemoryFailure: 0},
		// An expired recall: nothing was attempted, which is not a
		// memory failure of any size.
		{BlockNo: 2, Load: "d4", Correct: 3, Total: 6, MemoryFailure: -1},
	}
	answers := []DTAnswerRow{
		{Position: 1, BlockNo: 1, ItemCode: "A1", Outcome: "correct",
			ChosenIndex: 0, Confidence: 90, Brier: 0.01,
			Prompt: "a question", ChosenText: "right", CorrectText: "right"},
		{Position: 2, BlockNo: 1, ItemCode: "A2", Outcome: "expired",
			ChosenIndex: -1, Brier: -1},
	}

	out, err := BuildDTRunCSV(run, blocks, answers, false)
	if err != nil {
		t.Fatalf("BuildDTRunCSV: %v", err)
	}
	if out.BlockRows != 2 || out.AnswerRows != 2 {
		t.Fatalf("counted %d blocks and %d answers, want 2 and 2",
			out.BlockRows, out.AnswerRows)
	}

	bh, brows := parseCSV(t, out.BlocksCSV)
	ah, arows := parseCSV(t, out.AnswersCSV)

	// Zero and absent must not read the same.
	if got := cell(t, bh, brows[0], "prior_by_account"); got != "0" {
		t.Errorf("prior_by_account = %q, want %q: a link that existed and saw "+
			"nothing is not a link that never existed", got, "0")
	}
	if got := cell(t, bh, brows[0], "prior_by_email"); got != "" {
		t.Errorf("prior_by_email = %q, want empty for an identity that did not exist", got)
	}

	// Chosen option one is a real answer and must not come out blank.
	if got := cell(t, ah, arows[0], "chosen_index"); got != "0" {
		t.Errorf("chosen_index = %q, want %q for the first option", got, "0")
	}
	// An expiry chose nothing, and must not read as option -1.
	if got := cell(t, ah, arows[1], "chosen_index"); got != "" {
		t.Errorf("chosen_index = %q for an expired answer, want empty: a "+
			"spreadsheet will average a -1", got)
	}
	if got := cell(t, ah, arows[1], "brier"); got != "" {
		t.Errorf("brier = %q for an unscorable answer, want empty", got)
	}

	// A recall that expired is not a 100%% memory failure.
	if got := cell(t, bh, brows[1], "memory_failure_pct"); got != "" {
		t.Errorf("memory_failure_pct = %q for an expired recall, want empty", got)
	}
	if got := cell(t, bh, brows[0], "memory_failure_pct"); got != "0" {
		t.Errorf("memory_failure_pct = %q for a number held intact, want %q", got, "0")
	}

	// The key is withheld unless asked for.
	if strings.Contains(out.AnswersCSV, "a question") {
		t.Error("the prompt is in the file although include_key was false")
	}
	if contains(ah, "correct_text") {
		t.Error("correct_text is a column although include_key was false")
	}

	withKey, err := BuildDTRunCSV(run, blocks, answers, true)
	if err != nil {
		t.Fatalf("BuildDTRunCSV(includeKey): %v", err)
	}
	if !strings.Contains(withKey.AnswersCSV, "a question") {
		t.Error("the prompt is missing although include_key was true")
	}

	// Every row carries the run, or the file cannot be read on its own.
	for i, r := range arows {
		if cell(t, ah, r, "session_key") != run.SessionKey {
			t.Errorf("answer row %d does not carry the session key", i)
		}
	}
}

func parseCSV(t *testing.T, s string) ([]string, [][]string) {
	t.Helper()
	recs, err := csv.NewReader(strings.NewReader(s)).ReadAll()
	if err != nil {
		t.Fatalf("the export is not valid CSV: %v", err)
	}
	if len(recs) < 2 {
		t.Fatalf("the export has %d records, want a header and at least one row", len(recs))
	}
	return recs[0], recs[1:]
}

func cell(t *testing.T, header, row []string, name string) string {
	t.Helper()
	for i, h := range header {
		if h == name {
			if i >= len(row) {
				t.Fatalf("row is shorter than the header at %q", name)
			}
			return row[i]
		}
	}
	t.Fatalf("no column %q in the export", name)
	return ""
}

func contains(header []string, name string) bool {
	for _, h := range header {
		if h == name {
			return true
		}
	}
	return false
}
