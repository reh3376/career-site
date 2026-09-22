# The JD submitter's workflow

What a hiring manager goes through from first visit to a finished
review, what the site tells them at each step, and what was found and
fixed when this path was walked end to end on 2026-09-22 (owner's
backlog item: "investigate the workflow for a user that submits a JD
for review").

## The path

| Step | Page | What happens | What they are told |
|---|---|---|---|
| 1 | `/` (IT or OT landing) | "Request access" is the primary action; contact, sign in, LinkedIn and GitHub are the only other public routes | Who Roger is, what the JD review does |
| 2 | `/register` | Access request; the owner gets an approval email with one-click accept / decline | "We'll email you" |
| 3 | email | `user_approved` mail with a sign-in link | Access granted |
| 4 | `/login` → `/home` | Member home; "Upload a JD" card | Two live surfaces |
| 5 | `/jd-upload` | Paste the posting; optional role, employer, contact email, application link; the gate is quoted from `JD_MATCH_THRESHOLD` | What the score means, what happens above the gate |
| 6 | submit | Row stored; pipeline queued (one at a time on the box) | A modal opens with a 0 to 100% progress bar and the current stage ("judging requirement 4 of 12"); keep it open or close it, the email comes either way |
| 7 | waiting | Result panel polls for up to an hour (5 s, then 20 s); the pipeline writes `progress_pct` / `progress_stage` as it goes | Progress bar and stage; "15 to 30 minutes" |
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

## Fit categories

The bands are the owner's numbers, edited on `/admin/jd` (stored in
`app_settings` as `jd_fit_bands`, in force within seconds, existing
scores re-classified on read). "Strong" is also the résumé gate.
Defaults seeded from the calibration: very strong 0.85, strong 0.70
(the gate), possible 0.55, weak 0.35, very weak below.

## Still to look at

- Phone-width layout of the result panel and the verdict table.
- A résumé takes about 13 minutes to write on the box; a shorter
  target length would cut that (owner's call, see the tuning log).
- The register → approve → sign-in leg was not re-walked in this pass;
  it shipped earlier with its own live check.
