package users

import (
	"bytes"
	"context"
	"encoding/csv"
	"fmt"
	"strconv"
	"time"
)

// One run, as two CSV files: its blocks and its answers.
//
// Asked for by the owner on 2026-10-07: from a run's own page, download
// what the "By block" and "Every answer" tables show, plus anything else
// belonging to that run.
//
// **Built from exactly what the page was given.** These take the same
// run, blocks and answers that DTGetRun returned and that the page
// rendered, rather than running their own queries. A second query would
// be a second definition, and the one guarantee worth having here is
// that the file and the screen cannot disagree: a reviewer who downloads
// a run and finds different numbers in it has no reason to trust either.
//
// Each file carries the session context on every row. A block CSV that
// says "block 3, 4 correct" and nothing else cannot be joined back to
// anything or read a month later, and these are files that will sit in
// a folder away from the console that produced them.

// DTRunCSV is the pair of files, with the counts for the UI to report.
type DTRunCSV struct {
	FilenameStem string
	BlocksCSV    string
	AnswersCSV   string
	BlockRows    int
	AnswerRows   int
}

// dtRunContext is the session detail repeated on every row of both
// files, so each one stands alone.
func dtRunContextHeader() []string {
	return []string{
		"session_key", "started_at", "finished_at", "status", "is_synthetic",
		"attempt_no", "attempt_no_strongest", "attempt_source", "attempt_sources_disagree",
		"prior_by_account", "prior_by_email", "prior_by_cookie",
		"is_repeat", "repeat_matched_by",
		"instrument_version", "item_set_version", "key_version",
		"audio_mode", "device_class", "tap_check_passed",
		"baseline_rt_ms", "baseline_rt_sd_ms", "recall_strategy",
		"age_range", "education", "occupation", "gave_email", "wants_results",
		"session_review_status", "session_review_reason", "session_review_note",
		"reviewed_at", "reviewed_by",
	}
}

func dtRunContextRow(r DTRun) []string {
	return []string{
		r.SessionKey,
		csvValue(r.StartedAt),
		csvTime(r.FinishedAt),
		r.Status,
		csvValue(r.IsSynthetic),
		csvNullableInt(r.AttemptNo),
		csvNullableInt(r.AttemptNoStrongest),
		r.AttemptSource,
		csvValue(r.AttemptSourcesDisagree),
		csvNullableInt(r.PriorByAccount),
		csvNullableInt(r.PriorByEmail),
		csvNullableInt(r.PriorByCookie),
		csvValue(r.IsRepeat),
		r.RepeatMatchedBy,
		r.InstrumentVersion,
		r.ItemSetVersion,
		r.KeyVersion,
		r.AudioMode,
		r.DeviceClass,
		csvValue(r.TapCheckPassed),
		strconv.Itoa(r.BaselineRTMs),
		strconv.Itoa(r.BaselineRTSDMs),
		r.RecallStrategy,
		r.AgeRange,
		r.Education,
		r.Occupation,
		csvValue(r.GaveEmail),
		csvValue(r.WantsResults),
		r.ReviewStatus,
		r.ReviewReason,
		r.ReviewNote,
		csvTime(r.ReviewedAt),
		r.ReviewedByName,
	}
}

