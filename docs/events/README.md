# Product event stream

One append-only table, `events`, holds every product event the site
records: anonymous page views on the landing page, member actions, JD
reviews finishing, admin decisions. It replaced the member-only
`activity_events` table on 2026-09-22 (migration `00023_events.sql`
copied the old rows in, tagged `backfilled_from: activity_events`).

Why one stream: the questions worth answering cross the anonymous and
member boundary. "How many landing visitors request access, verify,
get approved, sign in, submit a JD, and open the result" is one query
when everything is in one table with one visitor id, and five joins
when it is not.

## Where events come from

| Source | How | Identity attached |
| --- | --- | --- |
| Browser | `apps/web/src/lib/events-client.ts` queues, `components/event-beacon.tsx` emits; batches POST to the public `EventService.Record` RPC, `sendBeacon` on page hide | api reads the session cookie and the `career_anon` cookie; the client never sends identity |
| api | `events.Writer.Emit` at the success point of a handler (`handlers/events_wire.go` lists the handlers) | the user id the handler already resolved |
| Scheduler | expiry and auto-decline jobs, and the nightly `events-anonymize` job | user id from the row |

The web proxy (`apps/web/src/proxy.ts`) sets `career_anon` on the
first page load: a random UUID, HttpOnly, SameSite=Lax, 400 days. It is
never read by script and never sent anywhere but this site.

## Row shape

| Column | Meaning |
| --- | --- |
| `tenant_id` | Which site the row belongs to. Always 1, the owner's site, until there is another. See ADR 0029; it is attribution, not isolation. |
| `event_id` | UUID. Client-minted for browser events so a retried batch de-duplicates; server-minted otherwise. |
| `name` | Registry name below. Unknown names are dropped at the edge. |
| `occurred_at` | Server clock at receipt. `client_ts` keeps the browser's clock for skew analysis only. |
| `anon_id` | The `career_anon` cookie, when present. |
| `user_id`, `session_id` | Member identity when known. |
| `ui_mode` | `it` or `ot`, from the mode cookie. |
| `path` | Page path, query string stripped (utm_* parameters are kept in `utm`). |
| `referrer_host` | Host of `document.referrer`, nothing more. |
| `device` | `phone`, `tablet`, `desktop` from the user agent. The user agent itself is not stored. |
| `ip_hash` | First 8 bytes of SHA-256(`EVENT_IP_SALT` + address), hex. |
| `app_commit` | Build commit, so a metric shift can be lined up with a deploy. |
| `props` | JSON object; only keys the registry allows for the name survive, and the whole object is capped at 2 KB. |

## Registry

The registry lives in code, `services/api/internal/events/events.go`,
and this table mirrors it. Add a name in both places in the same PR.
"Browser" means the beacon may send it; everything else is api-only.

| Name | Browser | Props | Emitted when |
| --- | --- | --- | --- |
| `page.view` | yes | `title` | a route renders |
| `page.leave` | yes | `dwell_ms` | the route changes or the page hides |
| `landing.mode_switch` | yes | `from`, `to` | the IT/OT toggle is used |
| `landing.cta_click` | yes | `cta`, `href` | an internal link is clicked on `/`, or any link with `data-event` |
| `link.external` | yes | `host` | a link leaves the site |
| `register.submit` | | `user_id` | registration accepted and the verify email sent |
| `verify.success` | | `user_id` | a verify token consumed |
| `verify.already_used` | | | a verify token rejected as used or expired |
| `approval.requested` | | `user_id` | a verified user parked in pending_approval |
| `approval.decided` | | `user_id`, `decision`, `channel` | approve or decline, from `email_link`, `console` or `whitelist_auto` |
| `approval.auto_declined` | | `user_id` | the scheduler declined a stale request |
| `login.success` | | `user_id` | a session minted |
| `login.failed` | | `reason` | `bad_credentials` or the account status that blocked it |
| `logout` | | | a session revoked by the user |
| `access.expired` | | `user_id` | the expiry job cut access |
| `article.view` | yes | `slug` | reserved; not yet emitted |
| `gallery.view`, `gallery.photo_open` | yes | `id` | reserved; not yet emitted |
| `repo.click` | yes | `name` | reserved; not yet emitted |
| `jd.view` | yes | | reserved; `page.view` on `/jd-upload` covers it today |
| `jd.submitted` | | `submission_id`, `chars`, `has_apply_url` | a JD stored |
| `jd.finished` | | `submission_id`, `outcome`, `score`, `threshold`, `fit` | a pipeline run reached ready, below_threshold or failed |
| `jd.result_viewed` | yes | `submission_id`, `via` | a finished result rendered; `via` is `submit` or `reopen` |
| `jd.pdf_downloaded` | | `submission_id` | the résumé PDF served |
| `jd.poll_abandoned` | yes | `submission_id`, `waited_ms` | reserved; not yet emitted |
| `jd.quota_blocked` | | `submitted`, `limit` | a member turned away at the daily cap |
| `jd.outcome_recorded` | | `submission_id`, `status` | ground truth attached to a submission |
| `jd.feedback_recorded` | | `submission_id`, `target`, `rating` | the member rated a verdict |
| `chat.opened` | yes | `surface` | the assistant panel is opened |
| `chat.asked` | | `conversation_id`, `chars`, `turn` | a question is accepted, **before** the answer. Deliberate: a question that is never answered still leaves a row, and that is the one most worth having |
| `chat.answered` | | `conversation_id`, `path`, `total_ms`, `miss_reason`, `best_similarity` | an answer is produced. `path` is `qa_bank`, `model`, `degraded`, `out_of_scope`, `no_support` or `error`; the bank-miss fields travel with it so "which phrasings to add" is one ordered query |
| `chat.abandoned` | yes | `waited_ms`, `had_answer` | the reader leaves while an answer is being written |
| `chat.citation_click` | yes | `href`, `cta` | a source under an answer is followed |
| `meeting.view` | yes | | `/meetings` renders |
| `meeting.duration_picked` | yes | `minutes` | a length is chosen |
| `meeting.slot_picked` | yes | `minutes`, `lead_days` | a slot is chosen |
| `meeting.booked` | | `booking_id`, `minutes`, `mode`, `provider`, `lead_days` | a booking is written. `lead_days` is how far ahead they planned |
| `meeting.cancelled` | | `booking_id`, `by`, `lead_days` | a booking is cancelled. `lead_days` here is how much notice was given, which is the half that says whether the slot was recoverable |
| `meeting.abandoned` | yes | `step`, `elapsed_ms` | the booking page is left without booking |
| `contact.submitted` | | `category`, `has_jd` | a contact message stored |
| `admin.decision_reviewed` | | `decision_id`, `kind`, `verdict`, `corrected`, `dimensions` | a decision-log row graded. `corrected` matters: a graded row that gained a corrected answer is worth something a graded row without one is not |
| `admin.rescore` | | `submission_id` | a rescore queued |
| `admin.fit_bands_changed` | | | the fit bands saved |
| `admin.jd_limit_changed` | | `limit` | the daily JD cap changed |
| `admin.qa_entry_created` | | `entry_id`, `enabled`, `covers_restricted` | a Q&A bank entry added |
| `admin.qa_entry_updated` | | `entry_id`, `enabled`, `covers_restricted` | an entry edited |
| `admin.qa_entry_enabled` | | `entry_id`, `enabled` | an entry approved or withdrawn |
| `admin.qa_entry_deleted` | | `entry_id` | an entry removed |
| `activity.*` | | | backfill only, from the old `activity_events.kind` |

