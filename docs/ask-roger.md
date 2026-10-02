# Ask Roger

The state of Phase 4, written to be read cold. What exists, what does
not, what is known to be wrong, and what is waiting on a decision.

**Last updated 2026-10-01.** **Production is `0ca151e2c502`**, database
at migration 48. Figures below were read from the production database on
that date, not estimated.

Specification lives in [`FSD.md`](FSD.md) §5.5 (FR-CHAT-01..21). This
document is the state of the build against it and the reasoning that is
not in the requirements.

---

## 1. What it is, and what it is for

An assistant on rogerhenley.dev that answers questions about Roger, in
his voice, from his own records, with sources. The reader is a hiring
manager or a recruiter, often visiting once, often out of hours.

There is a second purpose and it is not secondary. Every interaction is
recorded in a form the owner can grade, and a graded interaction is
training data for a persona adapter (D-26). The ability to generate
useful training sets quickly is treated here as a feature of equal
weight to the assistant working at all. That is why the instrumentation
landed in the same commit as the persona and not after it.

---

## 2. Current state

| Piece | State | Where |
|---|---|---|
| `ChatService` RPCs | **Live**, 8 of 11 | `handlers/chat.go` |
| Conversation store | **Live** | migration `00045`, `users/chat.go` |
| Retrieval, public and private | **Live** | `users/chat_retrieval.go`, `00046` |
| Persona prompt | **Live, v3** (`v3`) | `prompts/askroger.go` |
| Q&A bank + admin surface | **Live**, 13 entries approved, 64 phrasings, all embedded | `00047`, `/admin/qa` |
| Phrasing embedding job | **Live**, every 5 min | `chat/qaembed.go` |
| Decision capture and grading | **Live** | `00048`, `users/chat_decision.go` |
| Answer pipeline | **Live** | `internal/chat/answer.go` |
| `/ask` page and side panel | **Live**, used by the owner | `app/ask/`, `components/ask/` |
| Site guide in the corpus | **Live**, top hit for site questions | `content/other/career-site-guide.md` |
| Action intents (D-25) | **Live**, never yet seen to fire | `chat/intent.go` |
| Bank near-miss recording | **Live** | `users/qa.go`, `chat/answer.go` |
| Chat and meeting events | **Live**, all 9 verified firing on prod | `events/events.go` |
| Admin database queries | **Live**, dropdown on `/ask` | `users/admin_queries.go` |
| Admin grading console | **Live**, 51 of 1,743 rows graded | `/admin/decisions` |
| Streaming transport | Stream is real, **one delta** | `handlers/chat.go` |
| Escalation, quotas | **Refused, not stubbed** | `Escalate`, `GetQuota` |
| Golden set (FR-CHAT-15) | **Not built** | — |

Migrations 00045 to 00048 are applied in production; the database is at
48.

**Everything above is deployed and in use.** The plumbing is no longer
the open question, and most of what was unproven on 2026-09-30 has since
been exercised against production: 15 chat answers recorded, 51 decision
rows graded, the Q&A bank at 13 approved entries against a corpus of 38
documents, and every one of the nine chat and meeting events confirmed
firing with correct props.

Two things are still unproven, and both are behaviour rather than code.
**No action intent has been observed firing**, so D-25's confirm step
has never been exercised by a real answer. And the bank's **margin rule
has only been tested by measurement**, not by a live paraphrase: a
question matching its entry's exact wording proves the entry exists, not
that a differently-worded question reaches it.

What the numbers say about the design holding: a bank answer returns in
about 0.1 s against a 39.4 s mean on the model path, of which only 6.3 s
is generation. The rest is prompt evaluation on the per-question
evidence, which is why the Q&A bank matters more than any model change
available on this hardware.

Prompt fingerprints at this head:

    ask_roger_persona  v3
    requirement_judge  v12:df4c4cc7
    resume_tailor      v3:3e9fa0c1
    jd_requirements    v4:e13cbaeb
    posting_check      v1:30f63ac3

---

## 3. The decision that shapes everything: build for the CPX41

Decided by the owner on 2026-09-30, after measuring the alternative.

Inference runs on the production box's own Ollama (`qwen3:4b-q8_0`),
not on the owner's Mac and not on a hosted API. The reasoning was
availability over latency: the audience visits once, often out of
hours, and "not currently available" is a dead feature on a site whose
premise is that it is the working version of the owner's practice. The
Mac alternative also meant a tunnel through Starlink CGNAT, a
heartbeat, hours of operation and a banner, all of which fail quietly.

### The measurements

Taken on the production box, persona prefix cached, chunk count varied:

| Passages shown | Prompt tokens | First token | Total |
|---|---|---|---|
| 2 | 1019 | 11.4 s | 22.4 s |
| 4 | 1367 | 22.3 s | 26.9 s |
| 6 | 1715 | 37.1 s | 52.0 s |
| identical prompt, repeated | 1019 | **0.4 s** | 18.7 s |

