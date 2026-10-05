# The JD submitter's workflow

What a hiring manager goes through from first visit to a finished
review, what the site tells them at each step, and what was found and
fixed when this path was walked end to end on 2026-09-22 (owner's
backlog item: "investigate the workflow for a user that submits a JD
for review").

## The path

| Step | Page | What happens | What they are told |
|---|---|---|---|
| 1 | `/` (IT or OT landing) | All three core actions are shown and gated on click, so a visitor sees what access buys before being asked for it. The public surface is wider than it was: articles, gallery, `/how-ask-roger-works`, contact, the legal pages, podcast appearances and the "why this site exists" section. `lib/public-routes.ts` is the authoritative list | Who Roger is, what the JD review does, and which model does it |
| 2 | `/register` | Access request; the owner gets an approval email with one-click accept / decline | "We'll email you" |
| 3 | email | `user_approved` mail with a sign-in link | Access granted |
| 4 | `/login` → `/home` | Member home; "JD review" card alongside Ask Roger and booking a meeting | The site is in beta, and what each surface does |
| 5 | `/jd-upload` | Paste the posting; optional role, employer, contact email, application link; the gate quoted on the page is the live "strong" fit band read from the api (`JdService.GetJdReviewConfig`; 0.70 on prod), not an env var | What the score means, what happens above the gate |
| 6 | submit | Row stored; pipeline queued (one at a time on the box) | A modal opens with a 0 to 100% progress bar and the current stage ("judging requirement 4 of 12"); keep it open or close it, the email comes either way |
| 7 | waiting | Result panel polls for up to an hour (every 5 s for the first 3 minutes, then every 20 s); the pipeline writes `progress_pct` / `progress_stage` as it goes | Progress bar and stage; "15 to 30 minutes" |
| 8 | finished | Fit category (very strong / strong / possible / weak / very weak), score, "N of M evidenced", every requirement with its verdict and reason; strong or better: the résumé and the locked PDF | The category's meaning in one line; below the gate: what was and was not evidenced |
| 9 | email | `jd_result` mail to the member's address (and the contact address if different), by category: very strong / strong carry the résumé PDF as an attachment; possible / weak say Roger will review and reply; very weak says no further action is needed | |
| 10 | later | `/jd-upload` lists all their submissions; `/jd-upload/<id>` reopens any of them from any signed-in session | |

The owner gets the `jd_outcome` mail at step 8 as well (score, verdicts
with rationales, application link, admin and decision-review links).

## What was wrong before 2026-09-22

- **No way back.** The acknowledgement said "come back with your
  reference number", but no page took one; the result token lived
  only in the browser tab. Closing the tab lost the review.
- **Polling stopped at 20 minutes**, under the box's 15 to 30 minute
  pipeline (longer when queued). The panel froze on "scoring".
- **Nothing was sent to the submitter** when the review finished; the
  contact-email field implied it would be.
- **No list of a member's own submissions** anywhere.

## What changed (PR 87)

- `JdService.ListMySubmissions` and a "your submissions" list on
  `/jd-upload`.
- `/jd-upload/<id>`: the review page, reopenable by the member who
  submitted it. `GetJdResult` releases verdicts, résumé and the PDF
  link to the owning member by session; the token path still works
  for the submitting tab.
- Polling budget one hour with an honest "15 to 30 minutes, you can
  close this page" message; the acknowledgement links the review page.
- `jd_result` email to the submitter on every terminal state (ready,
  below the gate, failed), audited in `notification_deliveries`.

## What changed (PR 89)

- The progress modal on submit and the 0 to 100% progress bar
  (`progress_pct` / `progress_stage`, migration 00021).
- Fit categories on the result and in the submissions list, with the
  bands editable on `/admin/jd` (migration 00022, `app_settings`).
- The `jd_result` email is written per category, and the very strong /
  strong mails carry the locked résumé PDF as an attachment (Resend
  base64, SMTP multipart/mixed).
- The JD pages quote the live gate through `GetJdReviewConfig` instead
  of an env var.

## Fit categories

The bands are the owner's numbers, edited on `/admin/jd` (stored in
`app_settings` as `jd_fit_bands`, cached 15 s in the api so a change is
in force within seconds; existing scores are re-classified on read).
"Strong" is also the résumé gate. `JD_MATCH_THRESHOLD` only seeds the
setting on first read. Defaults in code (`internal/jd/fit.go`): very
strong 0.85, strong 0.70 (the gate), possible 0.55, weak 0.35, very
weak below.

**Production has since been edited away from two of those**, read from
`app_settings` on 2026-10-05: very strong 0.85 and the gate 0.70 as
documented, but possible is **0.6** and weak is **0.4**. That is the
setting doing its job rather than a defect, and it is recorded here
because a reader comparing a live score against this page would
otherwise misread two bands out of four.

## Since 2026-09-22

The path above still describes what a submitter goes through. Four
things around it have changed, none of which alter the steps:

- **A stranded run no longer stays stranded.** A posting sat in
  `generating` from 25 September until it was noticed on 30 September,
  because a restart mid-generation left the row claiming to be working.
  Startup now closes any run left in a non-terminal status
  (`FailStrandedRuns`), so the submitter gets a failure email instead of
  silence. The failed run's `decision_log` rows are kept: they are
  training data, and tidying them away would cost labels.
- **The landing shows all three core actions to signed-out visitors**,
  gated on click rather than hidden, so step 1 now tells a hiring
  manager what the review does before asking them to register.
- **The site is labelled beta.** "coming soon · late 2026" is gone and
  the status line reads `system · beta`.
- **Ask Roger is live**, which gives a submitter a second reason to have
  an account and a way to ask about a verdict they disagree with.

## Verified 2026-10-05

Re-read against the code, having sat unverified since 2026-09-22. The
path still describes what a submitter goes through, and four specific
claims were checked rather than assumed:

- The polling cadence, "every 5 s for the first 3 minutes, then every
  20 s", and the one-hour budget are exactly what `jd-upload/result.tsx`
  does (`elapsed < 3 * 60 * 1000 ? 5000 : 20000`, `elapsed < 60 * 60 *
  1000`).
- `FailStrandedRuns` exists and is covered by a test for every
  non-terminal state.
- The fit-band defaults match `internal/jd/fit.go`; production has
  moved two of them, now noted above.
- The gate is still read live through `GetJdReviewConfig`, which refuses
  an unauthenticated caller, so the members-only framing holds.

## Still to look at

- Phone-width layout of the result panel and the verdict table. Not yet
  reviewed on a real device; no UI work here counts as finished until it
  has been.
- A résumé takes about 13 minutes to write on the box; a shorter
  target length would cut that (owner's call, see the tuning log).
- The register → approve → sign-in leg was not re-walked in this pass;
  it shipped earlier with its own live check.