## Privacy rules

These are the commitments the privacy page makes; the code has to keep
them.

- No third-party analytics. The beacon talks to this site only.
- The user agent is reduced to a device class before storage. The
  address is stored as a salted hash (`EVENT_IP_SALT`, set in prod).
- Query strings are stripped from `path`; only `utm_*` survives.
- Identity columns (`user_id`, `session_id`, `anon_id`, `ip_hash`) are
  blanked by the daily `events-anonymize` job on rows older than
  `EVENT_IDENTITY_RETENTION_DAYS` (default 400). Counts survive.
- Deleting a user sets `user_id` to NULL on their rows (`ON DELETE SET
  NULL`); the anonymous row remains.

## Reading it

`/admin/analytics` renders the standing views. For anything else the
read-only query console under `/admin/db` takes SQL directly. Three
starting points:

```sql
-- Landing funnel for the last 30 days, by visitor.
WITH v AS (
  SELECT anon_id,
         bool_or(name = 'page.view' AND path = '/')   AS landed,
         bool_or(name = 'landing.cta_click')          AS clicked,
         bool_or(name = 'register.submit')            AS registered,
         bool_or(name = 'verify.success')             AS verified,
         bool_or(name = 'login.success')              AS signed_in,
         bool_or(name = 'jd.submitted')               AS submitted
  FROM events
  WHERE occurred_at > now() - interval '30 days' AND anon_id IS NOT NULL
  GROUP BY anon_id)
SELECT count(*) FILTER (WHERE landed)     AS landed,
       count(*) FILTER (WHERE clicked)    AS clicked,
       count(*) FILTER (WHERE registered) AS registered,
       count(*) FILTER (WHERE verified)   AS verified,
       count(*) FILTER (WHERE signed_in)  AS signed_in,
       count(*) FILTER (WHERE submitted)  AS submitted
FROM v;

-- JD outcomes by fit category and app commit.
SELECT app_commit, props->>'fit' AS fit, count(*)
FROM events WHERE name = 'jd.finished'
GROUP BY 1, 2 ORDER BY 1, 2;

-- Which Q&A phrasings to add next, ranked by how nearly they already
-- worked. A threshold miss wants a new phrasing; a margin miss wants
-- two entries pulled apart, which is why miss_reason is recorded.
SELECT props->>'miss_reason' AS why,
       round((props->>'best_similarity')::numeric, 4) AS best,
       count(*)
FROM events
WHERE name = 'chat.answered' AND props->>'path' = 'model'
GROUP BY 1, 2 ORDER BY 2 DESC;
```

## Rate limits and abuse

`EventService.Record` is public. It takes at most 50 events per call
and 120 events per minute per client address (token bucket in the
handler). Names outside the registry, props outside the registry, and
anything over 2 KB of props are dropped silently; the response only
says how many were stored. A hostile client can therefore inflate
counts for its own address hash, and nothing else.
