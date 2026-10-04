# FSD: the decision test

**Status: draft, in progress with the owner.** Open decisions are marked
**[OPEN]** and are the owner's to make. Nothing here is built.

A supplement to the *Better Business Decisions Require Better
Information* series (`/articles/better-business-decisions-part-1` to
`-part-5`). Open to members and to anonymous visitors.

---

## 1. What it is for

The owner's research question, in his words:

> How much effort is required to get a given person to make obviously
> wrong decisions in a manner that makes the person believe they are
> confidently correct.

Three claims sit behind it:

1. Attention and intuitive decision quality are related.
2. A person's capacity for effortful thinking is finite.
3. When effort runs short, a fast intuitive answer becomes the
   **default**, taken without regard to available data.

The test exists to collect data on that, and secondarily to let a reader
experience it on themselves rather than read about it.

### 1.1 The distinction that decides the instrument

"Finite attention" has two readings and they have very different
evidential standing.

**Sequential depletion (ego depletion).** Effortful task A leaves you
worse at later task B. This is the famous version and it largely failed
to replicate; the multi-lab registered replications found effects near
zero. **Not used here.** A test built on it would be the easiest
possible thing for a knowledgeable reader to dismiss, on a page whose
whole argument is about evidence quality.

**Concurrent load.** Working memory is occupied *at the moment of
decision*. This replicates well, and De Neys ran essentially the owner's
hypothesis: holding a digit string while answering base-rate and
conjunction problems reliably pushes people onto the heuristic answer.

So the test **occupies** attention during each decision rather than
trying to tire the participant out across a session. Shorter, better
evidenced, and it measures the thing actually described above.

This is the single most important design decision in the document and it
is why the test looks the way it does.

## 2. What is measured

Per trial, four values:

| Value | How |
|---|---|
| Answer | correct, **lure** (the intuitive wrong answer), or other |
| Latency | ms from item shown to answer committed |
| Confidence | the participant's own rating, collected every trial |
| Load | which load condition the trial ran under |

Confidence on every trial is not optional decoration. Error rate alone
answers "does load make people wrong". The owner's question is about
being wrong *and sure*, which needs confidence on the same trial.

### 2.1 The headline measure

**The confidence–accuracy gap under load.** If accuracy falls sharply
while confidence barely moves, that gap is the finding. It is also the
number worth showing a participant about themselves:

    unloaded    accuracy 78%   mean confidence 74%
    loaded      accuracy 41%   mean confidence 71%

Accuracy fell 37 points, confidence fell 3. That is the result stated in
a way a reader remembers, and it is exactly the owner's question
answered for one person.

Calibration can be scored properly as a Brier score. **[OPEN]** whether
the participant is shown the Brier score or only the plain comparison
above. Recommendation: plain comparison on screen, Brier stored.

### 2.2 The per-person threshold

The question says *a given person*, which means a threshold rather than
a group average: the load level at which this participant starts
answering with the lure while still reporting high confidence.

**DECIDED: staircase.** Load escalates as the test progresses, seeking
the point where this participant starts answering with the lure while
still reporting high confidence.

### 2.3 The confound a rising ramp creates, and the fix

A load level that only ever rises is confounded with time-on-task.
Trial 20 differs from trial 2 in two ways at once: more load, and
fifteen minutes of elapsed effort. When accuracy falls, nothing in the
data says which caused it, and "they were tired by the end" is exactly
the sequential-depletion account §1.1 excluded on evidential grounds. A
monotonic ramp lets it back in through the running order.

**Low-load probe trials are scattered throughout, including late ones.**
They are the control. A late probe answered as well as an early probe
rules out fatigue and leaves load as the explanation. Late probes that
degrade too are themselves a finding, about the fifteen minutes rather
than about load, and worth having rather than being unable to tell.

Probes are not optional and are not decoration. Without them the
headline measure in §2.1 cannot be attributed to anything.

**[OPEN]** probe frequency. Roughly one in four or five trials, spread
so that at least two land in the final third. Exact spacing is
downstream of the trial count.

## 3. How load is applied

Concurrent working-memory load, the standard manipulation:

| Level | Load | Participant does |
|---|---|---|
| 0 | none | answers the item |
| 1 | light | holds a 2-digit string, recalls it after answering |
| 2 | heavy | holds a 6 to 7-digit string, recalls it after answering |

**The recall step is what makes the load real.** Without being asked to
reproduce the string, a participant can simply drop it and the condition
measures nothing. Trials where recall fails are evidence that load was
genuinely carried, not trials to discard; **[OPEN]** whether a failed
recall invalidates the trial's answer data. Recommendation: keep it,
flag it, decide with data in hand.

