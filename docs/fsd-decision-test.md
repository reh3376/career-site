# FSD: the decision test

**Status: design settled, not built.** Every open decision has been answered by the owner. What remains before a sprint plan is the thirty item texts, the data model, and the roadmap.

A supplement to the *Better Business Decisions Require Better
Information* series (`/articles/better-business-decisions-part-1` to
`-part-5`). Open to members and to anonymous visitors.

---

## 1. What it is for

The owner's research question, in his words:

> How much effort is required to get a given person to make obviously wrong decisions in a manner that makes the person believe they are confidently correct.

Three claims sit behind it:

1. Attention and intuitive decision quality are related.
2. A person's capacity for effortful thinking is finite.
3. When effort runs short, a fast intuitive answer becomes the **default**, taken without regard to available data.

The test exists to collect data on that, and secondarily to let a reader experience it on themselves rather than read about it.

### 1.1 The distinction that decides the instrument

"Finite attention" has two readings and they have very different evidential standing.

**Sequential depletion (ego depletion).** Effortful task A leaves you worse at later task B. This is the famous version and it largely failed to replicate; the multi-lab registered replications found effects nearzero.
**Not used here.** A test built on it would be the easiest possible thing for a knowledgeable reader to dismiss, on a page whose whole argument is about evidence quality.

**Concurrent load.** Working memory is occupied *at the moment of decision*. This replicates well, and De Neys ran essentially the owner's hypothesis: holding a digit string while answering base-rate and conjunction problems reliably pushes people onto the heuristic answer.

So the test **occupies** attention during each decision rather than trying to tire the participant out across a session. Shorter, better evidenced, and it measures the thing actually described above.

This is the single most important design decision in the document and it is why the test looks the way it does.

## 2. What is measured

Per trial, four values:

| Value | How |
|---|---|
| Answer | correct, **lure** (the intuitive wrong answer), or other |
| Latency | ms from item shown to answer committed |
| Confidence | the participant's own rating, collected every trial |
| Load | which load condition the trial ran under |

Confidence on every trial is not optional decoration. Error rate alone answers "does load make people wrong". The owner's question is about being wrong *and sure*, which needs confidence on the same trial.

### 2.1 The headline measure

**The confidence–accuracy gap under load.** If accuracy falls sharply while confidence barely moves, that gap is the finding. It is also the
number worth showing a participant about themselves:

    unloaded    accuracy 78%   mean confidence 74%
    loaded      accuracy 41%   mean confidence 71%

Accuracy fell 37 points, confidence fell 3. That is the result stated in a way a reader remembers, and it is exactly the owner's question answered for one person.

**DECIDED: the emailed result carries the score and the confidence gap.** For a participant who gave an address:

    24 of 30 correct

    early blocks     83% accurate   79% confident
    hardest block    50% accurate   76% confident

Accuracy moved 33 points, confidence moved 3. Nothing identifies an item, so §2.4 holds and no answer leaks.

The Brier score is computed and stored but not sent. It is the more rigorous statement of the same thing and it needs a paragraph of explanation that most readers will skip, on a page where the plain comparison already lands.

A percentile against other participants was rejected. It would be compelling and it turns a demonstration into a ranking, which contradicts §7's rule that the result is never a verdict on the person.

### 2.2 The per-person threshold

The question says *a given person*, which means a threshold rather than a group average: the load level at which this participant starts answering with the lure while still reporting high confidence.

**DECIDED: staircase.** Load escalates as the test progresses, seeking the point where this participant starts answering with the lure while
still reporting high confidence.

### 2.3 The confound a rising ramp creates, and the fix:

A load level that only ever rises is confounded with time-on-task. Trial 20 differs from trial 2 in two ways at once: more load, and fifteen minutes of elapsed effort. When accuracy falls, nothing in the data says which caused it, and "they were tired by the end" is exactly the sequential-depletion account §1.1 excluded on evidential grounds. A monotonic ramp lets it back in through the running order.

**Low-load probe trials are scattered throughout, including late ones.**
They are the control. A late probe answered as well as an early probe rules out fatigue and leaves load as the explanation. Late probes that degrade too are themselves a finding, about the fifteen minutes rather than about load, and worth having rather than being unable to tell.

Probes are not optional and are not decoration. Without them the headline measure in §2.1 cannot be attributed to anything.

**RESOLVED by the block design.** Load is carried per block, not per trial, so there is no unloaded trial to scatter. The control is block 5 (§3.3), which returns to block 1's difficulty at the end of the test.

## 3. How load is applied

**DECIDED by the owner.** Load is carried at the **block** level: a number is shown, memorised in two seconds, held across a run of questions, then recalled. Four blocks, each a step up:

| Block | Memorise | Hold across | Recall as |
|---|---|---|---|
| 1 | 3 digits, 2 seconds | X questions | as shown |
| 2 | 4 digits, 2 seconds | Y questions | as shown |
| 3 | 4 digits, 2 seconds | Z questions | **each digit +1** (1234 to 2345) |
| 4 | 4 digits, 2 seconds | ? questions | **each digit +3** (1234 to 4567) |

The escalation is well formed, and blocks 3 and 4 are the strongest part of it. Blocks 1 and 2 raise **storage**, which is the cheaper kind of working-memory demand and saturates quickly. Blocks 3 and 4 add **transformation**, which recruits executive processing rather than storage, and is a far steeper step than a fifth or sixth digit would have been.

