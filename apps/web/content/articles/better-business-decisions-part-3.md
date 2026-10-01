---
title: "Better Business Decisions Require Better Information — Part 3"
subtitle: "When Being Right Is Not Enough"
author: "Roger E. Henley II"
date: 2026
series: "Better Business Decisions Require Better Information"
part: 3
tags:
  - decision-making
  - governance
  - risk
  - operations
  - engineering
  - manufacturing
---

# Better Business Decisions Require Better Information
## Part 3 — When Being Right Is Not Enough

*Roger E. Henley II · 2026*

## Purpose

This paper is intended to improve *how technical analysis moves through an organization and becomes a decision*. Parts 1 and 2 addressed the quality of the analysis itself. This part addresses what happens to it afterward: the exchange in which a recommendation is presented, evaluated, and either acted on or set aside.

That exchange is not a formality. It is a control point, and it fails in ways that have nothing to do with whether the underlying work was correct. A well-constructed analysis can be complete, well-sourced, and right, and still lose to a weaker one. Not because anyone acted in bad faith, and not simply because the analysis was poorly explained, but because the decision environment scores properties that are only loosely related to correctness.

The first two parts were originally written as a two-part framework. This part exists because that framework was incomplete in a way I did not see at the time. Part 1 held that decisions degrade when the information architecture hides the structure of the problem. Part 2 held that decisions degrade further when human interpretation distorts the evidence that does exist. Both are true. Neither explains why an analysis that survives both problems can still fail to change what the business does.

This part also carries a narrower obligation than the first two. Parts 1 and 2 could be read as descriptions of how other people go wrong. This one cannot be written honestly that way. The failure that prompted the series was mine, and the mechanisms described here apply to the person delivering the analysis at least as much as to the people receiving it.

The scope here is diagnostic. It identifies what fails in the technical review and why neither party can see it happening. It does not attempt to specify the meeting standard that would prevent it, which is a larger piece of work and the subject of Part 4.

## Introduction

I wrote the first two parts of this framework after a failure of my own, and the accurate account of it is more useful than the version I told myself at the time.

For several years I worked alongside an executive-level colleague with whom I could not reliably communicate. The pattern was consistent and specific. He would ask a direct question. I would answer with the conditions that governed the answer. He would hear a non-answer. When the conditions later changed and my answer changed with them, he would tell me I had contradicted myself. He asked, repeatedly and reasonably, for a clear yes or no and a date. I found both nearly impossible to give honestly.

The technical situations were genuinely complex. The timelines genuinely depended on inputs I did not control. Both of those things were true, and neither is an excuse. The outcome was that a person responsible for allocating capital across the business could not get a usable answer out of the person best positioned to give him one. That is a failure regardless of who is technically correct about the underlying engineering.

I left the meeting that finally prompted these papers believing I had not communicated well enough. I still believe that. What I did not understand then was why: what specifically was failing, why it kept failing for years despite effort on both sides, and why my instinct to add more context made things worse rather than better every time.

One thing has to be said before the diagnosis.

In those exchanges I was certain I was right and that I was not being heard. He was, in all likelihood, equally certain that he was right and that he was being given evasion instead of an answer. From the inside, those two states are indistinguishable. Mine did not feel like evasion. It felt like precision. His did not feel like impatience. It felt like asking a simple question and not getting one answered.

If the feeling of being right is identical whether or not you are right, then my conviction in those meetings is not evidence that I was correct. The question is not *why wouldn't he listen?* The question is: *how would I know if I were the one who was certain and wrong, not someday, but that day?*

I would not know. Not from the inside. That is why the remedies this series is building toward are structural rather than personal.

Four mechanisms were operating in those exchanges, and none of them is unique to that relationship. Each has a section below.

## Confidence Does Not Ensure Correctness

Part 1 devoted a section to a principle every analyst knows and every organization still violates: correlation does not ensure cause. Two variables move together, the inference to causation feels natural, and it fails because a hidden variable is driving both.

The same structure governs the subject of this paper.

> *Just as correlation does not necessarily imply a causal relationship, confidence does not necessarily imply correctness.*

This is not an analogy in the loose sense. It is the same failure with different variables. Confidence and correctness genuinely do correlate, which is precisely why the shortcut survives. Expertise drives both: the person who has worked a problem for twenty years is more often right and more often assured. The signal is real, and in most situations it is good enough.

But expertise is not the only variable that produces confidence. Repetition produces it. Temperament produces it. Never having encountered the exception produces it. Each of these raises confidence without touching correctness, and none is visible in the signal the listener receives.

