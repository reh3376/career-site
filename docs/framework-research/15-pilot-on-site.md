# Pilot On-Site: Accounts, the Instrument, the Stimulus and the Workflow Review on rogerhenley.dev

**Status.** Written 2026-11-08 (late) on the owner's instructions of the same evening, recorded in §0. The pilot moves from Google Forms plus a screen-shared call onto the site itself, behind a per-participant flag. Supersedes `11` §3 (the platform decision), `13` §0 decision 8 and `12` row 9. This workstream is built **before** Phase 0 step 3, because it blocks the owner's recruiting (P3) and step 3 blocks nothing outside this repository. Repository copy ahead of project (2026-11-08): new document.

FSD: §5.16 FR-PLT-01 to 14 and FR-AUTH-19 (specification ahead of code, all Not built at 0.5.1).

---

## 0. Decisions

**From the owner, 2026-11-08, asked one at a time and answered:**

1. **Every pilot participant creates an account** on rogerhenley.dev with an email address and a password. The owner recruits on LinkedIn, speaks to each person, confirms the address they will register with, and adds it to the whitelist. A whitelist entry marked as a pilot entry makes the registered user a pilot participant; the owner can also switch the flag on a member directly in the admin console.
2. **The pilot area is visible only to signed-in users whose pilot flag is set**, and it holds everything: the survey instrument (sections A to F, branching by role, a flag on every item, the pilot-only closing items), the mocked match record with a comment box per requirement row, and the company hiring workflow with a comment box per node. Google Forms is dropped.
3. **Forgot-password works by a 5-digit emailed code**, entered together with the new password, replacing the emailed link. The code lives fifteen minutes and is burned after five wrong attempts; the response to the address step is the same whether or not the account exists.

**Taken by the agent under the delegation, each reversible before the code lands:**

4. Participants are ordinary members plus the flag. They see the member area too (JD reviewer, Ask Roger, meetings); for a recruiter the reviewer is useful context, and a second access model is cost without a stated need.
5. The instrument is **data, not pages**: a versioned JSON definition embedded in the API binary (`survey v0.1`, the same version `12-instrument-change-log.md` tracks), rendered by the site. Every session records the version it ran under, and a copy of the definition is stored in the database the first time a version is fielded, so a later edit to the file cannot change what a past participant saw.
6. One session per participant per instrument version, resumable: answers save per section, a participant who closes the tab returns to the section they left. The pilot is a wording test, not a timed measurement, so resume costs nothing.
7. The candidate perspective is **not** shown to employer-side participants; it belongs to the candidate-side study.
8. Comments on the stimulus never change the record. Audit and Override are shown as the hiring manager would see them, disabled, with a caption saying so, because `08` §2.3 asks whether a hiring manager would touch them.
9. Exports are CSV, taken by the owner from the admin surface, and land in the private folder. Pilot rows appear in no public figure anywhere on the site.
10. The call keeps Part A (the cognitive interview on the survey) and uses Parts B and C to **discuss what the participant wrote on-site** rather than to show the material for the first time. The participant arrives having already read both pages at their own pace, which is a better first reading than a screen-share.

## 1. What a participant experiences

1. Receives the LinkedIn message or post, replies, speaks to the owner, confirms an email address. The owner adds it to the whitelist with the pilot flag.
2. Registers at `/register` with that address and a password of at least twelve characters. Because the address matches the whitelist, the account is approved the moment the verification email is confirmed; no waiting on a manual approval.
3. Signs in. The menu shows "Pilot"; `/home` shows a card. Both lead to `/pilot`.
4. `/pilot`: the consent screen from `11` §1 (who, what for, what is asked, what is kept, what they get, consent checkbox, recording preference). Consent timestamp and preference are recorded; nothing else.
5. `/pilot/survey/<section>`: one section per screen, progress shown, a "this was unclear or didn't fit me" checkbox beside every item, role branching from A1, the pilot-only closing items at the end. Section open and submit times are recorded.
6. `/pilot/record`: the mocked match record, read-only, in the site's own design, with a comment box under each of the ten rows and one general box. The first prompt is `08`'s: which row would you check first, and why.
7. `/pilot/workflow`: the company hiring workflow as a diagram and as a node list C-01 to C-26 with a comment box per node, plus a repeatable "a step that is missing" field.
8. `/pilot/done`: what happens next, the call, the summary they will receive. Finishing marks the session complete; the owner sees it on `/admin/pilot`.
9. Forgot-password at any point: address, new password, the 5-digit code from the email, signed in.

