# Ask Roger

The state of Phase 4, written to be read cold. What exists, what does
not, what is known to be wrong, and what is waiting on a decision.

**Last updated 2026-09-30.** Branch `claude_dev01`, head `d16f429`, PR #217.
**Deployed to production on 2026-09-30 as `7ca0e52c43bf`**, database at
migration 48.

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
| `ChatService` RPCs | **Built and mounted**, 6 of 9 | `handlers/chat.go`, `server/server.go` |
| Conversation store | **Built, tested, not deployed** | migration `00045`, `users/chat.go` |
| Retrieval path for chat | **Built, tested, not deployed** | `users/chat_retrieval.go`, migration `00046` |
| Persona prompt v1 | **Built, tested, not deployed** | `prompts/askroger.go` |
| Q&A bank | **Built, tested, not deployed** | migration `00047`, `users/qa.go` |
| Decision capture for answers | **Built, not deployed** | migration `00048`, `users/chat_decision.go` |
| Grading vocabulary and rubric | **Built, not deployed** | `users/decision_log.go` |
| Answer pipeline | **Built, tested, wired** | `internal/chat/answer.go` |
| Streaming transport | Stream is real, **one delta** until the sidecar streams | `handlers/chat.go` |
| Escalation, quotas | **Refused, not stubbed** | `Escalate`, `GetQuota` |
| `/ask` page and side panel | **Not built** | — |
| Admin grading console for chat | **Built, never seen live** | `/admin/decisions` |
| Q&A bank admin surface | **Built, live tested locally** | `/admin/qa`, `handlers/admin_qa.go` |
| Phrasing embedding job | **Built, tested, scheduled** | `chat/qaembed.go`, every 5 min |
| Golden set (FR-CHAT-15) | **Not built** | — |

Migrations 00045 to 00048 are committed and have been applied to a
clean database locally. **They have not been applied to production.**

Prompt fingerprints at this head:

    ask_roger_persona  v1:fb7c9aac
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

    grounded    citations    voice    scope    length

Grounding at 90 % and citation validity at 95 % are stated acceptance
criteria and neither can be computed from one overall verdict.

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

### Export

`ExportDecisionLog` (admin, MFA-fresh) emits JSON Lines. Each line
carries the prompt, the response, the input and output jsonb, usage
including `first_token_ms`, and the `human` block with verdict, note,
**answer** and **dimensions**. Format in
[`decision-log.md`](decision-log.md).

---

## 6. Known issues and things that are not right yet

1. **`QAMatchThreshold` is 0.72, measured, and still provisional.**
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
3. **Nothing streams.** The sidecar gateway is single-shot, so
   `first_token_ms` currently records the whole model call rather than
   the first token. It is written that way rather than left at zero,
   which would read as "instant" in the metrics, but it is not yet
   measuring what it is named for. Streaming needs a new sidecar RPC;
   the seam is `chat.Generator`.
4. **The persona has no schema.** It is the only prompt in the registry
   without one, because it streams prose. The grammar-level guarantee
   that stopped the judge runaway does not exist here; rules 9 and
   `AskRogerAnswerMaxTokens` are all that bound an answer, and a
   response that hits the cap stops mid-sentence.
5. **`Fingerprints()` now includes the persona in every JD run record.**
   Noise in that record, kept deliberately: it is a snapshot of what
   every prompt was, not a list of which ran.
6. **The local dev database is fixed.** It was stuck at migration 31
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
7. **Nothing here has been seen by a human.** No UI exists yet. Per the
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

## 7. Waiting on the owner

1. **Should Whiskey House / MDEMG material inform the assistant?**
   About 10 corpus documents. It is one `UPDATE` setting
   `chatbot_include = false`. `chatbot_include` defaults to **true**
   (migration 00046 explains why against the requirement's allow-list
   phrasing), so today the answer is "yes" by default.
2. **Deploy migrations 00045 to 00048?** They are additive and safe:
   three new tables, one new column on `corpus_documents`, three on
   `decision_log`. Nothing reads them yet.
3. **Seed the Q&A bank** at `/admin/qa`, now that there is somewhere to
   put entries. The bank is the fast path and it is empty. The
   FSD (§ corpus inventory) names the interview-prep material as the
   strongest seed, after removing employer-specific and compensation
   content. Entries need his words, not generated ones: that is the
   property that makes the bank usable for restricted topics.
4. **FR-CHAT-11 and FR-CHAT-16**, per §3 and §6.2.

---

## 8. Next steps, in order

1. `/ask` and the side panel. **Live test and UI/UX review before this
   is called done.**
3. Real streaming: a sidecar streaming RPC behind `chat.Generator`.
4. Quotas and budget cap (FR-CHAT-12), and `GetQuota`.
5. Escalation (FR-CHAT-10).
6. The golden set (FR-CHAT-15), which then calibrates §6.1.

---

## Related

- [`FSD.md`](FSD.md) §5.5, the requirements
- [`decision-log.md`](decision-log.md), the table and export format
- [`llm-tuning-log.md`](llm-tuning-log.md), measurements and runs
- [`in-flight.md`](in-flight.md), what is underway across the repo
- [`backlog.md`](backlog.md), understood and not started