The parallel extends further. Part 1 described Simpson's Paradox: a relationship that holds in aggregate can reverse inside the subgroups that matter, because the weighting structure is hidden in the combined view. The relationship between expressed confidence and correctness has the same shape. Across a population, confidence and correctness move together. Within the subgroup of people who understand a problem most deeply, the relationship can invert, because deeper understanding surfaces more conditions and exceptions, and honest expression of that view sounds less absolute.

That subgroup is the one an executive team is drawing from in a technical review. The aggregate relationship holds everywhere except in the room where the decision is being made.

> **Key point:** Confidence is a proxy for correctness in the same way ice cream sales are a proxy for drowning risk. The association is real, the underlying driver is something else, and acting on the proxy without examining the driver is how reasonable people reach wrong conclusions with conviction.

A proxy is acceptable when verification is expensive and the cost of error is low. It becomes dangerous in exactly the conditions Part 1 identified as the domain of this framework: high consequence, delayed feedback, difficult reversal.

## Why Confidence Persuades

Part 2 established that confidence is not evidence. That claim is correct and it is not sufficient, because knowing it does not stop it from working. Organizations that fully accept the principle still reward the confident presenter over the careful one, week after week.

The mechanism is *processing fluency*: the ease with which a listener's mind takes in a claim. Research in cognitive psychology has established that when something is easy to process, people rate it as more likely to be true, independent of whether it is. Fluent delivery is not persuasive because listeners are lazy or unsophisticated. It is persuasive because ease of processing is a signal the mind treats as evidence before deliberate evaluation begins.

Certainty is the most fluent thing a speaker can offer. It requires nothing of the listener. Nuance requires work: the listener has to slow down, hold conditions in mind, track exceptions, and keep two possibilities open at once. A recommendation with three conditions and a stated confidence interval is a bill for cognitive labor the meeting did not budget for. A recommendation with no conditions is a refund.

> **Key point:** Confidence persuades not because it is accurate, but because it is cheap to process. The correct analysis is often not rejected on the merits. It is not evaluated on the merits at all, because a cheaper signal arrived first and settled the question.

A second mechanism operates more slowly. Repeated exposure to a claim increases the felt sense that it is true, even when no new evidence has been introduced. In an enterprise this has a familiar shape. A figure repeated across enough presentations becomes a fact. A vendor claim repeated across enough meetings becomes a specification. A design assumption copied forward across enough projects stops being an assumption at all.

## The Substituted Question

Decision research describes a pattern in which the mind, faced with a hard question, quietly answers an easier one in its place and reports the answer as if it addressed the original. The listener does not experience the substitution.

In a review meeting, the hard question is *is this analysis correct?* Answering it properly would require examining the data, the method, the assumptions, and the alternatives. That is hours of work, often across domains no single attendee commands.

The easy question is *does this feel decision-ready?*

A hedged, well-calibrated engineering assessment fails the easy question by construction. Its calibration is exactly what makes it feel unfinished. The recommendation is not weighed and found wanting. It is screened by a proxy that was never designed to measure correctness, and the screening happens before the weighing.

## Meetings Exist to End Searches

A decision meeting is convened to terminate a search. The organization has a question open, capital or capacity is waiting on the answer, and the purpose of the gathering is to close the question and move. That is what meetings are for.

Expertise does the opposite. The deeper someone's understanding of a subject, the more conditions they can see, the more exceptions they know about, and the more questions each answer opens. This is not hedging and it is not timidity. It is what knowing more consists of. Beginners collect answers. Experienced practitioners collect better questions.

Set those two facts against each other:

- The meeting exists to end a search.
- Expertise, honestly expressed, expands one.

Certainty is therefore not merely more persuasive. It is delivering the thing the meeting was called to produce. That is a *delivery mismatch*, and it is a more useful diagnosis than *I did not explain it well*. I was not failing to communicate. I was communicating something the meeting was not structured to receive: an open search, at the moment a closed one was required.

## The Contradiction That Was Not One

The mechanisms above explain a single failed exchange. They do not explain a failure that repeats for years between two people who are both trying.

The accusation I heard most often was that I contradicted myself. I did not believe it at the time. I believe it now, in the only sense that matters: it was an accurate description of what reached him.

A conditional answer has two components, the answer and the envelope in which it holds. When I answered a question in March, I gave both, but the envelope was delivered as context rather than as part of the answer. Context is what a listener discards first, especially a listener under time pressure who is trying to extract a decision. What survived the exchange was the bare answer.