**DECIDED: six questions per block, five blocks, thirty questions total.** X = Y = Z = 6, and block 5 (§3.3) carries six as well.

### 3.1 The strategy problem, and how it is handled

A participant told at the start of block 3 that they will recall each digit plus one can do it two ways, and the two produce different loads during the questions:

- **Transform at encoding.** Convert 1234 to 2345 immediately, then hold 2345 like any other number. The effort spikes in the two-second window and the load during the questions is the same as block 2.
- **Hold and transform at recall.** Keep 1234, carry a pending operation through the questions, do the arithmetic at the end.

Both are reasonable, nobody will be told which to use, and **different participants will pick differently**. That matters because it means the load during the decisions, which is where the effect has to appear, is not fully controlled by the block.

Two mitigations, both cheap:

**Ask afterwards.** One question in the debrief: did you convert the number straight away, or remember it and convert at the end? That turns an uncontrolled variable into a recorded one, and the analysis can split on it.

**Two seconds is probably too short to pre-compute four digits**, especially at +3, which pushes most people toward holding raw and transforming at recall. That is the condition the design wants. It is a reasonable expectation rather than a guarantee, which is exactly why the debrief question exists.

### 3.2 Scoring the recall, which has two failure modes now

With a transformation, a wrong recall is no longer one kind of event:

| Recall | Means |
|---|---|
| Correct transformed value | held it and operated on it |
| Correct digits, untransformed (1234 for 2345) | **storage held, executive failed** |
| Wrong digits | storage failed |
| Partially right | scored per digit |

The second row is the interesting one and it would be invisible if recall were scored pass/fail. It says the number survived but the operation did not, which is the clearest single signal that the executive was saturated, and it is the mechanism the whole test is about.

### 3.3 The baseline block, which the ramp needs

§2.3 requires low-load controls scattered late, and this block design cannot deliver them trial by trial: within a block every question carries the same number, so there is no such thing as an unloaded trial inside a block.

**The fix is a fifth block that returns to block 1's difficulty**: 3 digits, no transformation, placed last.

    1 (3 digits) → 2 (4 digits) → 3 (4 digits +1) → 4 (4 digits +3) → 5 (3 digits, as block 1)

Block 5 against block 1 is the whole control. Same difficulty, twelve minutes apart. If accuracy and confidence match, fatigue is ruled out and the decline across blocks 2 to 4 is load. If block 5 is worse than block 1, the fifteen minutes are doing some of the work, which is worth knowing and is currently unknowable.

Without it the ramp is monotonic and load is perfectly confounded with time-on-task, which is the sequential-depletion account §1.1 excluded on evidential grounds. **DECIDED: block 5 is the control**, confirmed by the owner, and it is what the thirty-question total already assumed: six per block across five blocks.

**The recall step is what makes the load real.** Without being asked to reproduce the number, a participant can simply drop it and the block measures nothing.

**DECIDED: a missed recall is a data point, not a failure.** The block's six answers are kept and scored normally, with the recall stored by failure type (§3.2). In the owner's words, the miss on a memorised number is not a fail, it is simply a data point in the overall test.

Discarding the block would have deleted blocks 3 and 4 for exactly the participants the test is about, which is where the data would otherwise be richest.

### 2.4 The answer key exists and the participant never sees it

**The key is essential.** Grading, scoring, the lure classification, and every analysis in this document depend on knowing the right answer to each of the thirty items. It is authored with the items and stored with them.

**DECIDED: a participant is never shown the correct answer to any individual question.** Not during the test, not on the thank-you screen, not in the emailed results, not on the explanation page. What they get is an overall score in the form *N of 30 correct*, and nothing item by item.

The reason is that the test is standardized (§4): every participant meets the same thirty items in the same order. An answer key that escapes contaminates the instrument permanently, and it only has to escape once. A participant who learns item 7's answer and mentions it to a colleague has not spoiled their own run, which is already recorded, but the colleague's, and everyone after. With a fixed item set there is no recovery from that short of writing thirty new items.

This constrains the debrief rather than cancelling it. See §7.1.

### 2.5 Where the key lives, which is an architecture decision

"Never shown" has to be enforced by where the key is, not by what the interface chooses to render. A key that reaches the browser has been published, whatever the page does with it: anyone can read a JavaScript bundle, a network response, or a React prop, and with a standardized item set it only has to happen once.

So:

- **The answer key never leaves the server.** Not in the page payload, not in a bundle, not in a preloaded JSON blob, not as a hashed value the client could test guesses against.
- **The client is sent the item and its options, never which option is correct.** Options are not the key; which one is right is.
- **Grading is server-side.** The browser posts the chosen answer and receives an acknowledgement, not a verdict. It never learns correct, lure or other for any question.
- **The running total is server-side too.** A score accumulating in the browser is a score the participant can read.

This is also what makes "no correctness feedback during the test" (§3.4) a property of the system rather than a UI convention. The browser cannot leak what it was never told, and a future change to the run screen cannot accidentally reveal it.

It costs one round trip per answer, which is affordable inside a 20.5-second question, and the posts were happening anyway so that a partial session is still data (§5b).

### 3.4 No correctness feedback during the test

Nothing in the test tells a participant whether any answer was right: not the questions, and not the recall. The entry is accepted and the test moves on.

This is not politeness, it is a requirement of the measurement. **Confidence is a dependent variable** (§2), and telling someone they just got one wrong changes their confidence on the next one. A test that gave feedback would be measuring a participant's reaction to being corrected, which drifts further from the baseline with every trial, rather than their native confidence under load.

