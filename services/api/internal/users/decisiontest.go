package users

import (
	"context"
	cryptorand "crypto/rand"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// Storage for the decision test (docs/fsd-decision-test.md).
//
// The answer key lives in dt_items and is read here. It is never put in
// a response: grading happens in this package, the handler returns an
// acknowledgement, and the client learns nothing about correctness.
// That is what keeps confidence usable as a dependent variable, since a
// participant who is told they were wrong stops giving an
// uncontaminated rating.

// Block loads in running order. Block 5 returns to block 1's difficulty
// and is the fatigue control: without it the ramp is monotonic and load
// is perfectly confounded with time-on-task, so a decline at the end
// has two explanations and no way to choose between them.
var dtBlockLoads = []string{"d3", "d4", "d4_plus1", "d4_plus3", "d3_control"}

// dtTransform is the operation applied to each digit before the number
// is returned. The transformation blocks are the steepest step in the
// ramp because they move the task from storage to storage plus
// executive processing.
var dtTransform = map[string]int{
	"d3": 0, "d4": 0, "d4_plus1": 1, "d4_plus3": 3, "d3_control": 0,
}

// dtDigitCount is how many digits each block asks a participant to hold.
var dtDigitCount = map[string]int{
	"d3": 3, "d4": 4, "d4_plus1": 4, "d4_plus3": 4, "d3_control": 3,
}

// DTItem is one item as stored, key included. Never serialised to a
// client: the handler copies prompt, reminder and options across and
// leaves CorrectIndex behind.
type DTItem struct {
	ID           int64
	Code         string
	Version      int
	Family       string
	Kind         string
	Prompt       string
	Reminder     string
	Options      []string
	CorrectIndex int
	LureIndex    int
}

// DTBlock is one block ready to serve: the digits to hold and the
// questions to answer while holding them.
type DTBlock struct {
	BlockNo   int
	Load      string
	Digits    string
	Transform string
	Items     []DTItem
	// FirstPosition is the one-based position of this block's first
	// question in the whole test, for the progress counter.
	FirstPosition int
}

// DTSession is the handle a run is addressed by.
type DTSession struct {
	ID        int64
	PublicID  string
	Status    string
	Synthetic bool
}

// DTBlockCount and DTQuestionsPerBlock are the instrument's shape.
// Thirty questions at about 20.5 seconds each is what fits inside the
// owner's fifteen-minute ceiling once intake, instructions, a practice
// block and the thank-you are taken out.
const (
	DTBlockCount        = 5
	DTQuestionsPerBlock = 6
	DTQuestionCount     = DTBlockCount * DTQuestionsPerBlock
)

// The instrument version is no longer a constant. It is derived from the
// owner-editable timings (DTSettings.InstrumentVersion), because a
// question answered in 20 seconds and the same question answered in 25
// are not the same measurement, and a version that has to be remembered
// separately is a version that will eventually be forgotten.

// StartDecisionTest opens a session and returns it.
//
// The participant row is written here rather than on keystroke, so
// somebody who reads the effort warning and leaves has given us
// nothing: no name, no email, no row.
func (r *Repo) StartDecisionTest(
	ctx context.Context,
	in DTIntake, cond DTConditions, synthetic bool, visitorKey string, userID *int64,
	instrumentVersion, itemSetVersion, keyVersion string,
) (*DTSession, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("decision test: begin: %w", err)
	}
	defer tx.Rollback(ctx) //nolint:errcheck // rollback after commit is a no-op

	var participantID *int64
	if in.any() {
		var pid int64
		err = tx.QueryRow(ctx, `
			INSERT INTO dt_participants
			  (user_id, display_name, age_range, education, occupation, email, email_key, wants_results)
			VALUES ($1,$2,$3,$4,$5,$6,$7,$8)
			RETURNING id`,
			userID, in.DisplayName, in.AgeRange, in.Education, in.Occupation,
			in.Email, strings.ToLower(strings.TrimSpace(in.Email)), in.WantsResults,
		).Scan(&pid)
		if err != nil {
			return nil, fmt.Errorf("decision test: participant: %w", err)
		}
		participantID = &pid
	}

	// A repeat is marked, never blocked. Blocking is unenforceable (a
	// private window defeats it) so it would only inconvenience honest
	// participants, and a second run by somebody who knows the trick is
	// a different measurement rather than a spoiled one.
	isRepeat, matchedBy := r.dtDetectRepeat(ctx, userID, in.Email, visitorKey)

	s := &DTSession{Status: "running", Synthetic: synthetic}
	err = tx.QueryRow(ctx, `
		INSERT INTO dt_sessions
		  (participant_id, instrument_version, item_set_version, key_version,
		   audio_mode, device_class, tap_check_passed, baseline_rt_ms, baseline_rt_sd_ms,
		   is_repeat, repeat_matched_by, visitor_key, is_synthetic)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, public_id::text`,
		participantID, instrumentVersion, itemSetVersion, keyVersion,
		cond.AudioMode, cond.DeviceClass, cond.TapCheckPassed,
		nullableInt(cond.BaselineRTMs), nullableInt(cond.BaselineRTSDMs),
		isRepeat, matchedBy, visitorKey, synthetic,
	).Scan(&s.ID, &s.PublicID)
	if err != nil {
		return nil, fmt.Errorf("decision test: session: %w", err)
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("decision test: commit: %w", err)
	}
	return s, nil
}