Four months later the conditions had moved. The bare answer moved with them. From inside my head nothing had contradicted anything: the same rule, applied to different inputs, correctly produced a different output. From where he sat, he had asked the same question twice and received two different answers, with no visible reason for the change. He was not being unfair. He was reporting what he actually received.

> **Key point:** An answer without its envelope cannot help but appear to contradict itself the moment conditions change. The inconsistency is not in the analysis. It is manufactured by stripping the conditions out, and the stripping is usually done by the listener, not the speaker, which is why the speaker never sees it happen.

The fix is not *provide more context*. More context makes it worse, because more context is more material for the listener to discard. The fix is to make the condition *part of the answer itself*, in the same breath, so that it cannot be separated without the answer becoming visibly incomplete.

**Separable, and it will be separated:** We should go with option A. Now, there are some things to keep in mind about moisture content, because the material we've been receiving lately has been running higher than spec, and that interacts with residence time in a way that…

**Inseparable:** Above 15.5% moisture, option A. Below, option B. We are at 14.2% today.

The second version is shorter. It is also strictly more informative, because it survives compression. When conditions move to 16%, the answer changes and the change is self-explaining. The rule was stated up front and the input moved. Nothing contradicts.

I had this backwards for years. I believed the complexity was the message, and that the summary was a lossy compression of it I was being asked to accept. The reverse is true. *The decision rule is the message. The complexity is the derivation.* A derivation is what you bring when someone challenges the rule, not what you lead with, in the same way that a specification sheet carries the tolerance while the calculation package sits in the file, available on request.

The last piece of this cost the most. Every time I was told I was being unclear, I responded by adding explanation. That instinct treated the problem as insufficient information transfer, when the problem was that the information had no structure the listener could hold onto. I was increasing the volume of a signal that was failing on form, not on quantity. Years of that produced a stable and predictable equilibrium. He asked for less, I supplied more, and each of us grew more confident that the other was the obstacle.

## The Calibration Penalty

Combine the mechanisms above and the subgroup inversion described earlier becomes an institutional risk rather than a statistical curiosity.

Credibility, in most rooms, tracks fluency. Competence, as it deepens, tends to reduce fluency, because the practitioner now sees conditions and exceptions that a less experienced person does not.

> **Key point:** In an organization that scores confidence, the discount applied to a person's judgment grows with their expertise. The better informed they become, the less certain they sound, and the less weight their input carries.

This is not a complaint about how experts are treated. It is a description of a selection pressure. Over time, an organization operating this way will overweight the input of those who see the fewest conditions and underweight the input of those who see the most. The people best positioned to identify a developing problem are the people whose warnings sound the least decisive.

## Why Neither Side Can Detect This

The natural response is to solve the problem with judgment: to ask experienced leaders to look past the presentation and assess the substance. That will not work, for two reasons.

**From the inside, correct and incorrect feel the same.** The mind does not produce one sensation for a well-founded belief and a different one for an error. It produces conviction. Whether the conviction is deserved is a separate question the feeling cannot answer. Self-audit fails because the person holding a weak position experiences it exactly as the person holding a strong one does.

**From the outside, earned and borrowed confidence look the same.** A speaker whose position has been tested against contrary evidence and one who has simply repeated a comfortable claim present identically. Fluency is available to both. There is no observable difference in the signal itself.

Both natural detection mechanisms are therefore unavailable. The person cannot assess themselves, and the room cannot assess them either, not because the room lacks experience, but because the discriminating information is not present in what the room can see.

This is the same problem as verifying an instrument against itself. A gauge cannot confirm its own accuracy; every reading it produces, including readings about its own condition, is generated by the mechanism in question. Industry resolved this long ago, and not with judgment. It resolved it with *traceable calibration against an external standard*.

> **Key point:** If neither self-assessment nor peer assessment can discriminate, then seniority in the room is not a remedy. The organization must *test* rather than *assess*.

## Why This Is Worse in Manufacturing

Two features of industrial operations make everything above more dangerous than it would be elsewhere.

### Delayed feedback disables the correction

The natural corrective for an unfounded belief is collision with reality. A prediction fails, a project underdelivers, a conversation exposes a gap, and the belief gets revised.

Part 1 established that industrial feedback is delayed. A process change can look sound today, quietly affect every batch produced afterward, and only surface years later as sensory drift, quality loss, or margin erosion, often after the person who made the decision has moved on.

