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
| 6 | submit | Row stored; pipeline queued (one at a time on the box) | Acknowledgement: 15 to 30 minutes, the review lives at `/jd-upload/<id>`, an email follows |
| 7 | waiting | Result panel polls for up to an hour (5 s, then 20 s) | Status: queued / scoring / writing the résumé |
| 8 | finished | Score, "N of M evidenced", every requirement with its verdict and reason; above the gate the résumé and the locked PDF | Below the gate: what was and was not evidenced; Roger still sees it |
| 9 | email | `jd_result` mail to the member's address (and the contact address if different): score, summary, link to the review, PDF link when there is one | |
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

## Still to look at

- Phone-width layout of the result panel and the verdict table.
- A résumé takes about 13 minutes to write on the box; a shorter
  target length would cut that (owner's call, see the tuning log).
- The register → approve → sign-in leg was not re-walked in this pass;
  it shipped earlier with its own live check.