**[OPEN] Time pressure as a third manipulation.** A countdown is easy to
build and intuitively appealing, but it confounds effort with haste:
a wrong answer under a timer may be a rushed answer rather than a
substituted one. Recommendation: leave it out of v1, since latency is
already recorded and time pressure would muddy it.

## 4. The items

Each item needs a compelling wrong answer that arrives fast, and a
correct answer that requires deliberation. Three families, all with
decent replication records: base-rate neglect, conjunction, and
CRT-style arithmetic traps.

**The classic CRT items cannot be used.** Bat-and-ball, widgets and lily
pads are so widely circulated that a meaningful share of this audience
has met them. They would measure prior exposure, not reflection. Novel
items with the same structure are required, and item design is real work
rather than a lookup.

**DECIDED: no domain-specific items.** Generic throughout. Domain
scenarios would have let an expert bypass the lure through knowledge
rather than reflection, which is a confound dressed as personalisation.
The cost is that the page looks less like this site; the benefit is that
a plant engineer and an accountant are answering the same instrument.

## 4b. Timing and the metronome

**DECIDED: 15 minutes maximum, every portion time-controlled, and a
one-per-second tick audible throughout.**

### Budget

Fifteen minutes is the ceiling for the whole visit, not for the trials.
Intake, instructions, a practice trial and the debrief all come out of
it, which leaves roughly ten to eleven minutes of scored trials. At the
trial shape in §2 and §3 that is about twenty trials, and the probe
spacing in §2.3 fits inside that.

Each phase has a hard limit rather than a nominal one, so the test ends
at a known time for every participant and duration never becomes a
variable the analysis has to carry.

### The tick is the constant, not the ramp

**One tick per second, unchanging, for the whole test.** It is the
baseline, and it is explicitly *not* how effort is escalated; that
mechanism is specified separately.

Holding it constant is what makes it useful. A tick that varied with
load would be part of the manipulation and impossible to separate from
it. Identical in every trial and every condition, it cannot differ
between the things being compared, so it drops out of the analysis as a
controlled variable rather than entering it as a confound.

What it does buy: time is continuously salient, pacing is identical for
everyone, and the auditory channel carries the same occupancy
throughout, so a quiet trial and a loaded trial differ only in the thing
under test.

Three implementation consequences follow:

**It is disclosed before it starts.** A participant who is surprised by
it is a participant who abandons, and an unexpected noise is a worse
experience than a described one.

**Implementation is Web Audio, synthesised, scheduled ahead.** Not
`setInterval`, which drifts audibly over fifteen minutes and would make
the tick arrive at a different cadence for different participants, and
not an audio file, which is an asset request the CSP would have to allow
and a download that can fail mid-test. A synthesised click scheduled on
the audio clock is sample-accurate and loads nothing.

**It needs a user gesture to begin.** Browsers refuse audio that starts
without one. The "start the test" button is that gesture, which is
convenient, but it means audio cannot be verified before the
participant commits. **[OPEN]** whether the practice trial carries the
tick so a participant hears it before the scored section starts.
Recommendation: yes, and it doubles as a check that audio actually
works on their device.

### Who cannot hear it

A deaf or hard-of-hearing participant, or anyone who cannot have sound
on, is in a different condition from everyone else. That is a real
limitation and the options are not equally good:

- **Record it.** Whether audio was on becomes a stored variable and the
  analysis can split on it. Honest, cheap, and it does not pretend the
  problem is solved.
- **Visual equivalent.** A pulsing indicator at the same cadence. It
  substitutes one channel for another rather than matching the load,
  and a visual tick competes with reading the item in a way a sound
  does not.

Recommendation: record it, offer the visual equivalent, and state the
limitation rather than claiming parity between the two.

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
Name plus email plus occupation plus age range identifies a person. The
meeting scheduler shipped a week ahead of its privacy disclosure and
that is not repeated here.

**DECIDED, what the email is for.** Four uses, and the privacy policy
states all four because that is the commitment being made:

1. Sending the participant their results, **only if they ask for them**.
2. Follow-up contact.
3. A thank-you for taking part.
4. Identifying someone who takes the test more than once.

The fourth is the one worth noticing: it makes the address an
**identifier**, not just a contact. That is legitimate and it has to be
disclosed in those terms rather than buried as "we may contact you".

**Optional fields bias the sample.** Whoever fills them in is not a
random subset. That does not make them useless, it means analysis
reports completion rates alongside any demographic split. Worth knowing
before anyone reads a bar chart of education against accuracy.

## 5b. Pages and layout

### Routes