It also matters for the recall specifically. Being told "wrong" after holding a number through six questions reads as failure however it is worded, and a participant who feels they are failing behaves differently for the remaining blocks.

All feedback is deferred to the explanation (§7.1) and, for those who gave an address, the emailed results.

**SUPERSEDED.** Time pressure is not a separate manipulation: the owner's escalation (§3) carries the load, and every phase is time-controlled with expiry already specified. Adding a countdown on top would confound haste with substitution, which is the distinction the test exists to make.

## 4. The items

Each item needs a compelling wrong answer that arrives fast, and a correct answer that requires deliberation. Three families, all with decent replication records: base-rate neglect, conjunction, and CRT-style arithmetic traps.

**The classic CRT items cannot be used.** Bat-and-ball, widgets and lily pads are so widely circulated that a meaningful share of this audience has met them. They would measure prior exposure, not reflection. Novel items with the same structure are required, and item design is real work rather than a lookup.

**DECIDED: the test is standardized.** Every participant sees the same items, in the same order, with the same wording. No randomisation, no per-person item selection.

That is the right call for this question. The comparison that matters is *within* a person, across blocks, and a fixed order means no participant got an easier draw than another. It also makes the instrument reproducible: a result can be checked against the exact sequence that produced it.

Two consequences to carry rather than discover:

**Item difficulty and position are confounded.** With a fixed order, nothing separates "item 7 is hard" from "position 7 is hard". For comparing people that does not matter, since everyone met item 7 in position 7. For saying anything about a particular item it does, and the document should not later claim per-item difficulty as if it were measured.

**The block 1 against block 5 control (§3.3) compares different items.** Block 5 cannot reuse block 1's items, because a participant who has already met them is no longer naive. So the control rests on the two blocks being *matched* in difficulty rather than identical, and matching is a judgement until there is pilot data. Block 5's items should mirror block 1's in type and proportion, and the limitation belongs in any write-up: a block 5 drop is fatigue **or** a harder item set, and the first run cannot fully separate them.

**DECIDED: no domain-specific items.** Generic throughout. Domain scenarios would have let an expert bypass the lure through knowledge rather than reflection, which is a confound dressed as personalisation. The cost is that the page looks less like this site; the benefit is that a plant engineer and an accountant are answering the same instrument.

## 4b. Timing and the metronome

**DECIDED: 15 minutes maximum, every portion time-controlled, and a one-per-second tick audible throughout.**

### Budget

Fifteen minutes is the ceiling for the whole visit, not for the questions. Worked through, with thirty questions across five blocks:

    total                 900s   (15 min)
      intake               90s   explainer, the sound question, the form
      instructions         45s
      practice block       60s   short, carries the tick
      thank you            20s
      overhead            215s
    scored content        685s
      block fixed cost     70s   5 x (2s memorise + 12s recall and transition)
      left for questions  615s
      questions            30    5 blocks x 6
      PER QUESTION       20.5s   read, decide, rate confidence

Each phase has a hard limit rather than a nominal one, so the test ends at a known time for every participant and duration never becomes a variable the analysis has to carry.

### What 20 seconds per question forces

**Items must be short.** Twenty and a half seconds covers reading the item, deciding, and rating confidence. A base-rate problem with a paragraph of setup does not fit; two or three lines does. This is now a hard constraint on item writing (§4) rather than a style preference, and an item that cannot be read and answered in that window is the wrong item regardless of how good the trap is.

**DECIDED: a question that runs out of time is recorded as `expired`,** its own outcome alongside correct, lure and other. The test moves straight to the next question. No going back, no pause, no explanation offered at the time.

That is the right call because a timeout under load is a result rather than missing data, and it is the one outcome that cannot be reconstructed if it is dropped or merged into error. Expect expiries to cluster in blocks 3 and 4; where they cluster is itself a measurement of where the executive gave out.

**The participant is told this before the test starts** (§5b), in the instructions rather than discovered on question nine. Someone who does not know a question can expire will keep working on one that has already gone, and then answer the next one in a state the design never intended. Knowing the rule is what makes moving on feel like the test working rather than the site breaking.

### The tick is the constant, not the ramp:

**One tick per second, unchanging, for the whole test.** It is the baseline, and it is explicitly *not* how effort is escalated; that mechanism is specified separately. The participant is told to connect speakers or headphones and turn the volume up **before** the test starts, and is asked whether they can use sound at all; both are specified in §5b under "Before the start button", because an instruction given after the start button is an instruction given too late.

Holding it constant is what makes it useful, but it only works if the user hears it. A tick that varied with load would be part of the manipulation and impossible to separate from it. Identical in every trial and every condition, it cannot differ between the things being compared, so it drops out of the analysis as a controlled variable rather than entering it as a confound.

What it does buy: time is continuously salient, pacing is identical for everyone, and the auditory channel carries the same occupancy throughout, so a quiet trial and a loaded trial differ only in the thing under test.

Three implementation consequences follow:

**It is disclosed before it starts.** A participant who is surprised by it is a participant who abandons, and an unexpected noise is a worse experience than a described one.

**Implementation is Web Audio, synthesised, scheduled ahead.** Not `setInterval`, which drifts audibly over fifteen minutes and would make the tick arrive at a different cadence for different participants, and not an audio file, which is an asset request the CSP would have to allow and a download that can fail mid-test. A synthesised click scheduled on the audio clock is sample-accurate and loads nothing.