// DTIntake is the optional demographic form.
type DTIntake struct {
	DisplayName, AgeRange, Education, Occupation, Email string
	WantsResults                                        bool
}

func (i DTIntake) any() bool {
	return i.DisplayName != "" || i.AgeRange != "" || i.Education != "" ||
		i.Occupation != "" || i.Email != "" || i.WantsResults
}

// DTConditions is what was measured about the setup before the first
// question. Each field splits the sample, so none of it can be
// reconstructed later.
type DTConditions struct {
	AudioMode, DeviceClass string
	TapCheckPassed         bool
	BaselineRTMs           int
	BaselineRTSDMs         int
}

func nullableInt(v int) *int {
	if v <= 0 {
		return nil
	}
	return &v
}

// dtDetectRepeat looks for a previous session by this person, in
// descending order of reliability: account, then email, then the
// first-party anonymous cookie.
func (r *Repo) dtDetectRepeat(ctx context.Context, userID *int64, email, visitorKey string) (bool, string) {
	check := func(q string, arg any) bool {
		var n int
		if err := r.pool.QueryRow(ctx, q, arg).Scan(&n); err != nil {
			return false
		}
		return n > 0
	}
	if userID != nil && check(
		`SELECT count(*) FROM dt_sessions s JOIN dt_participants p ON p.id = s.participant_id
		  WHERE p.user_id = $1`, *userID) {
		return true, "account"
	}
	if k := strings.ToLower(strings.TrimSpace(email)); k != "" && check(
		`SELECT count(*) FROM dt_sessions s JOIN dt_participants p ON p.id = s.participant_id
		  WHERE p.email_key = $1`, k) {
		return true, "email"
	}
	if visitorKey != "" && check(
		`SELECT count(*) FROM dt_sessions WHERE visitor_key = $1`, visitorKey) {
		return true, "cookie"
	}
	return false, ""
}

// DTSessionByKey resolves the opaque handle a client holds.
func (r *Repo) DTSessionByKey(ctx context.Context, key string) (*DTSession, error) {
	s := &DTSession{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, public_id::text, status, is_synthetic FROM dt_sessions WHERE public_id = $1::uuid`,
		key).Scan(&s.ID, &s.PublicID, &s.Status, &s.Synthetic)
	if err != nil {
		return nil, fmt.Errorf("decision test: session %q: %w", key, err)
	}
	return s, nil
}

// DTBuildBlock assembles a block: its digits and its six questions.
//
// Digits are generated per session rather than fixed, so a participant
// who hears about the test cannot arrive knowing the numbers. The items
// are standardized and fixed; the numbers are not, because they carry
// no measurement of their own.
func (r *Repo) DTBuildBlock(ctx context.Context, blockNo int, items []DTItem) (*DTBlock, error) {
	if blockNo < 1 || blockNo > DTBlockCount {
		return nil, fmt.Errorf("decision test: block %d out of range", blockNo)
	}
	load := dtBlockLoads[blockNo-1]
	digits, err := dtRandomDigits(dtDigitCount[load])
	if err != nil {
		return nil, err
	}
	transform := "none"
	switch dtTransform[load] {
	case 1:
		transform = "plus1"
	case 3:
		transform = "plus3"
	}
	return &DTBlock{
		BlockNo:       blockNo,
		Load:          load,
		Digits:        digits,
		Transform:     transform,
		Items:         items,
		FirstPosition: (blockNo-1)*DTQuestionsPerBlock + 1,
	}, nil
}

// DTRecordBlockDigits stores what a block showed, so the recall can be
// graded against it later without the client being trusted to say what
// it was asked to hold.
func (r *Repo) DTRecordBlockDigits(ctx context.Context, sessionID int64, b *DTBlock) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO dt_recalls
		  (session_id, block_no, block_load, presented_digits, expected_digits, outcome)
		VALUES ($1,$2,$3,$4,$5,'expired')
		ON CONFLICT (session_id, block_no) DO UPDATE
		  SET presented_digits = EXCLUDED.presented_digits,
		      expected_digits  = EXCLUDED.expected_digits`,
		sessionID, b.BlockNo, b.Load, b.Digits, DTExpectedDigits(b.Digits, b.Load))
	if err != nil {
		return fmt.Errorf("decision test: record digits: %w", err)
	}
	return nil
}