## 2. Access and identity

| Change | Where | Notes |
|---|---|---|
| `access_grants.pilot boolean NOT NULL DEFAULT false` | migration `00064_pilot_access.sql` | The whitelist entry identifies a participant (decision 1) |
| `users.pilot_approved boolean NOT NULL DEFAULT false` | same migration | Set from the grant when registration auto-approves; switchable by the owner on the member page |
| `email_tokens.attempts integer NOT NULL DEFAULT 0` | same migration | Wrong-code counter; `code_hash` already exists on the table |
| `UpsertAccessGrantRequest.pilot`, `AccessGrant.pilot` | `admin.proto`, additive | Checkbox "pilot participant" on `/admin/access` |
| `SetMemberPilotApproved(user_id, approved)` | `AdminService`, new RPC, `AUTH_LEVEL_ADMIN` | Toggle on `/admin/registrations/<id>`; audited like other admin writes |
| `Me.pilot_approved` | `common.proto`, additive | The web reads it to show the menu entry and gate `/pilot/*` server-side |
| `ResetPasswordRequest.email`, `.code` | `auth.proto`, additive; `token` kept for wire compatibility and ignored | The handler requires `email + code + new_password`; the link path is removed from the email and the pages |
| `ForgotPassword` | handler | Mints a reset token whose `code_hash` is the hash of a 5-digit code from `crypto/rand`, TTL 15 minutes; emails the code; fixed response as today |
| `ResetPassword` | handler | Finds the user's live reset tokens, compares the code hash in constant time, increments `attempts` on a miss and burns the token at five, consumes on a hit, then the existing path: breach check, hash, set, revoke sessions, sign in |
| `/forgot-password`, `/reset-password` | web | Address first; then new password and code on one screen; copy says the code expires in fifteen minutes |
| `password_reset` email template | `services/api/internal/email/templates` | Carries the code and nothing clickable |

The pilot gate is enforced twice: the `/pilot/*` pages read `Me.pilot_approved` server-side and return 404 to a member without it, and every `PilotService` RPC refuses a caller without it. Per ADR 0031, the flag that is declared on the whitelist form is the flag the gate reads.

## 3. Data model

Migration `00065_pilot.sql`. Every table carries `tenant_id` and leads its indexes with it; every mutable table has `updated_at` and the trigger.