**It needs a user gesture to begin.** Browsers refuse audio that starts without one. The "start the test" button is that gesture, so audio cannot be verified before the participant commits. **DECIDED: the practice block carries the tick** and is followed by the did-you-hear-it question (§5b), which is the earliest point at which hearing it can be observed rather than assumed.

### Who cannot hear it

A deaf or hard-of-hearing participant, or anyone who cannot have soundon, is in a different condition from everyone else. That is a real limitation and the options are not equally good:

- **Record it.** Whether audio was on becomes a stored variable and the analysis can split on it. Honest, cheap, and it does not pretend the
  problem is solved.
- **Visual equivalent.** A pulsing indicator at the same cadence. It substitutes one channel for another rather than matching the load, and a visual tick competes with reading the item in a way a sound does not.

**DECIDED.** The participant is asked, before starting, whether they can use sound. If they cannot, for any reason and without having to give one, the metronome becomes a visual beat: a heartbeat icon pulsing once per second. Which mode they ran in is stored on the session.

The two are not equivalent and the document does not claim they are. A sound occupies a channel the task does not use; a pulsing icon competes with reading the item. The visual mode exists so that someone who cannot hear can take part at all, and the analysis keeps the two groups separable rather than pooling them and hoping.

## 5. What the participant gives us

A form before the test, **every field optional**:

| Field | Shape |
|---|---|
| Name | free text |
| Age range | dropdown |
| Highest education completed | dropdown: GED, High School, Skilled Trades Apprenticeship, Some College, Associate Degree, Bachelor's Degree, Master's Degree, PhD, Multiple Degrees |
| Occupation | free text |
| Email address | free text |

Three things follow from this that are not optional:

**It is personal data and the privacy policy covers it in the same PR.**
Name plus email plus occupation plus age range identifies a person. The meeting scheduler shipped a week ahead of its privacy disclosure and that is not repeated here.

**DECIDED, what the email is for.** Four uses, and the privacy policy states all four because that is the commitment being made:

1. Sending the participant their results, **only if they ask for them**.
2. Follow-up contact.
3. A thank-you for taking part.
4. Identifying someone who takes the test more than once.

The fourth is the one worth noticing: it makes the address an **identifier**, not just a contact. That is legitimate and it has to be disclosed in those terms rather than buried as "we may contact you".

**Optional fields bias the sample.** Whoever fills them in is not a random subset. That does not make them useless, it means analysis reports completion rates alongside any demographic split. Worth knowing before anyone reads a bar chart of education against accuracy.

## 5b. Pages and layout

### Routes

| Route | Chrome | What |
|---|---|---|
| `/decision-test` | normal site | explainer, intake form, start button |
| `/decision-test/run` | **none** | the test itself |
| `/decision-test/thanks` | normal site | thank you, then back to the home page |
| `/decision-test/about` | normal site | what the test measured and why each item works |

**DECIDED: there is no per-participant results screen.** On completion the participant is thanked and returned to `rogerhenley.dev`. All follow-up, including results for anyone who asked for them, happens by email.

That removes `/decision-test/results/<id>` and with it the question of whether a URL carrying one person's numbers should be indexable. Simpler, and nothing personal is ever served over the web.

All three are public and therefore go in `PUBLIC_PATHS` in
`apps/web/src/lib/public-routes.ts`, which is the one table the proxy,
`robots.ts` and `sitemap.ts` all read. A member-only test would defeat
the point: anonymous reach is the data.


### Before the start button

Everything the participant must know or do has to happen on `/decision-test`, before they commit. An instruction given after the start button is an instruction given too late, and the test cannot be paused to deliver it.

**The order matters and is itself a decision.** The warning comes before the form, not after it.

1. **The effort warning, first, before anything is collected.** This test is deliberately demanding. It needs full attention for fifteen unbroken minutes and should not be started by someone who cannot give that right now. Said plainly and early, in the participant's interest rather than as a disclaimer.
2. **What it involves.** Fifteen minutes, uninterrupted, no pause, no way to resume, leaving means starting over. **Questions are timed and a question that runs out of time simply moves on**: no going back, no pause, no explanation. Said here so it reads as the test working rather than the site breaking.
2b. **How to read the logic questions.** Some questions give two statements and a conclusion. **Assume both statements are true, even where they are obviously false in the real world, and answer only whether the conclusion must follow from them.** Stated here, repeated identically in every such question, and carried as a one-line reminder on the question itself. Three mechanisms for one rule because this test degrades the working memory that instructions live in: a participant in block 4 holding four digits and a transformation cannot also be relied on to retain a framing given twelve minutes earlier. Without the reminder, instruction decay would grow across exactly the blocks where the load effect is measured and would be indistinguishable from it.
3. **Sound is required.** Connect speakers or headphones and turn the volume up. A metronome ticks once per second throughout and it is part of the test, not decoration.
4. **"Can you use sound?"** An explicit question with an explicit answer, not an assumption. Answering no switches the metronome to a heartbeat icon beating once per second, costs nothing, and requires no explanation.
5. **The tap-along check.** Ten seconds of the tick, or the heartbeat for the visual mode, with the space bar pressed in time. Must pass to continue. Confirms the signal is actually perceived and records a reaction-time baseline (§5b, "Who cannot hear it").
6. **The intake form**, every field optional (§5), with the email note.
7. **What happens to their data**, in a sentence, linking to the privacy policy rather than restating it.
8. **The start button.**

