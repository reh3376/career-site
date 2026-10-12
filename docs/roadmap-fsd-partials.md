# Roadmap: the partially-shipped FSD rows

Opened 2026-10-12. Eleven rows the FSD marks shipped with a hole in
them, in the order they will be worked, each with a checklist and a link
to its own working document.

**Why these before the 45 not-built items.** A row marked *Shipped* with
a gap underneath is more dangerous than a row marked *Not built*,
because nobody is looking for it. Every one of these was invisible until
the documentation audit on 2026-10-12 went looking, and three of them
turned out to be controls that exist on paper and nowhere else.

**The ordering rule**, so it can be argued with rather than guessed at:

1. **Decided before undecided.** Items 1 and 2 have their direction
   settled by [ADR 0031](adr/0031-declared-controls-are-enforced-controls.md);
   they need costing, not debate.
2. **Enforcement gaps before features.** Items 1 to 3 are all the same
   shape — something declared and never applied — which is the failure
   this repository has produced eleven times.
3. **Then descending by how much already exists**, so the cheapest
   closures come first and the architectural work is reached with the
   small things out of the way.
4. **Except where a dependency outranks all of it.** Item 9 cannot move
   regardless of priority; it is gated on participant data, not code.

**Status key:** `[ ]` not started · `[~]` in progress · `[x]` done ·
`[-]` dropped, with the reason recorded in the working document.

---

## Order

| # | item(s) | working document | state |
|---|---|---|---|
| 1 | FR-AUTH-09 + FR-CNT-22 — Turnstile, and registration's missing rate limit | [FR-AUTH-09-FR-CNT-22-turnstile.md](fsd-items/FR-AUTH-09-FR-CNT-22-turnstile.md) | `[ ]` |
| 2 | FR-ADM-09 — write the audit log, and fix the lost approval actor | [FR-ADM-09-audit-log.md](fsd-items/FR-ADM-09-audit-log.md) | `[ ]` |
| 3 | FR-CNT-16 + FR-CNT-17 — content model and item fields | [FR-CNT-16-FR-CNT-17-content-model.md](fsd-items/FR-CNT-16-FR-CNT-17-content-model.md) | `[ ]` |
| 4 | FR-ADM-03 — surface the ratings that already exist | [FR-ADM-03-review-queue.md](fsd-items/FR-ADM-03-review-queue.md) | `[ ]` |
| 5 | FR-HIST-02 — the welcome-back panel | [FR-HIST-02-welcome-back.md](fsd-items/FR-HIST-02-welcome-back.md) | `[ ]` |
| 6 | FR-EVAL-06 — an evaluation must survive a deploy | [FR-EVAL-06-eval-durability.md](fsd-items/FR-EVAL-06-eval-durability.md) | `[ ]` |
| 7 | FR-CNT-19 — CI link health | [FR-CNT-19-link-health.md](fsd-items/FR-CNT-19-link-health.md) | `[ ]` |
| 8 | FR-PUB-05 — rendering without JavaScript | [FR-PUB-05-no-javascript.md](fsd-items/FR-PUB-05-no-javascript.md) | `[ ]` |
| 9 | FR-DT-20 — tap-check thresholds from pilot data | [FR-DT-20-tap-check.md](fsd-items/FR-DT-20-tap-check.md) | `[ ]` |

Nine rows for eleven requirements: `FR-AUTH-09`/`FR-CNT-22` and
`FR-CNT-16`/`FR-CNT-17` are each one piece of work.

**Moved off this list on 2026-10-12.** `FR-ADM-13` (scheduler) was
listed as partially shipped on the strength of a status line reading
"Not yet: the connect flow, the encrypted refresh token, and the
bookings list". All three exist. The row is now marked shipped, with
what closed it named.

---

## Findings that changed the shape of an item

These came out of establishing current state and were not visible from
the FSD rows.

**Approval decisions have no actor** (item 2). `approval_decisions.decided_by`
exists, the repo struct carries it, the INSERT names it, and nothing
has ever set it: production holds 5 decisions and 0 actors. The
`approval.decided` event emits the *target member's* id as the actor, so
the event stream cannot recover it. This is a correctness bug sitting
inside the audit item rather than a missing feature, and it makes item 2
bigger than "insert rows into a table".

**The quiet check gives evaluations no protection at all** (item 6).
`rollout.sh`'s `quiet_check` reads `dt_sessions` and `events`; it does
not look at `eval_runs`. An evaluation runs for hours and emits roughly
one event when it starts, so a run at minute 11 is already invisible to
the check. I had assumed this was partial cover. It is none, which makes
the cheapest mitigation — query `eval_runs` in the same place — more
valuable than it looked.

**No `stop_grace_period` is set on any service**, so a deploy gives a
running posting about ten seconds before SIGKILL. Raising it is a
one-line change that converts some killed runs into clean ones.

**FR-ADM-03 was never blocked on what the FSD said** (item 5). Its row
claimed the review queue is not driven by negative feedback "because
FR-CHAT-14 (member rating) is not built". FR-CHAT-14 is built and has
been since 2026-09-30, with a live control under every assistant
message; production holds 9 ratings. The real gap is that nothing
queries them. That makes item 5 a query and a surface rather than a
feature, and it moves up.

I reported the opposite earlier on 2026-10-12, from a case-sensitive
grep for `RateMessage` that missed the camelCase client call
`rateMessage`, and read no-match as not-built. Recorded because it is
the same error shape as the rest of this list.

**Photos are not in the corpus** (item 3). `publicCorpusSubdirs`
(`handlers/admin.go`) names `articles` and `other`; `apps/web/content/photos`
exists with frontmatter and is never walked, so nothing in it is
retrievable by Ask Roger or the JD reviewer. The map's own comment
records this trap catching someone before — "the job reported 'kinds
article' and ingested nothing". Whether photos *should* be retrievable
is a decision, not a defect, but the current state was not written down
anywhere.

**Three independent frontmatter parsers exist** (item 3):
`apps/web/src/lib/articles.ts`, `apps/web/src/lib/photos.ts` and
`services/api/internal/ingest/walker.go`. None validates, each reads a
different subset of keys, and the Go one's comment says they are "kept
in sync by convention". The articles themselves are already inconsistent:
five of six carry author/series/part/tags, the teaser carries only
title/subtitle/date.

**Four mutating admin RPCs accept a `reason` and discard it**
(`ApproveRegistration`, `DeclineRegistration`, `ExtendAccess`,
`SetMemberStatus`). The fields exist in the contract, no handler reads
them, and no console form collects one. I used `ExtendAccess`'s reason
on 2026-10-12 believing it was recorded; it was not.

---

## What each set has to clear before it is called done

Every item below, not only the ones that touch auth:

- [ ] The change is verified **against production**, not against a green
      suite. A green suite says nothing about whether a surface works.
- [ ] Where a control is being added, there is a test that fails when
      the control is removed. Verified by removing it, not by assuming.
- [ ] The FSD row, `docs/fsd-open-items.md` and this file are updated in
      the same PR, so the three cannot disagree.
- [ ] If anything turned out differently from this document, the
      document says what it claimed as well as what is true.

The second of those is not ceremony. Of the eleven controls this
repository has declared and not applied, every one had something that
looked like coverage: a config field that was read, a proto option that
was declared, a comment that described enforcement. None of them had a
test that would have failed.