```sql
-- The instrument as it was fielded, copied from the embedded JSON the
-- first time a version is used, so the file can change and a past
-- session still reads the exact questions it answered.
CREATE TABLE pilot_instruments (
  version     text        PRIMARY KEY,          -- 'v0.1', matches 12-instrument-change-log.md
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  definition  jsonb       NOT NULL,
  fielded_at  timestamptz NOT NULL DEFAULT now()
);

-- One per participant per instrument version. Resumable.
CREATE TABLE pilot_sessions (
  id                     uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id              bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  user_id                bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  instrument_version     text        NOT NULL REFERENCES pilot_instruments(version),
  role_branch            text        NOT NULL DEFAULT '',   -- recruiter | hm | both, from A1
  status                 text        NOT NULL DEFAULT 'consented', -- consented | survey_done | record_done | workflow_done | finished
  consent_at             timestamptz NOT NULL DEFAULT now(),
  recording_ok           boolean     NOT NULL DEFAULT false,
  self_reported_minutes  integer,
  most_wanted_to_skip    text        NOT NULL DEFAULT '',
  survey_finished_at     timestamptz,
  finished_at            timestamptz,
  created_at             timestamptz NOT NULL DEFAULT now(),
  updated_at             timestamptz NOT NULL DEFAULT now(),
  UNIQUE (user_id, instrument_version)
);
CREATE INDEX idx_pilot_sessions_status ON pilot_sessions (tenant_id, status, created_at DESC);

-- One row per item answered; the flag rides the answer (08 §2.2).
CREATE TABLE pilot_answers (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  session_id  uuid        NOT NULL REFERENCES pilot_sessions(id) ON DELETE CASCADE,
  section     text        NOT NULL,
  item_id     text        NOT NULL,                 -- A1, C1, D4, ...
  value       jsonb       NOT NULL,                 -- shape per item type
  flagged     boolean     NOT NULL DEFAULT false,
  answered_at timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (session_id, item_id)
);

-- Section open and submit instants, for the measured-against-self-
-- reported time check (07 §1).
CREATE TABLE pilot_section_events (
  id         uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id  bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  session_id uuid        NOT NULL REFERENCES pilot_sessions(id) ON DELETE CASCADE,
  section    text        NOT NULL,
  event      text        NOT NULL,                  -- opened | submitted
  at         timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_pilot_section_events_session ON pilot_section_events (tenant_id, session_id, at);

-- Comments on the stimulus rows and the workflow nodes, one per target
-- per session, upserted.
CREATE TABLE pilot_comments (
  id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id   bigint      NOT NULL DEFAULT 1 REFERENCES tenants(id),
  session_id  uuid        NOT NULL REFERENCES pilot_sessions(id) ON DELETE CASCADE,
  target_kind text        NOT NULL,                 -- record_row | record_general | workflow_node | workflow_missing | workflow_general
  target_id   text        NOT NULL DEFAULT '',      -- r1..r10, C-01..C-26, or a sequence number for missing steps
  body        text        NOT NULL,
  created_at  timestamptz NOT NULL DEFAULT now(),
  updated_at  timestamptz NOT NULL DEFAULT now(),
  UNIQUE (session_id, target_kind, target_id)
);
```

No table stores a name or an employer beyond what `users` already holds; the export joins `users.name` and `users.email` only on the admin surface, and the private-folder files are keyed by `P-NN`, not by either.

## 4. The instrument as data

`services/api/internal/pilot/instruments/v0.1.json`, embedded. Shape:

```json
{
  "version": "v0.1",
  "sections": [
    {"id": "A", "title": "About your role in hiring", "intro": "...",
     "items": [
       {"id": "A1", "type": "single", "prompt": "In the last 24 months, which best describes your role in hiring?",
        "options": ["Recruiter / talent acquisition", "HR generalist who screens applicants",
                    "Hiring manager (I own the role and decide)", "Both recruiting and hiring-manager duties", "None of these"],
        "required": true, "routes": {"None of these": "end"}}
     ]}
  ],
  "branches": {"A1": {"Recruiter / talent acquisition": "recruiter", "HR generalist who screens applicants": "recruiter",
                       "Hiring manager (I own the role and decide)": "hm", "Both recruiting and hiring-manager duties": "both"}}
}
```