### Why the warning precedes the form

**DECIDED by the owner: the effort warning is early, before any user-specific data is collected.**

Three things follow, and the third is the one that would be missed:

**It is honest at the only moment it can be.** A warning after the form is a warning after the commitment, and a participant who has already typed their name and email is more likely to push on when they should stop. Someone whose attention is divided produces noise and a worse experience for themselves.

**It is data minimisation, not just courtesy.** Anyone who reads the warning and leaves has given us nothing. No name, no email, no row. The alternative collects personal data from people who were never going to finish, and then has to justify holding it.

**It is a screening device and that is a feature.** Participants who opt out here are the ones who would have produced the noisiest data. The test is recruited from volunteers (§1), so there is no traffic target to protect and no reason to keep someone who should not be taking it today. A smaller clean sample beats a larger contaminated one, and this is the cheapest filter available.

**No personal data is written before the start button.** The intake form posts with the session, not on keystroke or on blur. A half-filled form that is abandoned leaves nothing behind.

**An audio check belongs here, not in the first trial.** Browsers refuse audio without a user gesture, so nothing can be verified before the participant acts. The practice trial carries the tick (§4b) so they hear it before anything is scored, and the practice trial is the right place to discover a muted laptop, not trial seven.

**DECIDED: a tap-along pre-test, before the intake form.** The participant hears the tick for about ten seconds and presses the space bar in time with it. Someone running the visual mode presses in time with the heartbeat pulse instead: same task, different channel.

This is the owner's design and it is strictly better than asking. It replaces a claim with an observation. A muted laptop cannot be tapped along to, and unlike the microphone option (rejected below) it works with headphones on, which is the configuration participants are told to use.

**It also produces a baseline, which is the part worth noticing.** Tapping a 1 Hz signal yields, per participant, before any load is applied:

- confirmation the signal is perceived at all, which is what it was built for
- **a simple reaction-time baseline**
- **timing variability**, a cheap read on baseline attention

That baseline turns latency (§2) from an absolute into a relative measure. A slow participant under load is only interesting against their own unloaded speed, not against everyone else's, and without a baseline there is no way to make that comparison. It costs ten seconds and it makes the latency column mean something it otherwise would not.

**Pass criterion, and the honest state of it.** The check has to reject two different things: no signal perceived (few or no presses near a tick) and mashing (presses unrelated to the signal). Hit rate alone cannot separate them, because at 1 Hz a generous window makes random pressing pass by chance. So it is scored on three things together: press count close to tick count, presses landing near ticks, and **low variance in the offsets**, since someone tracking a metronome has a consistent lag and someone mashing does not.

**[OPEN] the exact thresholds, which cannot be set honestly without piloting.** They are a calibration, not a design decision, and guessing them in a document is how a check becomes either a nuisance that fails good participants or a formality that passes anyone. Starting point to pilot against: ten ticks, presses within 300 ms, at least seven of ten, offset standard deviation under 150 ms.

**On failure the participant is told plainly and can retry**, after turning the volume up or switching to the visual mode. It is a setup problem, not a verdict, and it is worded that way. Failing twice should offer the visual mode rather than a third attempt.

**Placement: before the intake form.** Someone whose audio does not work finds out before typing their name and email, not after. It also supplies the user gesture that unlocks audio, which has to happen somewhere and may as well happen here.

The browser cannot reliably report that a device is muted, so a claim at setup is only ever a claim. This turns it into something observed, at the one moment when correcting it is still free. Catching a silent session on question nine is too late; catching it before block 1 costs nothing but the practice block, which was already being spent.

**Considered and rejected: verifying the tick through the participant's microphone.** Technically buildable with `getUserMedia` and a 1 Hz transient detector. Rejected for three reasons, in descending order of how fatal they are:

- **Headphones defeat it.** Participants are instructed to wear headphones (§5b). With headphones on, the microphone hears nothing, so the check fails precisely for the people who complied and succeeds only for those using speakers. It is backwards before anything else goes wrong.
- **Echo cancellation suppresses the signal.** Browsers apply AEC by default on microphone capture, and its whole purpose is removing the device's own output from the input. Requesting `echoCancellation: false` has uneven support and the operating system may apply it regardless.
- **The permission prompt costs more than the problem.** Asking a volunteer for fifteen minutes and then for access to their microphone is a large escalation of trust; some refuse, some leave at the prompt, and it would need its own privacy disclosure even with nothing stored. It also reads badly against a site that self-hosts fonts so builds never touch Google and links out to Spotify rather than embedding it.

Underneath all three: **nobody has an incentive to lie here.** Self-report is unreliable when the respondent gains from the lie. A volunteer helping with research gains nothing by claiming to hear a tick they cannot hear, which is what makes one honest question sufficient where it would not be in an adversarial setting.

### The chrome problem, and why it is not cosmetic

The root layout renders `SiteHeader`, `SiteFooter`, and, for a signed-in member, `AskPanel` on **every** page. For the run screen all three are wrong, and the panel is worse than wrong.

**The Ask Roger panel can open during a trial.** A member and an anonymous visitor would then be taking different tests: one of them has a chat surface that can appear over the item mid-decision. That is not a styling blemish, it is two populations in different conditions, and the comparison between them would be worthless. Suggestion: remove access to Ask Roger during the test.

Next.js cannot remove a root layout from a nested route; the root always renders. The fix is the standard App Router pattern: move the header, footer and panel out of the root layout into a `(site)` route group wrapping the ordinary pages, leaving the root bare, and put the run screen outside that group. **Route groups do not change URLs**, so nothing that exists today moves.