Prompt evaluation runs at roughly **32 tokens/second** and generation at
**6.5**. FR-CHAT-11 asks for 2 seconds to first token. That is
unreachable here by 6x to 18x, and the requirement is not met.

**Memory is not the constraint, CPU is.** The CPX41's spare RAM was
reserved for this feature and that reservation is real, but more RAM
buys a bigger model, not a faster one. There is no GPU.

### What follows, and these are not optional

- **The system prompt must be byte-identical on every call.** Ollama
  reuses its KV cache for an identical prefix; that is the 11.4 s to
  0.4 s row above. Evidence goes in the user turn, never the system
  one. The same trick took a JD judge pass from 34 minutes to 10.
- **Retrieve few.** 2 to 3 passages shown, not 6. Each one costs about
  5 seconds before the first word.
- **The Q&A bank is the main path, not a fallback.** It answers common
  questions with no model call at all.
- **Brevity is correctness.** At 6.5 tok/s a 400-token answer is a
  minute of watching. Persona rule 9 caps answers at about 150 words
  and `AskRogerAnswerMaxTokens` is 320 as a backstop.
- **Be honest in the UI.** "Reading his records" for 15 seconds, not a
  spinner pretending to be fast.

### Still open

FR-CHAT-16 names Anthropic Claude via the Messages API as the
production default for generation. It was written before the no-spend
constraint. At this volume it would cost cents a month and would meet
the latency spec with none of the Mac's availability problem. Raised
with the owner on 2026-09-30 and not taken up. **Do not assume it.**

---

## 4. How an answer is produced

Reachable from outside the API since `ChatService` was mounted:
`CreateConversation`, `ListConversations`, `GetConversation`,
`SendMessage`, `DeleteConversation`, `RateMessage` and
`GetSuggestions` are implemented. `Escalate` and `GetQuota` refuse with
`unimplemented` rather than returning plausible zeroes, because a
stubbed quota would have the panel render an allowance the member does
not have.


Designed, partly built. Steps marked **[built]** exist and are tested.

1. Embed the question once, with `nomic-embed-text`. **[built]**
2. **Q&A bank lookup** with that embedding. `MatchQA` above
   `QAMatchThreshold`. A hit returns the owner's words verbatim and
   stops. **[built]**
3. **Restricted topics.** Compensation, references,
   employer-confidential matters and personal life (FR-CHAT-06) are
   refused in code, with no model call. The bank is consulted first, so
   reaching here means the owner has written no approved statement.
   Checked in code as well as in persona rule 7 because it is the one
   category where being talked into an answer does real damage, and
   because refusing here costs nothing while asking the model to refuse
   costs twenty seconds. The patterns are deliberately narrow: a false
   positive silently refuses a fair question, which is the expensive
   mistake. **[built]**
4. **Retrieval.** `SearchCorpusForChat`, gated on `chatbot_include` and
   on visibility. Capped at 24 considered, 2 to 3 shown. **[built]**
5. **Nothing retrieved** returns "I don't have anything in my records
   about that" (FR-CHAT-03). No model call. **[built]**
6. **Render.** `RenderAskUser` builds history, numbered passages and
   the question, and returns the citation map. **[built]**
7. **Generate** through the sidecar gateway. **[built, not streamed]**
8. **Validate citations** against the map; drop markers pointing at
   nothing, and tidy the text so the removal does not show. **[built]**
9. **Persist** message and citations in one transaction. **[built]**
10. **Log the decision** to `decision_log` as `chat_answer`, on the
    same best-effort footing as the JD pipeline. **[built]**

**Four of the six stages can answer with no model call at all**, which
is the entire point on this hardware.

Everything except persisting the message is best-effort. A Q&A bank
failure logs a warning and falls through to the model; a model outage
degrades to an honest reply (FR-CHAT-17); a decision-log failure
warns and returns the answer anyway. Only failing to persist the
member's message is fatal, because then they have nothing.

### Citations are enforced by construction, not by instruction

`RenderAskUser` numbers **only** the citable passages and returns that
exact slice as the citation map. Private-corpus passages are shown
unnumbered and untitled: there is no marker the model could write, so
the never-cite-private rule (FR-CHAT-04) has nothing to fail at. A
private document's own title is unpublished material and never reaches
the prompt.

### Tools

None in v1. D-25 allows a three-tool allowlist rendering UI intents the
member confirms, and FR-CHAT-13 requires the assistant never fire the
underlying call. Structured tool calls do not co-exist with streamed
prose through a single-shot gateway, so v1 is D-25's stated baseline:
the assistant names the contact page or the scheduler in words and the
surface links them. **Nothing the assistant says can cause an action.**

---

## 5. The training loop

This is the part the owner asked for explicitly, and the part most
likely to be quietly dropped under delivery pressure. It is not
optional.

### What is captured, per answer