// DTExpectedDigits applies the block's transformation. Digits wrap at
// ten, so 9 plus 3 is 2: without wrapping, a +3 block could not use a
// digit above six and the numbers would give the rule away.
func DTExpectedDigits(digits, load string) string {
	add := dtTransform[load]
	if add == 0 {
		return digits
	}
	out := []rune(digits)
	for i, c := range out {
		if c < '0' || c > '9' {
			continue
		}
		out[i] = rune('0' + (int(c-'0')+add)%10)
	}
	return string(out)
}

// DTGradeRecall classifies a returned number.
//
// A missed recall is a data point, not a failure (owner, 2026-10-04):
// the block's six answers are kept and scored normally either way.
//
// The distinction that matters is untransformed against wrong_digits.
// Returning 1234 when 2345 was required means the number survived and
// the operation did not, which is storage holding while the executive
// fails, and that is the mechanism this whole test exists to measure.
// Scoring recall pass/fail would erase it.
func DTGradeRecall(presented, expected, response string) (outcome string, digitsCorrect int) {
	response = strings.TrimSpace(response)
	if response == "" {
		return "expired", 0
	}
	for i := 0; i < len(expected) && i < len(response); i++ {
		if expected[i] == response[i] {
			digitsCorrect++
		}
	}
	switch {
	case response == expected:
		return "exact", digitsCorrect
	case response == presented && presented != expected:
		return "untransformed", digitsCorrect
	case digitsCorrect > 0:
		return "partial", digitsCorrect
	default:
		return "wrong_digits", 0
	}
}

// DTSaveRecall stores the graded recall.
func (r *Repo) DTSaveRecall(ctx context.Context, sessionID int64, blockNo int, response string, latencyMs int) error {
	var presented, expected string
	if err := r.pool.QueryRow(ctx,
		`SELECT presented_digits, expected_digits FROM dt_recalls WHERE session_id=$1 AND block_no=$2`,
		sessionID, blockNo).Scan(&presented, &expected); err != nil {
		return fmt.Errorf("decision test: recall lookup: %w", err)
	}
	outcome, correct := DTGradeRecall(presented, expected, response)
	_, err := r.pool.Exec(ctx, `
		UPDATE dt_recalls
		   SET response_digits=$3, outcome=$4, digits_correct=$5, latency_ms=$6
		 WHERE session_id=$1 AND block_no=$2`,
		sessionID, blockNo, response, outcome, correct, nullableInt(latencyMs))
	if err != nil {
		return fmt.Errorf("decision test: save recall: %w", err)
	}
	return nil
}