// BuildDTRunCSV renders one run's blocks and answers.
//
// includeKey adds the prompt, the chosen option and the correct option
// to the answers file. Off by default and opt-in on the page, matching
// the collapsed section there: an answer key that escapes contaminates a
// standardized instrument permanently, and a file is easier to forward
// than a screen. The whole-dataset export never includes these at all
// (FR-DT-16); this is one run, downloaded deliberately by the person who
// built the instrument.
func BuildDTRunCSV(run DTRun, blocks []DTBlockSummary, answers []DTAnswerRow, includeKey bool) (DTRunCSV, error) {
	ctxHeader := dtRunContextHeader()
	ctxRow := dtRunContextRow(run)

	blocksCSV, err := writeCSV(
		append(append([]string{}, ctxHeader...),
			"block_no", "load", "correct", "total", "lure", "expired",
			"mean_confidence", "mean_latency_ms", "mean_latency_vs_baseline",
			"presented_digits", "expected_digits", "response_digits",
			"recall_outcome", "digits_correct", "digits_held", "recall_latency_ms",
			"memory_failure_pct", "block_review_status", "block_review_note",
		),
		func(emit func([]string) error) error {
			for _, b := range blocks {
				row := append(append([]string{}, ctxRow...),
					strconv.Itoa(b.BlockNo),
					b.Load,
					strconv.Itoa(b.Correct),
					strconv.Itoa(b.Total),
					strconv.Itoa(b.Lure),
					strconv.Itoa(b.Expired),
					strconv.Itoa(b.MeanConfidence),
					strconv.Itoa(b.MeanLatencyMs),
					strconv.FormatFloat(b.MeanLatencyVsBaseline, 'f', 2, 64),
					b.PresentedDigits,
					b.ExpectedDigits,
					b.ResponseDigits,
					b.RecallOutcome,
					strconv.Itoa(b.DigitsCorrect),
					strconv.Itoa(b.DigitsHeld),
					strconv.Itoa(b.RecallLatencyMs),
					// Negative means the recall expired and nothing was
					// attempted, which is not a memory failure of any
					// size. Left empty rather than written as a number,
					// because -100 in a spreadsheet becomes a data point.
					csvFailurePct(b.MemoryFailure),
					b.ReviewStatus,
					b.ReviewNote,
				)
				if err := emit(row); err != nil {
					return err
				}
			}
			return nil
		})
	if err != nil {
		return DTRunCSV{}, err
	}

	answerHeader := append(append([]string{}, ctxHeader...),
		"position_overall", "position_in_block", "block_no",
		"item_code", "item_version", "item_family",
		"outcome", "chosen_index", "is_lure", "confidently_wrong",
		"confidently_lured", "confidence", "latency_ms",
		"latency_vs_baseline_offsets", "brier",
	)
	if includeKey {
		answerHeader = append(answerHeader, "prompt", "chosen_text", "correct_text")
	}
	answersCSV, err := writeCSV(answerHeader, func(emit func([]string) error) error {
		for _, a := range answers {
			row := append(append([]string{}, ctxRow...),
				strconv.Itoa(a.Position),
				strconv.Itoa(a.PositionInBlock),
				strconv.Itoa(a.BlockNo),
				a.ItemCode,
				strconv.Itoa(a.ItemVersion),
				a.ItemFamily,
				a.Outcome,
				// -1 is an expiry: nothing was chosen. Empty rather than
				// -1 so a spreadsheet does not average it.
				csvChosenIndex(a.ChosenIndex),
				csvValue(a.IsLure),
				csvValue(a.ConfidentlyWrong),
				csvValue(a.ConfidentlyLured),
				strconv.Itoa(a.Confidence),
				strconv.Itoa(a.LatencyMs),
				strconv.FormatFloat(a.LatencyVsBaseline, 'f', 2, 64),
				csvBrier(a.Brier),
			)
			if includeKey {
				row = append(row, a.Prompt, a.ChosenText, a.CorrectText)
			}
			if err := emit(row); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return DTRunCSV{}, err
	}

	// The key in the filename, truncated: enough to tell two files
	// apart in a folder, short enough to read.
	stem := "decision-test-run"
	if len(run.SessionKey) >= 8 {
		stem += "-" + run.SessionKey[:8]
	}
	return DTRunCSV{
		FilenameStem: stem,
		BlocksCSV:    blocksCSV,
		AnswersCSV:   answersCSV,
		BlockRows:    len(blocks),
		AnswerRows:   len(answers),
	}, nil
}

func writeCSV(header []string, rows func(emit func([]string) error) error) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write(header); err != nil {
		return "", fmt.Errorf("decision test: csv header: %w", err)
	}
	if err := rows(w.Write); err != nil {
		return "", fmt.Errorf("decision test: csv row: %w", err)
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", fmt.Errorf("decision test: csv flush: %w", err)
	}
	return buf.String(), nil
}

func csvTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

// csvNullableInt keeps NULL as empty. Zero is a real and different
// answer for the prior counts, so the two must not collapse here either.
func csvNullableInt(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}

func csvChosenIndex(i int) string {
	if i < 0 {
		return ""
	}
	return strconv.Itoa(i)
}

func csvBrier(b float64) string {
	if b < 0 {
		return ""
	}
	return strconv.FormatFloat(b, 'f', 4, 64)
}

func csvFailurePct(f float64) string {
	if f < 0 {
		return ""
	}
	return strconv.Itoa(int(f * 100))
}

// DTExportRun reads a run and renders it as the two files.
func (r *Repo) DTExportRun(ctx context.Context, key string, includeKey bool) (DTRunCSV, error) {
	run, blocks, answers, err := r.DTGetRun(ctx, key)
	if err != nil {
		return DTRunCSV{}, err
	}
	return BuildDTRunCSV(*run, blocks, answers, includeKey)
}