**SHIPPED 2026-10-03**, PR 264, deployed on `e34b3fef51ab`. 101 of 105 files were pure renames and every route verified at its original path. The run screen can now be built outside `(site)` and will have no header, no footer and no Ask Roger panel, which is the owner's requirement that access to the assistant is removed during the test. It is removed by not being mounted, rather than hidden: a mounted panel can still open.

### The run screen

Deliberately sparse. One item, the response control, the confidence control, nothing else competing for attention. No navigation, no links away, no progress bar.

**No countdown timer on screen.** The tick already makes time salient and the phase limits already bound it. A visible clock would add a second, uncontrolled pressure on top of the manipulation under test.

**DECIDED: a question counter, "12 of 30", and never a time remaining.** With no pause and no resume, a participant who cannot tell how far through they are is the one most likely to abandon at minute nine. The counter carries progress; the tick already carries time. They are different things and only one of them belongs on screen.

Deliberately a question count rather than a block count. "Block 3 of 5" would leak the structure, and a participant who works out there are five blocks may infer the last one is easier again, which is precisely the block the control depends on being met naively.

### Interruption

Fifteen minutes is long enough that refreshes, phone calls and closed laptops will happen. Each trial result posts as it completes, so a partial session is still data rather than nothing.

**DECIDED: no resume, and the participant is warned before starting.**

Resuming would poison the timing control the instrument depends on: a trial answered after a four-minute phone call is not the same trial, and nothing in the data would say so. Clean timing is worth more here than the convenience of picking up where someone left off.

What follows from it:

- **The warning is prominent and specific**, on the screen before the start button: fifteen minutes, uninterrupted, no way to pause, and leaving means starting over. Vague wording here converts directly into abandoned sessions and wasted volunteer goodwill.
- It doubles as a **commitment device**. Someone told plainly that they cannot pause is more likely to find headphones and shut the door first, which is exactly the preparation the instrument needs.
- **A `beforeunload` prompt** on the run screen, so a stray tab close or back gesture asks first. It is the one browser nag that is justified here, because the cost of the accident is ten minutes of someone's time.
- **Returning starts a fresh session.** No "continue where you left off", because there is nothing valid to continue.
- **Partial sessions are kept and marked abandoned**, not discarded. Where people stop is a measurement of the fifteen-minute burden, and it is the only evidence available about whether the test is too long. Abandoned sessions are excluded from the accuracy and confidence analysis by status, never by deletion.

### Phone

Fifteen uninterrupted minutes with audio, entering digit strings on a touch keyboard, with notifications arriving, is a noisier instrument than the same test on a desktop.

**DECIDED: allow, and record the device class** (desktop, tablet, phone) on the session. A willing volunteer on a phone is worth more than one who defers to later and never comes back, and sample size is the scarcest thing here.

Recording it means the question gets answered rather than assumed. Watch latency in particular: entering four digits on a touch keyboard is slower, and that comes out of the same 20.5 seconds everyone else gets, so a phone effect would most likely appear there first.

## 6. Anonymous participation

Anonymous visitors already carry a first-party id cookie, HttpOnly, 13 months (`docs/events/README.md`), so repeat attempts are **detectable but not preventable**, and one enthusiast can be eleven data points.

**DECIDED: allow, and mark it as a repeat.** Detection has three sources, in descending reliability: the account for a member, the email address when one is given, and the 13-month anonymous cookie. A session records which of them matched.

Blocking was rejected as unenforceable rather than undesirable: a private window or a second device defeats it, so it would mostly inconvenience honest participants while the determined get through undetected and unmarked, which is worse than not trying.

A repeat is excluded from the headline figures by its flag, never by deletion, and is interesting in its own right: someone who already knows the trick and still answers under load is a different measurement, not a spoiled one.

Members are identified by account, so their repeats are unambiguous.

## 7. Ethics and the debrief

The test deliberately induces error and then tells the participant they were confidently wrong. That can land as insight or as a gotcha, and the difference is entirely in the debrief.

Non-negotiable:

- The **mechanism** is explained afterwards: what the test was doing, how load escalated, why an intuitive answer feels right when it is wrong. A participant should leave understanding what happened to them.
- **Not item by item.** Per §2.4, no correct answer is ever disclosed. The explanation teaches the effect using published, already-public demonstrations rather than this test's own items, which keeps the obligation and the instrument both intact.
- The result is framed as a demonstration of a general effect, never as a verdict on the person. "This is what load does to everyone" rather than "you scored poorly".
- An n of 1 proves nothing about the individual, and the page says so.

### 7.1 The gap created by email-only follow-up

Two decisions are individually fine and collide:

- The email address is **optional** (§5).
- There is no results screen; all follow-up is **by email** (§5b).

Together they mean a participant who gives no email takes fifteen minutes, is deliberately led into confident error, and is then thanked and sent to the home page having been told nothing. That is the gotcha outcome §7 exists to prevent, and it lands hardest on the people being most generous: the ones who helped without wanting anything back.

**The fix does not require an email address.** `/decision-test/about` is a public page explaining what the test measured and how the load escalated. No personal data, nothing per-participant, available to anyone including people who never took it.

**It reveals no answers.** Per §2.4 it explains the effect through published, already-public demonstrations rather than through the thirty items, so a reader learns the mechanism without learning what to answer. That is a real constraint on the writing: it is harder to explain a trap without showing the trap, and the page has to be written that way regardless.