// DTSaveAnswer grades one answer and stores it.
//
// Grading is here, in the api, reading the key from the database. The
// caller gets an error or nothing; it never gets the verdict.
//
// A chosen index of -1 is an expiry, which is its own outcome rather
// than an error or a dropped row. A timeout under load is a result, and
// it is the one outcome that cannot be reconstructed afterwards if it
// is thrown away.
func (r *Repo) DTSaveAnswer(
	ctx context.Context, sessionID int64, blockNo, positionOverall, chosenIndex, latencyMs, confidence int,
	item DTItem,
) error {
	outcome := "other"
	var chosen *int
	switch {
	case chosenIndex < 0:
		outcome = "expired"
	case chosenIndex == item.CorrectIndex:
		outcome, chosen = "correct", &chosenIndex
	case chosenIndex == item.LureIndex:
		outcome, chosen = "lure", &chosenIndex
	default:
		chosen = &chosenIndex
	}

	load := dtBlockLoads[blockNo-1]
	positionInBlock := (positionOverall-1)%DTQuestionsPerBlock + 1
	var conf, lat *int
	if outcome != "expired" {
		conf, lat = &confidence, &latencyMs
	}
	var answeredAt *time.Time
	if outcome != "expired" {
		now := time.Now().UTC()
		answeredAt = &now
	}

	_, err := r.pool.Exec(ctx, `
		INSERT INTO dt_answers
		  (session_id, item_id, item_code, item_version, block_no, block_load,
		   position_in_block, position_overall, outcome, chosen_index,
		   latency_ms, confidence, answered_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		ON CONFLICT (session_id, position_overall) DO NOTHING`,
		sessionID, item.ID, item.Code, item.Version, blockNo, load,
		positionInBlock, positionOverall, outcome, chosen, lat, conf, answeredAt)
	if err != nil {
		return fmt.Errorf("decision test: save answer: %w", err)
	}
	return nil
}

// DTFinish closes a session and returns the only figure a participant
// is ever shown: how many of the thirty were correct, never item by
// item.
func (r *Repo) DTFinish(ctx context.Context, sessionID int64, strategy string) (correct, total int, err error) {
	if _, err = r.pool.Exec(ctx,
		`UPDATE dt_sessions SET status='completed', finished_at=now(), recall_strategy=$2 WHERE id=$1`,
		sessionID, strategy); err != nil {
		return 0, 0, fmt.Errorf("decision test: finish: %w", err)
	}
	err = r.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE outcome='correct'), count(*)
		  FROM dt_answers WHERE session_id=$1`, sessionID).Scan(&correct, &total)
	if err != nil {
		return 0, 0, fmt.Errorf("decision test: score: %w", err)
	}
	return correct, total, nil
}

// DTScoredItems returns the thirty scored items in presentation order,
// key included. Callers serve prompt, reminder and options and keep the
// rest.
func (r *Repo) DTScoredItems(ctx context.Context) ([]DTItem, error) {
	return r.dtItems(ctx, "scored")
}

// DTPracticeItems returns the practice items, which carry lures too: a
// gentle practice block would rehearse a different experience and leave
// the first real trap arriving at question one.
func (r *Repo) DTPracticeItems(ctx context.Context) ([]DTItem, error) {
	return r.dtItems(ctx, "practice")
}

func (r *Repo) dtItems(ctx context.Context, kind string) ([]DTItem, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, code, version, family, kind, prompt, reminder, options, correct_index, lure_index
		  FROM dt_items WHERE kind=$1 AND active ORDER BY position, id`, kind)
	if err != nil {
		return nil, fmt.Errorf("decision test: items: %w", err)
	}
	defer rows.Close()
	var out []DTItem
	for rows.Next() {
		var it DTItem
		var raw []byte
		if err := rows.Scan(&it.ID, &it.Code, &it.Version, &it.Family, &it.Kind,
			&it.Prompt, &it.Reminder, &raw, &it.CorrectIndex, &it.LureIndex); err != nil {
			return nil, fmt.Errorf("decision test: scan item: %w", err)
		}
		if err := json.Unmarshal(raw, &it.Options); err != nil {
			return nil, fmt.Errorf("decision test: item %s options: %w", it.Code, err)
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

// dtRandomDigits makes the number a block asks a participant to hold.
//
// crypto/rand rather than math/rand: not for secrecy, which does not
// matter here, but because a predictable sequence across sessions would
// let a participant who takes the test twice meet the same numbers, and
// the second run would measure recall of the first.
func dtRandomDigits(n int) (string, error) {
	b := make([]byte, n)
	if _, err := cryptorand.Read(b); err != nil {
		return "", fmt.Errorf("decision test: digits: %w", err)
	}
	out := make([]rune, n)
	for i, v := range b {
		out[i] = rune('0' + int(v)%10)
	}
	return string(out), nil
}