Every answer writes one `decision_log` row with `kind = 'chat_answer'`,
`ref_kind = 'chat_message'`. The jsonb shapes are pinned in Go
(`users/chat_decision.go`) rather than left per-caller, because an
export that cannot be joined on in six months is worth nothing.

`input` carries:

- the question, the persona fingerprint, how many history turns were in
  the prompt, and the corpus scope retrieval ran at;
- **the whole retrieval set**, in rank order, with similarity, whether
  each passage was citable, whether it was shown, and the marker it was
  offered as. What was *not* shown is how a retrieval problem is told
  from a generation one: an answer that missed an obvious fact sitting
  at rank 4, below the cut, is a tuning problem;
- **the Q&A lookup, matched or not**, with the best similarity either
  way. The near misses are the only thing that can calibrate
  `QAMatchThreshold`, and they exist only because misses are logged;
- **per-stage timings**: embed, bank match, retrieve, first token,
  total.

`output` carries the answer, the surviving citations, the four flags,
the finish reason, and **markers offered / written / dropped**.

`model` is `code` for the paths that never called one (a bank hit, a
refusal, a no-support answer). Those rows still matter: "did the bank
fire when it should not have" is one of the most valuable labels the
owner can give, and it cannot be asked about a row that was never
written.

### What is computed rather than marked

**Citation validity** (FR-CHAT-04, acceptance 95 %) is
`markers_dropped` against `markers_written` across rows. A human will
not mark several hundred answers; a query will. An assistant that
invents a `[7]` is doing something specific and detectable.

**Latency** splits into `first_token_ms` and `latency_ms` because on
this hardware they move for different reasons: the first tracks how
much context was retrieved, the second how much was written. One number
cannot tell a retrieval regression from a verbose one.

### What the owner grades

`ReviewVocabulary["chat_answer"]`:

| Verdict | Means |
|---|---|
| `good` | as he would have answered |
| `needs_edit` | close, not his words |
| `wrong` | factually wrong or ungrounded |
| `rejected_correctly` | a refusal or "I don't know" that was right |
| `insufficient_evidence` | could not judge from what was shown |

`rejected_correctly` exists so a correct refusal is not graded as a
failure. Without it the assistant scores worst exactly where it behaved
best.

`needs_edit` is kept apart from `wrong` because it is the outcome that
produces the **best preference pair**: the corrected answer and the
model's own answer then differ in precisely the way an adapter needs to
learn.

`ReviewDimensions["chat_answer"]` adds per-rubric marks, each `yes`,
`partial`, `no` or `n/a`:

    grounded    voice

**Two, not five, and the cut came from watching the first real
grading.** Roger wrote a 608-character correction and left every radio
blank. That is the right instinct: the correction is the valuable
artefact and twenty radios after writing it is a toll.

Three of the original five were asking a person for something the row
already holds. `citations` is `markers_dropped` against
`markers_written`, counted at answer time, so FR-CHAT-04's 95 % is a
query rather than a judgement. `length` is `completion_tokens`.
`scope` is carried by the verdict, where `rejected_correctly` says a
refusal was right.

What remains needs a person. Whether an answer is grounded in what it
was shown cannot be computed from the answer, and whether it sounds
like Roger is the whole point of a persona adapter and the one thing no
counter will ever measure.

### The column that makes it training data

`decision_log.human_answer` is **the wording the owner would have
given**. It is deliberately separate from `human_note`:

- `human_note` explains a grade to a human reader.
- `human_answer` is text a model is meant to imitate.

An exporter cannot tell one from the other if they share a column, and
a note like "too long, and he never worked at Amazon" would be exported
as though it were a model answer.

With `response_text` it forms a preference pair: `human_answer` chosen,
`response_text` rejected. On its own it is a supervised target.

**This is the single highest-value action in the admin console.** A
graded answer with no correction can be counted. A graded answer with
one can be learned from.

### Grading, in the console

`/admin/decisions` renders a `chat_answer` row for grading rather than
for browsing, in the order the grader needs: what was asked, what was
said, then what it was said from.