The thank-you screen links to it directly rather than burying it. A participant who wants the explanation gets it immediately; a participant who wants *their own numbers* gives an email and receives them later. The obligation is met for everyone and the email stays genuinely optional.

It is also reusable content. The same page is a supplement to the article series, which is where this started.

**DECIDED: held back.** `/decision-test/about` is not published while collection is open, because a participant who reads it first is contaminated. It is linked from the thank-you screen, so everyone who finishes gets it, and published openly once collection closes. The explainer screen says this plainly rather than leaving it to be noticed.

**DECIDED: the intake form warns about the email.** Stated where the field is, not in the small print: without an email address there is no way to send results. The participant then makes that choice knowingly, which is the right resolution. It keeps the field genuinely optional while removing the quiet unfairness of someone discovering afterwards that their generosity bought them nothing.

Note what this does and does not fix. The *explanation* reaches everyone through the thank-you link regardless of email. The *personal numbers* require an address. Those are different things and the warning should say which is which, or a participant will reasonably read "no results" as "no explanation either" and supply an address they did not want to give.

## 7b. The dataset, which is the actual deliverable

The test is the collection mechanism. **The product is a governed, curated dataset and the means to analyse it.** A well-curated dataset makes a conclusion cheap to reach and easy to defend; a badly curated one makes every question a fresh archaeology project, and the cost compounds with every run.

Three rules, all of which this repository already runs on elsewhere, so none of them is new discipline:

### 7b.1 Nothing is ever deleted, everything is flagged

`decision_log` is kept as training data and is never tidied away; repeats are marked rather than refused; abandoned sessions are kept rather than dropped. Same here, without exception.

Every exclusion is a **column**, never a `DELETE`:

| Flag | Why a later analysis wants it |
|---|---|
| `status` (completed, abandoned) | where people stop measures the fifteen-minute burden |
| `is_repeat` | a second run by someone who knows the trick is a different measurement, not a spoiled one |
| `outcome = expired` | clusters in blocks 3 and 4 show where the executive gave out |
| recall outcome by type | untransformed digits mean storage held and the executive failed |
| `audio_mode` | sound and visual participants are not interchangeable |
| `device_class` | a phone is a noisier instrument |
| `tap_check_passed` | separates verified perception from a claim |

A question nobody has asked yet is the one that gets answered by a row somebody nearly deleted.

### 7b.2 Provenance on every row, or the data cannot be pooled

**Items will change. Timings will change. Scoring will change.** The moment any of them does, old and new rows stop being comparable, and nothing in the numbers themselves says so. This system already solved that problem twice: prompts carry a version, evaluation runs carry a `corpus_fingerprint`, and a run records the configuration it executed under.

Every session therefore records:

- `instrument_version` — block structure, counts, per-phase time limits
- `item_set_version` — which thirty items, in which order
- `key_version` — the scoring key in force

Without these, the first time an item is reworded the dataset silently becomes two datasets wearing one name. With them, pooling is a decision someone makes deliberately and can defend.

### 7b.3 Identity is separable from measurement

Name, email, occupation and age range are personal data; an answer and a latency are not. Keeping them in one table makes the whole dataset personal data and makes retention an all-or-nothing choice.

**Participant identity lives in its own table**, referenced by the session. Clearing a participant row anonymises every measurement they produced while leaving the measurements intact and analysable, which is exactly how the event stream already drops identity at 13 months while the counts survive. It is also what makes "delete everything you hold about me" a one-row operation rather than a reconstruction.

### 7b.4 The grain, and the shape analysis needs

**One row per question presented.** That is the unit. Block, session and participant aggregates are derived from it and never stored as the source of truth, per the one-definition rule in `docs/metrics.md`.

Three tables carry the measurements, at three natural grains: one per session, one per answer, one per block recall. On top of them, a flat analysis view joins everything a correlation needs onto each answer row: demographics, audio mode, device, baseline reaction time, block load, item family, outcome, latency, confidence. **Analysis should never require a join somebody has to remember to write.** The relationships the owner is looking for, education against calibration, baseline attention against susceptibility, load against the confidence gap, are all one `GROUP BY` away from that view or they will not be looked for.

### 7b.5 Where it is stored, and where it is analysed

**DECIDED: collection is relational, in the existing Postgres. Graph analysis comes later, from a clean projection.** Recorded as [ADR 0030](adr/0030-relational-collection-graph-analysis.md).

The short reasoning: a statistical correlation is not an edge. It is computed across many rows rather than traversed between two nodes, and the questions here are `GROUP BY` and regression, which SQL does natively. The data is a star schema with fixed grain and fixed joins. And the asymmetry decides it: relational data projects into a graph cheaply, while relational rigour cannot be recovered from a graph built without it, and getting that wrong costs a re-collection of the scarcest input this project has.

Graph analysis is expected, not precluded, and is the better tool once test results are linked across the rest of the site. Four commitments keep that export mechanical rather than a redesign:

- relationships are **named explicitly**, each stating the edge it becomes: a participant *took* a session, a session *contains* an answer, an answer *is of* an item and *sits in* a block
- the grain stays **one row per answer**
- the **flat analysis view is the projection boundary**, and nothing outside it needs to know the table layout
- entities carry **stable identifiers** that survive an export, so a node traces back to the row that produced it

### 7b.6 Mechanisms, not just storage