Item types: `single`, `multi` (with `max_select`), `text`, `number`, `scale` (1 to 5), `grid` (rows × columns, used by C3 over the items selected in C1, which the on-site form can do and Google Forms could not; this closes `12` row 9's compromise). Each item may carry `branch: recruiter | hm | both`, `pilot_only: true`, `required: true`. Every item renders the flag checkbox; it is not in the definition because it is not the instrument's business.

Content: `06` §3 sections A to F verbatim, with the eight `[WF]` additions from `12` rows 1 to 8, C1 in the fixed order of `06` §3 (`13` §0 decision 8's reasoning survives the platform change), and the two pilot-only closing items from `08` §2.2. A change to the file is a new version and a row in `12`.

## 5. The stimulus as data

`services/api/internal/pilot/stimulus/v1.json`: the ten rows of the mocked match record, transcribed from the design artifact (`07` §3), with the header figures (75 % evidenced, 25 % unresolved with two questions out, 0 % unmet, 17 % of evidenced weight on an equivalency), the legend, each row's local id, requirement text, source quote, category, type, weight, determination, basis line, rationale, evidence line, and for r6 the generated question with its two option lists. Rendered by a site component in the blueprint-editorial design, no em dashes, with the comment box under each row.

The record is read-only. Audit and Override render disabled with the caption "disabled in the pilot: shown as a hiring manager would see them". The "How this was produced" footer and the versions block are kept, because an interviewee who reads that far is telling us something.

## 6. The workflow page

The company perspective only. Two renderings on one page: the diagram as an SVG committed under `apps/web/public/pilot/company-workflow.svg` (rendered once from `diagrams/company-workflow.mmd`; the site loads no third-party script, and its CSP would refuse one), and the node list from `09` §1 as data (`workflow/company-v2.json`: id, name, owner, one line), each node with a comment box. Below the list: "A step that is missing" with "between which two nodes" and a description, repeatable. The owner's six Part C prompts from `11` §2 appear as the page's guidance text, so the participant knows what kind of comment is wanted.

## 7. The admin surface

`/admin/pilot`: sessions (participant, role branch, status, consent time, measured minutes beside self-reported, finished), each opening to its answers with flags, its comments by target, and its section timings. Export: one CSV per table (`sessions`, `answers`, `comments`) keyed by a `P-NN` code the owner assigns, taken into the private folder (`11` §5). The whitelist form at `/admin/access` gains the pilot checkbox; the member page at `/admin/registrations/<id>` gains the toggle. Nothing from these tables reaches `/how-ask-roger-works` or any public figure.

## 8. Build sequence

| PR | Scope | Exit |
|---|---|---|
| PL-1 | `00064_pilot_access.sql`; proto additive fields and the new admin RPC; the pilot flag through registration auto-approval and the admin toggle; `Me.pilot_approved`; the reset-by-code flow end to end (handler, email template, two web forms); tests for the code path (success, wrong code five times burns it, expiry, unknown address gets the fixed response) | The owner adds a test address with the flag, registers, is approved, sees "Pilot" in the menu, resets a password by code |
| PL-2 | `00065_pilot.sql`; `PilotService` (start or resume, get section, submit section with flags, upsert comment, finish; admin list, get, export); instrument v0.1 and the stimulus as embedded JSON; events in the registry | Sessions, answers, flags and comments round-trip in tests against the test database |
| PL-3 | The six pages under `/pilot`, the record and workflow components, the SVG, `/admin/pilot`, the whitelist checkbox and member toggle in the web | The owner completes the pilot end to end on the deployed build and the export opens |
| PL-4 | Field: whitelist the ten, live pass per `11`, exports into the private folder after week 2 | Ten finished sessions |

Phase 0 step 3 resumes after PL-3. The hiring migrations in `14` §2 renumber from 00066; `14` anticipated that.

## 9. What this changes in the earlier documents

- `11` §1: the consent text is now the first screen of `/pilot`. §2: Parts B and C are discussions of what the participant wrote on-site; the prompts stand. §3: superseded; the platform is the site. §4: the stimulus check is replaced by the site page (PL-3's live pass). §6: the owner's list loses the Google Form and gains the whitelist entries.
- `12`: row 11 records the move on-site; row 9's compromise is withdrawn because C3 can be a true per-selection grid.
- `13` §0 decision 8 superseded by this document; §2 gains the PL sequence before step 3.
- `08` §1 and §2.2: the survey channel is the site; the per-item flag is a true per-item control, as specified.
- `00-todo.md`: P2b, the on-site build, with PL-1 to PL-4.