- **The path is stated first**, in words ("from your Q&A bank, no
  model", "model unavailable, degraded reply"), because six different
  systems working or failing render almost identically to a reader.
- **Citations** are listed, and dropped markers are called out as what
  they are: the answer claimed support it did not have.
- **What it was shown**, with similarities, and what was retrieved and
  left out. Grounding is a property of an answer *given what it was
  shown*, so it cannot be marked from the answer alone.
- **The bank lookup**, matched or not, with the closest similarity
  against the threshold. Reading the near misses is how the threshold
  gets calibrated.
- **The correction box is seeded with the model's own answer**, so a
  correction costs an edit rather than a retype. That is the difference
  between grading a hundred answers and grading five. The original
  travels in a hidden field and an unchanged answer is dropped, because
  saving `human_answer` identical to `response_text` would be a
  preference pair of a thing against itself: noise nothing downstream
  could detect.
- Each rubric dimension carries a few words saying what it is asking,
  since "scope" and "length" are otherwise guesses, and a mark made
  from a guess still counts toward a rate.

**Not yet seen live.** Per the owner's standing rule it is not complete
until it is deployed, opened and reviewed with him, and there are no
`chat_answer` rows anywhere yet to render.

### What a row carries, and what it did not

Found by reading the first three real production answers on
2026-09-30. The rows recorded the answer and almost nothing that makes
an answer comparable to another answer, which is the whole purpose of
the table.

| Field | Was | Now |
|---|---|---|
| `completion_tokens` | 0, hardcoded | the provider's count |
| `prompt_version` | 0 | the persona version |
| `num_ctx` | 0 | the configured window |
| `run_id` | empty | one id per answer, joinable to the usage ledger |
| `first_token_ms` | equal to `latency_ms` | the provider's own prompt-eval time |
| `prompt_eval_ms` / `generate_ms` | absent | the provider's own split |
| `llm_usage` row | **never written** | written per model call, success or failure |

The last two matter most. Ollama reports `prompt_eval_duration` and
`eval_duration` on every response and the sidecar simply never read
them, so a 45 second answer recorded 44975 ms to first token and 45009
ms total: the same number twice, saying nothing. Without the split
there is no way to tell an answer that was slow because it **read** too
much from one that was slow because it **wrote** too much, and those
are fixed in opposite directions (show fewer passages, or ask for
shorter answers).

`llm_usage` was worse than incomplete, it was empty. That is the table
the monthly budget cap reads from (FR-CHAT-12), so the cap could never
have fired, and chat spend was invisible next to the JD reviewer's.

### Export

`ExportDecisionLog` (admin, MFA-fresh) emits JSON Lines. Each line
carries the prompt, the response, the input and output jsonb, usage
including `first_token_ms`, and the `human` block with verdict, note,
**answer** and **dimensions**. Format in
[`decision-log.md`](decision-log.md).

---

## 6. Known issues and things that are not right yet

0b. **The assistant could not see the career record at all.** Roger
   noticed every answer came from the same published article. It was
   worse than that: *every chunk of every answer* did.

       corpus_only (private)   31 docs   251 chunks
       public                   5 docs    52 chunks, 50 of them one article

   Retrieval defaults to public and the handler never widened it, so
   the assistant was answering career questions from one marketing
   article. "Does he have capital project experience" came back empty
   from a corpus whose own facts sheet answers it in a sentence.

   The private-corpus path was built and tested and never switched on.
   Fixed by setting `AllowPrivate` in the handler. The flag is not what
   keeps that material safe: private passages reach the model
   unnumbered and untitled so there is no marker it could cite,
   `Citable()` keeps them out of the source list, the persona is told
   they may inform and may never be named, and `chatbot_include`
   excludes a document outright.

   **The typo hypothesis was wrong.** The misspelt question scored
   0.574 and the corrected one 0.580, and both retrieved the same
   article. Spelling was not the problem; the corpus boundary was.

   **Still lopsided, and worth a look.** Even private is dominated by
   documentation: the UxTS guide is 50 chunks, while the résumé is 6
   and the career facts sheet is 2. The JD reviewer solved this by
   always putting the facts sheet first and whole, where it is also a
   stable prefix and therefore cached. Chat takes the top three by
   similarity and does neither. That is the next thing to try.

0a. **Two answers were served with no training record, and the cause
   was the instrumentation work itself.** Messages 10 and 12 on
   production have no `decision_log` row and no `llm_usage` row.

   `run_id` on both tables is a foreign key to `jd_runs`. The fix that
   added a run id to chat answers, so the two rows could be joined,
   minted an id that matches no `jd_runs` row, so both inserts failed
   the constraint. Both writes are best-effort by design, so it
   surfaced as a log line nobody was reading.

   Chat no longer mints one; the column is nullable for exactly this
   case, and the two rows are joined through the message instead
   (`decision_log.ref_id` is the message, `llm_usage.ref_id` is its
   conversation, `chat_messages` carries both). Both failures now log
   at ERROR rather than WARN, because a lost training row is the thing
   this instrumentation exists to prevent.

   A unit test could not have caught it: the bug is a database
   constraint and a fake store has none. There is now an integration
   test against a real Postgres that inserts both rows, and
   reintroducing the run id makes it fail with the production error.

0. **The first real question on production failed, and why.** Roger
   asked one on 2026-09-30 and got "network error". The handler had
   worked: it logged status 200 after **44.9 seconds**. Go's
   `http.Server.WriteTimeout` is 30 seconds and covers the whole
   response, so the connection had been cut fourteen seconds before the
   answer was ready.

   Fixed by exempting only `SendMessage` from the deadline. Behind that
   sat a second bug of the same family as the flusher one: clearing a
   deadline goes through `http.ResponseController`, which walks
   `Unwrap()` to find a capable writer, and the logging wrapper had no
   `Unwrap`. Without it the exemption compiles, runs, reports nothing
   and changes nothing. There is now a test that fails with "feature
   not supported" if `Unwrap` is removed.

   **Forty-five seconds was model thrash, and it is fixed.** Both of
   Roger's questions took almost exactly the same time, 44.9 s and
   45.0 s, which ruled out the cold-cache explanation. The cause was
   `OLLAMA_MAX_LOADED_MODELS=1`, carried over from the CPX31 when the
   model was qwen3:8b. The chat pipeline embeds the question and then
   generates, so one slot meant every question evicted the 5 GB
   language model to load a 376 MB embedder and then reloaded it.

   Both models now sit resident at the app's own 8192 context:
   qwen3 at 5.2 GB, the embedder at 376 MB, **5.19 GiB of the 7 GiB
   cap**, and they survive an api restart.

   **The reload was not the expensive part, and I said it was.** The
   5 GB model comes back from page cache quickly. What eviction
   actually destroyed was qwen3's **KV cache**, and that is what cost
   the time, because the persona system prompt is byte-identical on
   every call and is exactly the thing a KV cache makes free.

   **It front-loads on every restart.** The scheduler fires each job
   once at start, so the warm begins within seconds of boot and the
   cold 78 seconds lands inside the deploy window rather than on a
   visitor: a deploy spends minutes pulling images, restarting and
   running live checks before anyone can arrive. Confirmed on
   production, first tick after a deploy 78.2 s, next tick 0.2 s.

   The tick runs every minute, because a warm tick costs 0.2 s and the
   interval is really a bound on how long someone can arrive to a cold
   cache after something evicted it.

   **It stands aside for a JD run.** There is one cache slot
   (`OLLAMA_NUM_PARALLEL=1`), so a run and the chat prefix cannot both
   be held. Warming through a run would evaluate two thousand tokens,
   be evicted by the run's next call, and do it again a minute later:
   CPU spent on a cache nothing will read, taken from the run that is
   actually working. Chat is slow during an evaluation either way; this
   stops it also making the evaluation slower.

   Measured on the box, warm, with a fixed system prompt and two
   *different* questions:

       question A   428 prompt tokens   prompt_eval  9.5 s
       question B   428 prompt tokens   prompt_eval  0.4 s

   The prefix caches across different questions. With one model slot
   the embedding call between two questions evicted qwen3 and threw
   that away, so every question paid full prompt evaluation. With two
   slots it survives.

### Where the seconds actually go

Measured on production, warm, 2026-10-01:

| | rate | cost |
|---|---|---|
| prompt evaluation, uncached | ~39 tok/s | 874 tokens = 22.3 s |
| prompt evaluation, cached prefix | — | 0.4 to 0.6 s |
| generation | ~8 tok/s | 120 tokens = 15.0 s |

So a first question after a quiet spell is roughly 1,200 prompt tokens
(31 s) plus a 90-token answer (11 s), which is the 45 s Roger saw.
A *following* question should pay only the varying part, the evidence
and the question, because the persona prefix is cached: expect
somewhere near 24 s.

**Generation is now the floor**, not retrieval.

The persona's answers at v1 were already running about 90 tokens, so
the old 150-word rule was never binding and shortening it is worth
about five seconds, not the fifteen the old limit implied. Taken
anyway, as **persona v2** (`v2:8a4c2f19`): rule 9 goes from 150 words
to 90 and `AskRogerAnswerMaxTokens` from 320 to 200. Five seconds off a
twenty-five second answer is worth having, and a limit that never binds
is not doing any work.

The other lever is showing fewer passages, `Show` 3 to 2, worth about
175 tokens or 4.5 s and costing grounding. Not taken: the two together
would bring a cached follow-up from roughly 25 s to 15 s, and the
retrieval one is the half that makes answers worse.

1. **`QAMatchThreshold` is back to 0.85, and the mechanism changed.**
   The 0.72 above was measured off one question family and it was
   wrong. A second probe over six real entries and thirty phrasings
   found the bands overlap completely:

       0.766  "do you have leadership experience"
              vs "How much experience do you have?"   DIFFERENT entries
       0.605  "do you have a masters"
              vs "What is your education?"            SAME entry
       0.475  "do you know Ignition"
              vs "What automation platforms ...?"     SAME entry

   `nomic-embed-text` is scoring shared vocabulary far more than shared
   intent on strings this short. "experience" on both sides carries
   0.766 between unrelated questions; "Ignition" and "automation
   platforms" share no words and score 0.475 despite being the same
   question. **No single threshold separates those lists**, so this
   stopped being a threshold problem.

   What changed: the threshold goes back above the worst observed
   cross-entry pair, and `QAMatchMargin` (0.05) additionally requires
   the winner to beat the best candidate from a *different* entry. The
   margin is scale-free and should outlive a change of embedding model,
   which a threshold will not.

   **What this means for the bank.** Matching runs over *every*
   phrasing and the best one wins, so each phrasing is its own target
   rather than a satellite of the canonical question. A phrasing that
   scores 0.36 against its own canonical is not dead weight: it covers
   a different vocabulary neighbourhood, and a visitor who asks in
   those words hits it at close to 1.0.

   So the rule for writing phrasings is the opposite of the obvious
   one. They should be **diverse**, not similar to each other: each one
   buys coverage around its own wording, and two phrasings that are
   near-identical buy the same ground twice. What the bank cannot do is
   bridge to vocabulary nobody listed, and a question asked in
   unanticipated words falls through to the model, which is slow and
   correct, rather than matching the wrong entry, which is fast and
   wrong.

   (An earlier version of this section said those distant variants
   "will never fire". That was wrong, and wrong in a way that would
   have led to writing worse phrasings.)

   *Superseded, for history:* 
   The original 0.85 was reasoning rather than measurement, and the
   first real measurement killed it. Taken on production against
   `nomic-embed-text` on 2026-09-30, with one banked question and
   seven probes written as disabled entries, which can never be
   matched or served, and deleted afterwards:

       1.0000  Are you open to relocating?        the anchor
       0.8824  are you willing to relocate        paraphrase
       0.7941  would you move?                    paraphrase
       0.7627  can you relocate for this role     paraphrase
       0.5453  where are you based?               same topic, different question
       0.4351  what PLC platforms have you used?  unrelated
       0.4248  what is your favourite pizza       unrelated
       0.3657  do you know Rockwell ControlLogix  unrelated

   At 0.85 only the closest paraphrase matched. "would you move?"
   would have gone to the model for fifteen to twenty-five seconds,
   which is the exact wait the bank exists to avoid, on a question it
   already had an answer to. The strictness was chosen because a false
   match is worse than a miss, and that reasoning was sound; the
   number was simply in the wrong place.

   The useful finding is the gap: real paraphrases bottom out around
   0.76, the nearest non-match sits at 0.55, so anything from 0.6 to
   0.75 separates them cleanly. 0.72 takes all three paraphrases with
   0.04 to spare and clears the nearest non-match by 0.17.

   Still provisional: this is one question family, and a threshold
   generalised from one anchor is a guess with better manners. It was
   changed anyway because 0.85 was demonstrably wrong rather than
   merely unverified. FR-CHAT-15's golden set should settle it across
   many questions.

   The bank is still empty, so it does not fire at all yet.
2. **FR-CHAT-11 is not met and will not be.** 2 s p50 to first token
   against 11 to 25 s measured. The requirement should be amended to
   match the hardware, or the hosted provider in §3 reopened.
3. **Server streaming was broken until 2026-09-30.** The logging
   middleware wrapped the `ResponseWriter` in a type that embedded the
   interface, which gave it the interface's methods and none of the
   optional ones, so `http.Flusher` was silently removed. connect-go
   checks for it on a streaming handler and refused every call with
   `*server.recorder does not implement http.Flusher`. `SendMessage`
   is the only streaming RPC, so this was the whole assistant failing
   with an internal error while every unary RPC on the same mux worked.

   It compiled, it passed the Go tests and it passed the browser build.
   It was found by sending a real Connect stream frame at a running
   server, and there is now a type assertion in
   `internal/server/recorder_test.go` so it cannot come back quietly.

4. **Nothing streams yet.** The sidecar gateway is single-shot, so
   `first_token_ms` currently records the whole model call rather than
   the first token. It is written that way rather than left at zero,
   which would read as "instant" in the metrics, but it is not yet
   measuring what it is named for. Streaming needs a new sidecar RPC;
   the seam is `chat.Generator`.
5. **The persona has no schema.** It is the only prompt in the registry
   without one, because it streams prose. The grammar-level guarantee
   that stopped the judge runaway does not exist here; rules 9 and
   `AskRogerAnswerMaxTokens` are all that bound an answer, and a
   response that hits the cap stops mid-sentence.
6. **`Fingerprints()` now includes the persona in every JD run record.**
   Noise in that record, kept deliberately: it is a snapshot of what
   every prompt was, not a list of which ran.
7. **The local dev database is fixed.** It was stuck at migration 31
   with migration 32 failing against an older view shape. Resolved by
   dropping the derived views and re-applying migration 29's
   definitions by hand, then letting goose replay 31 to 48. Only views
   were touched, so no data was lost. Recorded here because the same
   drift will bite anyone who restores an old dump.

   *Previously:* the local dev database had drifted from the migration
   history.
   `goose_db_version` was at 30 while migration 31's views already
   existed, and migration 32 fails against the older view shape. Worked
   around by testing against a fresh `career_test` database. **The main
   local database is still wedged**; production is unaffected and is on
   44. Recreating the local database is the fix and it destroys local
   dev data, so it is the owner's call.
8. **Roger has not opened any of it.** The surfaces exist and have
   been exercised locally, not reviewed. Per the
   owner's standing rule, no UI work counts as complete until it is
   deployed, opened, and reviewed with him.

---

### What the local live test could and could not show

Run on 2026-09-30 against the local stack, with a throwaway admin
account that was removed afterwards.

**Verified working:** the page renders; an entry saves with its
canonical phrasing plus two variants; an off-site source path is
refused with the reason shown; the awaiting-embedding banner appears
and then clears; the embedding job picks the phrasings up and stores
vectors; the nav entry appears; the count line reads correctly.

**Could not be verified locally:** whether the bank *matches sensibly*.
`SIDECAR_EMBED_PROVIDER=stub` on the local stack, so the vectors are
placeholders, and two phrasings of the same question sit at a cosine
similarity of about zero rather than the 0.8 to 0.9 real embeddings
would give. The plumbing is proven; `QAMatchThreshold` can only be
calibrated on the box with `nomic-embed-text`.

**Three bugs it found that nothing else would have:**

1. Four `admin.qa_entry_*` events were emitted but never registered, so
   they were refused and logged as a warning nobody would read. Fixed,
   and a test now parses this package's `requestEvent` calls and checks
   every name and prop against the registry. That test immediately
   found two more that had been missing far longer: `jd.quota_blocked`
   and `admin.jd_limit_changed`.
2. `admin.decision_reviewed` was given `kind`, `corrected` and
   `dimensions` in the grading work above, precisely so the growth of
   the training set could be read off the event stream. Props outside a
   spec's list are dropped with no warning at all, so all three were
   being discarded.
3. **The embedding job was wired inside the generation-provider
   check.** The bank is what answers when generation is unavailable
   (FR-CHAT-17), so tying its embeddings to generation meant the
   degrade path went dark exactly when it was needed: no model,
   therefore no embedding, therefore no bank, therefore no answer at
   all. Confirmed on the local stack, where generation is stubbed and
   the job silently did nothing.

### Answering about the site itself

Asked "how do I upload a JD on this site", the assistant said it had
nothing in its records. It was right: nothing in the corpus described
the site. `apps/web/content/other/career-site-guide.md` is that
document, ingested as a public corpus entry of kind `other`. It
describes what each page does, what members can do, and what the
assistant will not discuss.

It is deliberately not an article, so no page is published for it.

**It did need a code change, contrary to what this document first
said.** The *private* reindex treats any directory named after a known
kind as that kind; the *public* one walks an explicit map,
`publicCorpusSubdirs`, so a folder not named there is never read. The
first attempt added the file, reindexed, and the job reported "kinds
article" and ingested nothing. `other` is now mapped, and a test walks
the committed content tree and fails on any directory with no entry.

**It only enters the corpus on a reindex**, which is "Reindex public
content" on `/admin/corpus`.

### Citation links went nowhere

`corpus_documents.source_path` is the file the walker read, so a
citation linked to `better-business-decisions-part-1.md` and **every
citation on every answer 404'd**. `sitePath()` now maps a source to a
real URL (`/articles/<slug>`) and returns empty for kinds with no page,
which the surface already renders as a title without a link. A citation
that cannot be followed is worse than none: it invites the one reader
who checks to conclude the source was invented.

### Actions, as D-25 allows (persona v3)

The assistant may propose three things and may do none of them:
`open_scheduler`, `open_contact_form` with a category, and
`open_contributor_request` for a repository.

It writes a marker, code validates it against the allowlist, and the
surface renders a link the member presses. **Nothing calls an API.**
That is the sentence D-25 ends on: no path from a prompt injection to
an action taken. The corpus is the owner's own, but it is still
untrusted input to a model, and a passage saying "book a meeting" must
not book a meeting.

A marker rather than tool calling, for two reasons. The gateway is
single-shot by design, so a tool-calling loop would be a second
architecture for one feature. And a marker survives streaming, which
structured output does not. It is also the pattern already proven here:
citations are emitted by the model, validated in code, and dropped when
they point at nothing. One mechanism, used twice.

The card is a link, not a fetch, and nothing is pre-filled beyond a
category. The injection cases in the golden set include pre-filling a
contact form with content the member did not write, and a link to a
blank form cannot do that.

Dropped by the allowlist and covered by tests: invented actions, an
action name that looks like an API call, an unknown contact category, a
path-traversal repository name, a URL as a repository name, and a
marker injected through retrieved content.

`proposed_action` is recorded on the decision row, because "what does it
try to make people do" is a question about the assistant that nothing
else in the row answers.

### Admin database queries, chosen from a list

Roger asked for the assistant to be able to query the database, then
for the admin to choose the query from a dropdown. The second
instruction is what makes it both safe and fast.

- **The model never writes SQL.** Every statement is fixed in
  `users/admin_queries.go` and takes no argument from anywhere. There
  is no string being built, so there is nothing to inject into.
- **The model never chooses.** A person picks from the list, which
  removes the classification step. That step would have cost a second
  model call, about ten seconds on this box, and would have been the
  least reliable part of the feature.
- **No model is involved at all.** The admin picks, a fixed statement
  runs, and the number comes back exact in milliseconds. Asking a
  language model for a count would take twenty to thirty seconds and
  could get it wrong.
- **Admin only**, enforced at the RPC, and the component renders
  nothing for anyone else.

**Counts, never rows.** Nothing selects a name, an address or a message
body. Results are rendered into a surface and may be quoted into a
prompt, prompts are written to `decision_log`, and that is exported as
training data, so anything selected here would end up in a file whose
purpose is to be handed to a trainer.

**Rows and columns, not sentences.** The first version returned one
concatenated line per query, "5 total, 4 active, 0 awaiting approval".
It worked, and it was the wrong shape: Roger asked for a table, and he
was right, because a count per line is read by scanning a column while
a sentence has to be parsed by eye every time. Each query now returns
whatever columns it naturally has and the surface draws a table.
Column headers come from the statement itself, so a query and its
header cannot drift apart.

Ten queries: members, JD submissions by day, Ask Roger usage, how
answers were produced (with average seconds and output tokens), the
Q&A bank, the corpus by visibility, the corpus by document kind,
meetings, model usage over seven days, and the review queue. Adding
one is an edit in that file and nowhere else.

A test runs every query against a database with the full migration
history, which caught two written against a schema that had moved: a
`user_status` enum with no `pending` value, and a `meetings` table that
is called `meeting_bookings`.

## 7. Waiting on the owner

1. **Should Whiskey House / MDEMG material inform the assistant?**
   About 10 corpus documents. It is one `UPDATE` setting
   `chatbot_include = false`. `chatbot_include` defaults to **true**
   (migration 00046 explains why against the requirement's allow-list
   phrasing), so today the answer is "yes" by default.
2. **FR-CHAT-11 and FR-CHAT-16**, per §3 and §6.2.

Two items that stood here on 2026-09-30 are closed. *Deploy migrations
00045 to 00048* is done, production and local dev are both at 48. *Seed
the Q&A bank* is done: 13 entries, all approved by the owner in his own
words, 64 phrasings, all embedded.

---

## 8. Next steps, in order

Everything structural is built. What is left divides into making what
exists work better, and the requirements still unimplemented.

**Making it work better, highest value first.**

1. **Grade more answers.** 51 of 1,743 decision rows are graded, which
   is 2.9%. The console, the rubric and the correction box all work and
   have been used; the limit now is volume, and only the owner can
   supply it. Of the 51, none carries a corrected answer, so the set can
   be counted but not yet learned from.
2. **Add the phrasings the data is already naming.** `chat.answered`
   records `miss_reason` and `best_similarity` on every model-path
   answer, so "which phrasing to add next" is one ordered query rather
   than a hunt. This was proved the hard way: a real question missed by
   0.0134 because a member typed `ci-cap` for `hi-cap`, and finding it
   took a manual embedding comparison because the near-miss score was
   being recorded as zero.
3. **Split the site guide.** Three chunks for the whole site means a
   question about the scheduler and one about JD upload compete for the
   same passage. One document per feature would sharpen retrieval more
   than any tuning.

Two entries that stood here are done. *Seed the Q&A bank* is item 1
above, now closed. *Put the career facts sheet in every prompt* shipped,
and with the one-minute warmer holding it in Ollama's KV cache a cold
82.5 s prompt evaluation became 0.8 s; the ticks sit at about 0.2 s.

**Still unimplemented.**

4. Real streaming: a sidecar streaming RPC behind `chat.Generator`.
   Would not make an answer faster, but would make the wait legible.
   FR-CHAT-11's 2 s first token is not reachable on this hardware and
   the FSD now says so rather than leaving it open.
5. Quotas and budget cap (FR-CHAT-12), and `GetQuota`. `llm_usage` has
   chat rows, so the cap has something to read.
6. Escalation (FR-CHAT-10). FR-ADM-04, replying to escalations from the
   console, waits on this and nothing else.
7. The golden set (FR-CHAT-15), which then settles the Q&A threshold
   properly rather than from one question family.
8. Member answer rating (FR-CHAT-14). Worth noting it is a dependency,
   not a nicety: FR-ADM-03's review queue is specified as
   negative-feedback driven and cannot be until this exists.

---

## Related

- [`FSD.md`](FSD.md) §5.5, the requirements
- [`decision-log.md`](decision-log.md), the table and export format
- [`llm-tuning-log.md`](llm-tuning-log.md), measurements and runs
- [`in-flight.md`](in-flight.md), what is underway across the repo
- [`backlog.md`](backlog.md), understood and not started