- **Metric views** for every headline number, defined once in a migration, per `docs/metrics.md`. The confidence gap, accuracy by block, expiry rate by block, recall failure by type.
- **A CSV export** from the admin surface, built on the flat view, so the owner can work in whatever tool he prefers without anyone writing SQL by hand at the point of curiosity.
- **An admin surface** to read sessions and spot problems early, because a collection fault found after forty participants has cost forty participants.
- **The item bank, including the key, as data rather than code**, versioned, so an item can be corrected without a deploy and the correction is recorded.

## 7c. Operational means live-tested through the UI

**DECIDED by the owner: the test is not operational until it has been
fully exercised through the UI on a deployed build.** Not a smoke check,
not a component test, not a scripted run against the API.

The reason is specific to this build rather than general caution. Almost
everything that makes this instrument *valid* is invisible to a test
runner: whether the tick is actually audible at one per second, whether
the visual heartbeat is perceivable, whether twenty seconds is enough
for a person to read and decide and rate, whether two seconds is enough
to memorise four digits, whether holding those digits actually loads
anybody, and whether the instructions are understood, particularly the
syllogism framing where a misreading produces data indistinguishable
from belief bias.

A green suite would say the code does what it was told. It would say
nothing about whether the instrument measures what it claims to, and
that is the only question worth answering before volunteers are asked
for fifteen minutes each.

The protocol, the paths it has to cover and the rows to read back
afterwards are in the roadmap under M5b.

## 8. Still to specify

Not yet drafted, pending the decisions above:

- Routes, and whether the test is public or gated. The data argument says public, since anonymous reach is the point.
- Data model and migrations, to §7b: `dt_participants`, `dt_sessions`, `dt_answers`, `dt_recalls`, `dt_items` (key included, server only), plus the flat analysis view.
- Events, which go into `docs/events/README.md` in the same PR.
- Admin surface for reading results.
- Metric views, under the one-definition rule in `docs/metrics.md`.
- Test length, which is downstream of §2.2 and §3 and is the single biggest driver of completion rate.

## 9. Open decisions, collected

| # | Decision | Recommendation |
|---|---|---|
| 1 | Fixed blocks or adaptive staircase | **DECIDED: staircase** |
| 2 | Domain-flavoured items | **DECIDED: generic only** |
| 3 | What the email address is for | **DECIDED: results on request, follow-up, thanks, repeat detection** |
| 4 | Test length | **DECIDED: 15 minutes hard ceiling, every phase time-controlled** |
| 5 | Metronome | **DECIDED: one tick per second, constant throughout, the baseline rather than the ramp** |
| 6 | Probe frequency and spacing | ~1 in 4 or 5, at least two in the final third |
| 7 | Time pressure as a separate load level | Leave out of v1; latency is already recorded and a timer confounds haste with substitution |
| 8 | Repeat attempts | **DECIDED: allow and mark, flagged not deleted, detected by account, email or cookie** |
| 22 | Progress indicator | **DECIDED: question counter "12 of 30", never time remaining, never block count** |
| 23 | Phones | **DECIDED: allowed, device class recorded on the session** |
| 24 | Claimed audio that is actually muted | **DECIDED: tap-along pre-test before the form, space bar in time with the tick or the heartbeat. Observed, not self-reported, and it yields a reaction-time baseline** |
| 25 | Disclosing correct answers | **DECIDED: the key exists and is never shown. Overall score only, "N of 30 correct"** |
| 27 | Where the answer key lives | **DECIDED: server only. Grading server-side, the client is sent items and options but never which is correct, and never the running score** |
| 28 | What counts as operational | **DECIDED: a full live pass through the UI on a deployed build, both signal modes, rows read back afterwards. CI passing is not sufficient** |
| 26 | Microphone verification of the tick | **REJECTED: headphones defeat it, echo cancellation suppresses it, and the permission costs more than the problem** |
| 8b | Resuming an interrupted session | **DECIDED: no resume, warn before starting, keep partials as abandoned** |
| 9 | What the emailed result contains | **DECIDED: score plus the confidence gap. Brier stored not sent; no percentile; no item identified** |
| 10 | Failed recall invalidates the block | **DECIDED: no, keep the answers, store the recall by failure type; a miss is a data point, not a fail** |
| 21 | Correctness feedback during the test | **DECIDED: none at all, because confidence is a dependent variable and feedback would contaminate it** |
| 11 | Tick during the practice trial | Yes, it doubles as an audio check |
| 12 | Participants who cannot hear the tick | **DECIDED: ask before starting; a heartbeat icon beating once per second replaces the tick; the mode is stored on the session** |
| 13 | Ask Roger during the test | **DECIDED and SHIPPED: not mounted on the run screen at all, via the (site) route group, PR 264** |
| 14 | Audio preconditions | **DECIDED: told to connect sound and raise the volume before starting, §5b** |
| 15 | Questions per block | **DECIDED: 6 per block, 5 blocks, 30 total** |
| 16 | The about page | **DECIDED: held back until collection closes, linked from the thank-you screen** |
| 17 | Email warning on the form | **DECIDED: states that no address means no personal results, while the explanation still reaches everyone** |
| 18 | Effort warning placement | **DECIDED: first, before any user-specific data is collected** |
| 19 | A question that runs out of time | **DECIDED: `expired` as its own outcome, move straight on, no going back or pausing, stated in the instructions beforehand** |
| 20 | Block 5 | **DECIDED: the fatigue control, back to block 1's difficulty** |