| Route | Chrome | What |
|---|---|---|
| `/decision-test` | normal site | explainer, intake form, start button |
| `/decision-test/run` | **none** | the test itself |
| `/decision-test/results/<id>` | normal site | debrief and the participant's numbers |

All three are public and therefore go in `PUBLIC_PATHS` in
`apps/web/src/lib/public-routes.ts`, which is the one table the proxy,
`robots.ts` and `sitemap.ts` all read. A member-only test would defeat
the point: anonymous reach is the data.

**[OPEN]** whether `/decision-test/results/<id>` should be indexable.
It is public by URL, carries one person's numbers, and the id should be
unguessable regardless.

### The chrome problem, and why it is not cosmetic

The root layout renders `SiteHeader`, `SiteFooter`, and, for a signed-in
member, `AskPanel` on **every** page. For the run screen all three are
wrong, and the panel is worse than wrong.

**The Ask Roger panel can open during a trial.** A member and an
anonymous visitor would then be taking different tests: one of them has
a chat surface that can appear over the item mid-decision. That is not a
styling blemish, it is two populations in different conditions, and the
comparison between them would be worthless.

Next.js cannot remove a root layout from a nested route; the root always
renders. The correct fix is the standard App Router pattern: move the
header, footer and panel out of the root layout into a `(site)` route
group layout wrapping the ordinary pages, leaving the root bare, and put
the run screen outside that group. **Route groups do not change URLs**,
so nothing that exists today moves.

That is a structural change touching every route, which is why it is
named here rather than discovered mid-sprint. It is also worth doing on
its own merits: the site currently has no way to serve a focused screen.

### The run screen

Deliberately sparse. One item, the response control, the confidence
control, nothing else competing for attention. No navigation, no links
away, no progress bar.

**No countdown timer on screen.** The tick already makes time salient
and the phase limits already bound it. A visible clock would add a
second, uncontrolled pressure on top of the manipulation under test.

**[OPEN]** whether to show trial count ("4 of 20"). It helps a
participant commit to finishing, and it is a weaker time cue than a
clock. Recommendation: yes, count only, never time remaining.

### Interruption

Fifteen minutes is long enough that refreshes, phone calls and closed
laptops will happen. Each trial result posts as it completes, so a
partial session is still data rather than nothing.

**[OPEN]** whether an interrupted session can resume. Resuming is kinder
to the participant and poisons the timing control the test depends on;
starting over is clean data and throws away ten minutes of someone's
goodwill. Recommendation: no resume, say so before they start, and keep
the partial data marked as abandoned.

### Phone

Technically fine, and worth deciding against on substance. Fifteen
uninterrupted minutes with audio, entering digit strings on a touch
keyboard, with notifications arriving, is a different and noisier
instrument than the same test on a desktop. **[OPEN]**: allow and record
the device class, or discourage phones at the door. Recommendation:
allow, record, and let the data show whether it mattered.

## 6. Anonymous participation

Anonymous visitors already carry a first-party id cookie, HttpOnly, 13
months (`docs/events/README.md`), so repeat attempts are **detectable
but not preventable**, and one enthusiast can be eleven data points.

**[OPEN]** how to treat a repeat attempt: block, allow and mark, or
allow and only count the first. Recommendation: allow and mark. A second
attempt by someone who now knows the trick is itself interesting, and
blocking is both unenforceable and annoying.

Members are identified by account, so their repeats are unambiguous.

## 7. Ethics and the debrief

The test deliberately induces error and then tells the participant they
were confidently wrong. That can land as insight or as a gotcha, and the
difference is entirely in the debrief.

Non-negotiable:

- Every item is explained afterwards, including **why** the lure is
  compelling. A participant should leave understanding the mechanism,
  not just their score.
- The result is framed as a demonstration of a general effect, never as
  a verdict on the person. "This is what load does to everyone" rather
  than "you scored poorly".
- An n of 1 proves nothing about the individual, and the page says so.

## 8. Still to specify

Not yet drafted, pending the decisions above:

- Routes, and whether the test is public or gated. The data argument
  says public, since anonymous reach is the point.
- Data model and migrations.
- Events, which go into `docs/events/README.md` in the same PR.
- Admin surface for reading results.
- Metric views, under the one-definition rule in `docs/metrics.md`.
- Test length, which is downstream of §2.2 and §3 and is the single
  biggest driver of completion rate.

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
| 8 | Repeat attempts | Allow and mark |
| 9 | Show the Brier score or the plain comparison | Plain on screen, Brier stored |
| 10 | Failed recall invalidates the trial | Keep and flag |
| 11 | Tick during the practice trial | Yes, it doubles as an audio check |
| 12 | Participants who cannot hear the tick | Record it, offer a visual equivalent, state the limitation |
