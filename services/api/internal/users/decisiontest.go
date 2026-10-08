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

// DTBlockLoad names a block's load for callers outside this package.
// Out of range returns "", which reads as missing rather than as a load
// the block did not have.
func DTBlockLoad(blockNo int) string {
	if blockNo < 1 || blockNo > len(dtBlockLoads) {
		return ""
	}
	return dtBlockLoads[blockNo-1]
}

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
	// Set on the run that opens a session, so the event stream can carry
	// it. Not read back by session(): the stored column is the record,
	// this is for the one emit at the start.
	IsRepeat bool
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
	allowFixtures bool,
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
	byAccount, byEmail, byCookie := r.dtPriorSittings(ctx, userID, in.Email, visitorKey, synthetic)
	isRepeat, source := dtAnyPrior(byAccount, byEmail, byCookie)
	matchedBy := ""
	if isRepeat {
		matchedBy = source
	}

	s := &DTSession{Status: "running", Synthetic: synthetic, IsRepeat: isRepeat}
	err = tx.QueryRow(ctx, `
		INSERT INTO dt_sessions
		  (participant_id, instrument_version, item_set_version, key_version,
		   audio_mode, device_class, tap_check_passed, baseline_rt_ms, baseline_rt_sd_ms,
		   is_repeat, repeat_matched_by, visitor_key, is_synthetic,
		   prior_by_account, prior_by_email, prior_by_cookie)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16)
		RETURNING id, public_id::text`,
		participantID, instrumentVersion, itemSetVersion, keyVersion,
		cond.AudioMode, cond.DeviceClass, cond.TapCheckPassed,
		nullableInt(cond.BaselineRTMs), nullableInt(cond.BaselineRTSDMs),
		isRepeat, matchedBy, visitorKey, synthetic,
		byAccount, byEmail, byCookie,
	).Scan(&s.ID, &s.PublicID)
	if err != nil {
		return nil, fmt.Errorf("decision test: session: %w", err)
	}
	// The thirty questions, drawn now and inside this transaction.
	//
	// In here rather than later so a session can never exist without its
	// items: the client fetches blocks one at a time, and a run whose
	// draw failed half way would be unservable from whichever block the
	// failure landed on. If the draw refuses, the session is rolled back
	// and the participant is told the test could not start, which is a
	// far better outcome than fifteen minutes on a short instrument.
	//
	// A repeat sitting avoids what this participant has already seen
	// (D3a). The lookup runs on the pool rather than the transaction
	// because it reads earlier sessions, which this one cannot have
	// touched.
	var exclude []int64
	if seen, err := r.DTSeenItems(ctx, visitorKey, participantID); err != nil {
		// Not fatal. Failing to remember what somebody saw last time
		// costs a less varied draw; refusing the run costs the run.
		_ = err
	} else {
		exclude = seen
	}
	if err := DTDrawItems(ctx, tx, s.ID, exclude, allowFixtures); err != nil {
		return nil, fmt.Errorf("decision test: draw: %w", err)
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

// dtPriorSittings asks each identity how many sittings it has already
// seen, and returns all three answers.
//
// It does not combine them. The owner's rule, 2026-10-05: "we dont get
// to cherry pick data sets in that manner". An earlier version returned
// a single number taken from whichever source reported the most, which
// collapsed three observations into one at write time, discarded the
// components, and biased the figure upward whenever the sources
// disagreed. The combination now lives in v_dt_answers, where it is one
// line and the components are still there beside it.
//
// nil means there is no such link to ask, which is a different fact
// from zero. Zero is an identity reporting no prior sitting, which is
// positive evidence of a first attempt; nil is silence.
//
// These counts are point-in-time and are stored for that reason. The
// same queries run next month would include sittings that had not
// happened when this one started.
func (r *Repo) dtPriorSittings(
	ctx context.Context, userID *int64, email, visitorKey string, synthetic bool,
) (byAccount, byEmail, byCookie *int) {
	// Synthetic and real runs are counted separately. An agent run must
	// not bump a real participant's count, and a real run must not
	// inherit one from an agent that happened to share a browser. The
	// detection this replaces did not separate them: a latent fault
	// that had not fired only because no synthetic run shared a
	// visitor_key with a real one.
	count := func(q string, arg any) *int {
		var n int
		if err := r.pool.QueryRow(ctx, q, arg, synthetic).Scan(&n); err != nil {
			// A failed count is not zero. Returning nil says "could not
			// ask", which is the truth and is already the value that
			// means the sequence is unknown from this source.
			return nil
		}
		return &n
	}

	if userID != nil {
		byAccount = count(
			`SELECT count(*) FROM dt_sessions s JOIN dt_participants p ON p.id = s.participant_id
			  WHERE p.user_id = $1 AND s.is_synthetic = $2`, *userID)
	}
	if k := strings.ToLower(strings.TrimSpace(email)); k != "" {
		byEmail = count(
			`SELECT count(*) FROM dt_sessions s JOIN dt_participants p ON p.id = s.participant_id
			  WHERE p.email_key = $1 AND s.is_synthetic = $2`, k)
	}
	if visitorKey != "" {
		byCookie = count(
			`SELECT count(*) FROM dt_sessions WHERE visitor_key = $1 AND is_synthetic = $2`,
			visitorKey)
	}
	return byAccount, byEmail, byCookie
}

// dtAnyPrior answers "has this person been here before" from the three
// counts, and names the most reliable source that saw a prior sitting.
//
// This is an OR across the sources, not a selection among them, and the
// distinction matters. Picking one number and discarding the rest is
// what the owner rejected. Asking whether ANY identity saw a prior
// sitting discards nothing: it is the complete answer to a yes-or-no
// question, and all three counts remain stored beside it.
//
// It follows that is_repeat can be true while the view's derived
// attempt_no reads 1, when the account reports no prior sitting and the
// cookie reports one. That is not an inconsistency to paper over. It is
// the signal that somebody sat the test anonymously and later signed
// up, which is exactly the case a single number would have hidden, and
// a reader who sees the two disagree should go and look at the three
// counts.
func dtAnyPrior(byAccount, byEmail, byCookie *int) (bool, string) {
	any := false
	source := ""
	// Reliability order, so the name attached to a repeat is the best
	// evidence for it rather than whichever source was checked last.
	for _, c := range []struct {
		n    *int
		name string
	}{{byAccount, "account"}, {byEmail, "email"}, {byCookie, "cookie"}} {
		if c.n != nil && *c.n > 0 {
			any = true
			if source == "" {
				source = c.name
			}
		}
	}
	return any, source
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
func DTGradeRecall(presented, expected, response string) (outcome string, digitsCorrect, digitsHeld int) {
	response = strings.TrimSpace(response)
	if response == "" {
		return "expired", 0, 0
	}
	for i := 0; i < len(expected) && i < len(response); i++ {
		if expected[i] == response[i] {
			digitsCorrect++
		}
	}

	// Retention, as distinct from correctness.
	//
	// Scored against whichever of the two numbers the response is closer
	// to, because either one proves the number survived: giving back the
	// raw digits shows it was held, giving back the transformed digits
	// shows it was held and operated on. Only matching neither means
	// memory actually failed.
	//
	// Measuring against the expected number alone would score a
	// perfectly-held but untransformed answer as total memory loss,
	// which is the exact opposite of what happened and would bury the
	// sharpest signal the instrument produces.
	var vsPresented int
	for i := 0; i < len(presented) && i < len(response); i++ {
		if presented[i] == response[i] {
			vsPresented++
		}
	}
	digitsHeld = digitsCorrect
	if vsPresented > digitsHeld {
		digitsHeld = vsPresented
	}
	switch {
	case response == expected:
		return "exact", digitsCorrect, digitsHeld
	case response == presented && presented != expected:
		return "untransformed", digitsCorrect, digitsHeld
	case digitsCorrect > 0:
		return "partial", digitsCorrect, digitsHeld
	default:
		return "wrong_digits", 0, digitsHeld
	}
}

// DTSaveRecall stores the graded recall and returns what it graded, so
// the caller can put the block in the event stream. The grade still goes
// nowhere near the participant: the handler's response is "stored".
func (r *Repo) DTSaveRecall(
	ctx context.Context, sessionID int64, blockNo int, response string, latencyMs int,
) (outcome string, digitsHeld int, err error) {
	var presented, expected string
	if err := r.pool.QueryRow(ctx,
		`SELECT presented_digits, expected_digits FROM dt_recalls WHERE session_id=$1 AND block_no=$2`,
		sessionID, blockNo).Scan(&presented, &expected); err != nil {
		return "", 0, fmt.Errorf("decision test: recall lookup: %w", err)
	}
	outcome, correct, held := DTGradeRecall(presented, expected, response)
	if _, err := r.pool.Exec(ctx, `
		UPDATE dt_recalls
		   SET response_digits=$3, outcome=$4, digits_correct=$5, latency_ms=$6, digits_held=$7
		 WHERE session_id=$1 AND block_no=$2`,
		sessionID, blockNo, response, outcome, correct, nullableInt(latencyMs), held,
	); err != nil {
		return "", 0, fmt.Errorf("decision test: save recall: %w", err)
	}
	return outcome, held, nil
}

// DTSessionFinish is what the finish event needs beyond the two figures
// the participant is shown.
type DTSessionFinish struct {
	Expired      int
	DurationS    int
	WantsResults bool
}

// DTSessionSummary reads the session-level figures for the finish event.
// Counted from the answer rows and the timestamps rather than stored,
// for the same reason as DTBlockTally.
func (r *Repo) DTSessionSummary(ctx context.Context, sessionID int64) (DTSessionFinish, error) {
	var out DTSessionFinish
	err := r.pool.QueryRow(ctx, `
		SELECT (SELECT count(*) FROM dt_answers WHERE session_id=s.id AND outcome='expired'),
		       coalesce(extract(epoch FROM (coalesce(s.finished_at, now()) - s.started_at))::int, 0),
		       coalesce(p.wants_results, false)
		  FROM dt_sessions s
		  LEFT JOIN dt_participants p ON p.id = s.participant_id
		 WHERE s.id = $1`,
		sessionID).Scan(&out.Expired, &out.DurationS, &out.WantsResults)
	if err != nil {
		return out, fmt.Errorf("decision test: session summary: %w", err)
	}
	return out, nil
}

// DTBlockTally counts how a block went, for the event stream. Derived
// from the answer rows rather than tracked alongside them, per
// docs/metrics.md: the grain is one row per question and everything else
// is counted from it.
func (r *Repo) DTBlockTally(ctx context.Context, sessionID int64, blockNo int) (correct, expired int, err error) {
	err = r.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE outcome='correct'),
		       count(*) FILTER (WHERE outcome='expired')
		  FROM dt_answers WHERE session_id=$1 AND block_no=$2`,
		sessionID, blockNo).Scan(&correct, &expired)
	if err != nil {
		return 0, 0, fmt.Errorf("decision test: block tally: %w", err)
	}
	return correct, expired, nil
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
	// Confidence is stored NULL rather than zero when it was never
	// given, which happens when the clock runs out while the
	// participant is still on the rating step. Zero would be a rating,
	// would sit below "Guessing" at 10, and would be averaged into every
	// calibration figure as though somebody had claimed it.
	//
	// The views already handle a NULL confidence, because an expired
	// answer has always had one: avg() skips it and the Brier score is
	// not computed. So an answer recovered this way contributes its
	// accuracy, its latency and its lure capture, and abstains from the
	// calibration measure it has nothing to say about.
	var conf, lat *int
	if outcome != "expired" {
		lat = &latencyMs
		if confidence > 0 {
			conf = &confidence
		}
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
	// Idempotent, because the run screen calls this twice: once to close
	// the run before the debrief question is shown, so a participant who
	// shuts the tab on that question is recorded as having completed
	// thirty answers rather than abandoned them, and once more with the
	// answer itself.
	//
	// Both branches matter. finished_at keeps its first value, because
	// the run really did end before the debrief and the second call
	// arrives however long the participant took to read three options,
	// which would otherwise be added to every duration. And an empty
	// strategy never overwrites a stored one, so the order of the two
	// calls cannot erase the answer.
	if _, err = r.pool.Exec(ctx, `
		UPDATE dt_sessions
		   SET status='completed',
		       finished_at = coalesce(finished_at, now()),
		       recall_strategy = CASE WHEN $2::text = '' THEN recall_strategy ELSE $2::text END
		 WHERE id=$1`,
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