**In exactly the environment where errors are most expensive, the mechanism that would correct them is structurally disabled.** Unfounded certainty is not punished. It is carried forward, and by the time the cost appears, the causal chain back to the original decision is difficult to reconstruct and easy to dispute. An organization operating under long feedback latency cannot rely on experience to calibrate its people. Experience only calibrates when the results come back.

### A small map feels complete

Part 1 argued that fragmented information hides the structure of a problem. What it did not explain is why the fragmentation so rarely announces itself, and why funding the remedy is so difficult.

**Completeness is a property of the map, not the territory.** Each system in the enterprise stack, MES, ERP, WMS, QMS, CMMS, historian, LIMS, is internally coherent. The person working inside the maintenance system sees a complete maintenance picture. Nothing in that view is marked as missing, because the missing material lies outside the map's edge entirely. A gap that has never been represented does not register as a gap. It registers as certainty about a smaller world.

That is why integration is chronically underfunded, and it is not a failure of business acumen. Nobody requests a capability whose absence they cannot perceive.

> **Key point:** You cannot pay attention to what you do not know exists. Governed, connected information architecture is not primarily a reporting improvement. It is map expansion. It converts variables that were invisible into variables that can be attended to.

## Borrowed Certainty

Part 2 discussed untested assumptions. One variety behaves differently from the rest and requires a different control.

Borrowed certainty is confidence built on conclusions that were never tested by the person holding them. It arrives through repetition rather than investigation. The belief was not strengthened by evidence; it was strengthened by familiarity, and familiarity produces comfort, and comfort quietly becomes conviction.

In industrial operations this is not abstract. It is:

- That is how we have always run it.
- A vendor performance claim repeated until it functions as a specification.
- A design figure carried forward across three projects without re-derivation, its original basis now unavailable.
- The known cause of a recurring failure that no one has re-tested in a decade.
- A control described as effective because it exists and has never visibly failed.

This is inherited technical debt in epistemic form, with the same properties: created by decisions made elsewhere, invisible until something loads it, and paid for by whoever is present when it finally breaks.

The mind does not tag its own beliefs by origin. It does not separate an idea that was tested from an idea that was merely heard many times. If that distinction is going to exist inside an organization, **it has to be recorded, because it will not be remembered.**

## What Engineering Already Solved

Everything described above is a measurement and verification problem. Industrial engineering has mature, funded, uncontroversial answers to measurement and verification problems. It has not applied them to decisions.

| Failure in decision-making | Discipline that already addresses it |
|---|---|
| Self-verification is invalid | Calibration traceable to an external standard |
| Claims presented without stated limits | Operating envelope; validated range |
| Uncertainty that is raised but never closed | Decision rule with defined trip points |
| Fluency-based judgment on irreversible choices | Management of Change; risk-triggered formal review |
| Untested claims accepted on presentation | Proof test: load it before you trust it |
| Undifferentiated uncertainty delivered whole | Criticality ranking; threshold-based reporting |
| Review by people who share your incentives | Independent verification; separate reporting line |
| Tested and merely familiar are indistinguishable | Provenance and lineage recorded on the artifact |
| Defending the model against the observation | As-built discipline: walk it down, redline, reissue |

None of these is a new practice. Every one is already funded somewhere in a well-run plant. The argument is not that the organization should adopt an unfamiliar discipline. It is that the discipline it already accepts for a pressure transmitter has never been extended to a capital decision, and the reasoning behind it applies equally to both.

An instrument managed the way most organizations manage decisions, self-verified, never proof-tested, no traceable provenance, reviewed only by the team that owns it, would be pulled from service.

## Four Things That Work Immediately

The full standard belongs in Part 4. Four practices require no process change and can be adopted by an individual contributor tomorrow.

**Make the condition inseparable from the answer.** State the governing variable and the threshold in the same breath as the recommendation. Above 15.5%, option A. Below, option B. We are at 14.2%. The answer now survives compression, and it cannot later appear to contradict itself.

**Give dates the same treatment.** A bare date is a promise you cannot keep. Conditions without a date is a refusal to answer. State the date with its dependency and its trip point: August 14, contingent on the switchgear shipping by June 1. Every week it slips past that moves the date a week. I will confirm or revise on June 1 either way. That is a yes. It fits on one line of a status report, and when the date moves, the movement is self-explaining.

**Rank the doubts before presenting them.** Rigor is not the same as completeness of disclosure. Presenting every uncertainty at equal weight transfers the ranking problem to an audience that lacks the context to perform it. A competent risk register does not present two hundred line items; it presents what clears the threshold. Surface the two doubts that could change the decision, not the nine that could not.

**Tag the provenance of every material figure.** Measured, modeled, vendor-asserted, assumed, or inherited. This is data lineage applied to the decision brief rather than the database. It requires no new system, and it makes borrowed certainty visible on the page, where familiarity can no longer impersonate evidence.

For the receiving side of the exchange, one question does most of the work: *when someone presents with certainty, do not ask how confident they sound, ask what earned that confidence.* Ask it about yourself before asking it about anyone else.

## Toward a Formal Standard

These four practices are individually useful and collectively insufficient, for a reason that has run through this entire paper. Understanding these mechanisms confers no protection from them. People who study this material still hold blind spots they cannot see, for the same reason everyone else does: the blind spot is not accompanied by a sensation of missing something.

*If education does not confer immunity, the remedy cannot be educational.* Training people to recognize borrowed certainty produces people who are certain they have recognized it. What works is procedure, precisely because procedure does not depend on the participant's ability to detect their own error.

That means the technical review itself has to be specified: entry criteria, required artifacts, the form in which uncertainty is admissible, who is qualified to challenge, what constitutes closure, and what triggers reassessment. Industrial engineering already specifies far less consequential activities to that level of detail.

Part 4 develops that standard. It addresses, among other things, how to define the decision class above which fluency-based judgment is formally suspended; how to structure a technical review so that a bounded answer is the expected deliverable rather than an unwelcome one; how to construct independence that is real rather than nominal; and how to manufacture the collision with reality early, through pre-mortem, shadow-running, and staged validation, in an environment where natural feedback arrives too late to teach anyone. Part 5 specifies what the organization's systems have to record for that standard to run on evidence rather than memory.

The diagnosis is complete. The standard is the work that follows from it.

## Sources and Influences

A paper arguing that claims should carry their provenance is obligated to carry its own.

The framing of this part was prompted by a video essay on the psychology of unearned confidence, published on YouTube by the channel August under the title "The Psychology of Stupid People Who Think They're Smart." Several of the organizing ideas here, including the distinction between borrowed and earned confidence, the observation that a small map feels complete, and the closing question about which is redrawn when reality and the map disagree, originate in that piece.

The underlying research is older and belongs to its authors. Processing fluency and its effect on judgments of truth is established in the work of Rolf Reber, Norbert Schwarz, and later Adam Alter and Daniel Oppenheimer. The illusory truth effect traces to Hasher, Goldstein, and Toppino (1977). Attribute substitution, answering an easier question in place of a hard one, is Daniel Kahneman's. The relationship between competence and self-assessment is Kruger and Dunning (1999), which should be read alongside the substantial statistical criticism it has attracted; the popular reading of that work is considerably stronger than the finding supports. The argument that teams cannot surface error without psychological safety is Amy Edmondson's. The map is not the territory is Alfred Korzybski (1931).

Where this paper contributes something of its own, it is in the observation that industrial engineering already solved these verification problems for instruments, drawings, and physical change, and has never extended that discipline to the decisions those instruments inform.

## Closing Thought

There is a question every process engineer has already answered, many times, without needing to think about it.

When the plant disagrees with the drawing, which one is wrong?

Nobody hesitates. The plant is right. The drawing is stale. You walk it down, you field-verify, you redline, you reissue as-built. An engineer who defended the drawing against the plant would be removed from the job, and rightly, because the drawing was never the point. It was always an approximation maintained for as long as it remained useful.

Organizations do the opposite with decisions constantly. The model is defended and the observation explained away. The operator who saw the anomaly is discounted because the report does not show it. The original framing is preserved and the contradicting evidence absorbed into it. Same error, different artifact, and no discipline attached to it.

The quality of a decision can never exceed the quality of the information behind it. That was Part 1. Better information is not enough if people search, frame, and defend conclusions without discipline. That was Part 2. This part adds the third constraint: a correct conclusion that cannot survive the exchange in which it is delivered has not yet done any work.

The remedy is not louder conviction from the people who are right, and it is not more skepticism from the people deciding. It is a process that does not require either party to detect what neither party can see.

We already know how to do this with drawings. We have not done it with decisions.

---

*Roger E. Henley II — Part 3 of a five-part series on decision quality in complex operations. Preceded by Part 1: "Why Acting on Partial Data Feels Right and Often Is Not," and Part 2: "Why and How We Become Confidently Wrong." Continued in Part 4: "The Technical Review Standard," which develops the review standard this part argues for, and Part 5: "The Decision Infrastructure Standard."*
*github.com/reh3376 · linkedin.com/in/rogerehenley*
