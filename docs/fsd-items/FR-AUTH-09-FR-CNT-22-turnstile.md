# FR-AUTH-09 + FR-CNT-22: Turnstile

**State as of 2026-10-12.** Both requirements describe Cloudflare Turnstile protecting a public endpoint against automated submission. Neither exists. There is no Turnstile verification code anywhere in the repository, no Cloudflare site key or secret in config, and no widget in the web app. The proto fields that carry a `turnstile_token` are declared with `buf.validate` rules, but `buf.validate` itself is not enforced at runtime anywhere in this service (no `protovalidate` interceptor, `protovalidate-go` is not a dependency), so those rules check nothing even on paper. The two requirements share one gap and will share one fix.

## What the FSD requires

- `FR-AUTH-09` (docs/FSD.md:308): "Authentication endpoints shall be rate-limited per IP and per account, with progressive delays after failed attempts, and registration shall be protected by Cloudflare Turnstile (or equivalent)."
- `FR-CNT-22` (docs/FSD.md:356): "Contact / support form... Anonymous submissions require Turnstile."

## What exists today

**Registration (`FR-AUTH-09`'s Turnstile half).** `Register()` in `services/api/internal/handlers/auth.go:107-178` never reads `msg.TurnstileToken` — the field is not mentioned anywhere in that function or anywhere else in `auth.go`. The web registration form doesn't populate it either: `apps/web/src/app/(site)/register/actions.ts:48-58` builds the `RegisterRequest` with `name`, `email`, `password`, `organization`, `statedRole`, `consentVersion`, and `consentAccepted` — no `turnstileToken` key, so it defaults to `""` over the wire. There is no Turnstile widget anywhere under `apps/web/src/app/(site)/register/`.

**Anonymous contact (`FR-CNT-22`'s Turnstile half).** `SubmitContact()` in `services/api/internal/handlers/contact.go:60-165` does check the token for the anonymous path, at line 104: `if msg.TurnstileToken == "" && h.turnstileEnforced() { ... }`. But `turnstileEnforced()` (`contact.go:167-172`) is:
```go
func (h *Contact) turnstileEnforced() bool {
	// Turnstile hookup ships with the FR-AUTH-09 rate-limit pass.
	// Anonymous submissions are still rate-limited by IP at the repo layer
	// once that lands; for now the form is unprotected in dev.
	return false
}
```
It returns `false` unconditionally, so the check at line 104 is always skipped. The web contact form does not send a token either; nothing under `apps/web/src/app/(site)/contact/` references Turnstile.

**What rate limiting does exist (the non-Turnstile half of `FR-AUTH-09`).** Login is rate-limited: `auth.go:99` constructs `ratelimit.New(5, 5.0/(15*60))` (5 attempts, refilling to 5 over 15 minutes), keyed on `"login:" + ClientIP(req) + "|" + addr` and enforced in `Login()` at `auth_session.go:40-46`. This is a single-process, in-memory token bucket (`services/api/internal/ratelimit/limiter.go:1-101`) — it counts successful logins the same as failures, has no progressive delay, and does not survive a multi-instance deployment. There is no equivalent limiter on `Register` — no rate limit of any kind on registration attempts, only the (absent) Turnstile check.

**The proto declarations.** `proto/career/v1/auth.proto:209-213` declares `turnstile_token` on `RegisterRequest` with `(buf.validate.field).string = { min_len: 1, max_len: 2048 }`, and `auth.proto:275-278` declares it on `LoginRequest` (`max_len: 2048`, comment says it becomes required after repeated failures, returning `failed_precondition` / `turnstile_required` — no such logic exists in `Login()`). `proto/career/v1/contact.proto:81-83` declares it on `SubmitContactRequest` (`max_len: 4096`). None of these `buf.validate` rules are checked at runtime: there is no `protovalidate` interceptor in the service, and `protovalidate-go` is not a dependency (confirmed by repo-wide search; the only `protovalidate` references are generated descriptor types pulled in because the protos import `buf/validate/validate.proto`).

## What is missing

- A Cloudflare Turnstile site key (client-side) and secret key (server-side) in any config: not in `services/api/internal/config/config.go`, `docker-compose.yml`, `docker-compose.prod.yml`, or `.env.prod.example`. None exist.
- Any code that calls Cloudflare's siteverify endpoint. A repo-wide search for `siteverify` and `challenges.cloudflare` returns nothing.
- A Turnstile widget in the web app. A repo-wide search of `apps/web/src` outside the generated `gen/` files returns nothing (`turnstile`/`cf-turnstile` both empty).
- `Register()` reading `turnstile_token` at all.
- `Contact.turnstileEnforced()` ever returning `true`.
- Any logic for the `LoginRequest.turnstile_token` "required after repeated failures" behavior the proto comment describes.

## Dependencies and constraints

- **ADR 0031** (`docs/adr/0031-declared-controls-are-enforced-controls.md`), accepted 2026-10-12, named this exact gap as one of ten declared-but-unenforced controls found in that audit, and set the direction: *"Turnstile is verified on registration and on anonymous contact, or both declarations come out."* That is a decision to either build it or remove both FSD promises — not a decision about how to build it.
- **The open design question the ADR left explicit:** what happens when Cloudflare is unreachable. Fail closed (registration and anonymous contact submission both die during a Cloudflare outage) versus fail open (today's behavior — unprotected, but available). The ADR calls this "a decision in itself" and does not make it.
- **`buf.validate` enforcement is a separate, larger piece of work** (no `protovalidate` interceptor exists for *any* message, not just these two) and is tracked separately in `docs/backlog.md` and ADR 0031 item 1. Wiring Turnstile verification does not require fixing `buf.validate` first — the handler-level checks (`turnstileEnforced()`, the `Register` read) are independent of whether `protovalidate` ever runs.
- **The actual exposure today, stated precisely:** registration is approval-gated (new accounts land in `unverified`/`pending_approval`, not active) and email-verified, so an automated signup cannot reach member content. What an attacker *can* do is make the service send a verification email to any address they supply, and populate the `users` table with unverified rows. That is an email-reputation and database-noise problem, not an access-control breach — real, but not urgent in the way an auth bypass would be.

## Decisions the owner must make

1. **Build it or remove both declarations.** ADR 0031 already settled this direction (build it), but it is listed here because it is the fork point everything else depends on.
2. **Fail open or fail closed on a Cloudflare siteverify outage/timeout**, for both registration and anonymous contact. Fail closed means a Cloudflare outage blocks new signups and anonymous contact submissions. Fail open means the status quo (no protection) persists during exactly the window an attacker might expect defenses to be down.
3. **Whether the two surfaces need the same failure-mode choice**, or whether contact (lower stakes — no account created) can fail open while registration (creates a DB row, sends email) fails closed.
4. **Whether `LoginRequest.turnstile_token`'s "required after repeated failures" behavior (per the proto comment) ships now or is descoped** — it is not required by either FSD row's text and is currently unimplemented in `Login()`.
5. **Where the secret lives operationally** — `.env.prod` plus `docker-compose.prod.yml`, following the pattern of other secrets in this repo.

## Task checklist

Each item is independently verifiable against the repo.

1. Add `TURNSTILE_SECRET_KEY` (server) to `.env.prod.example`, `.env.prod`, and the config struct read in `services/api/internal/config/config.go`; verify with a settings/config test following the `compose_env_test.go` pattern (ADR 0031 calls out that exact prior failure mode — a setting the code reads but the container never receives).
2. Add the Turnstile site key (public) to the web app's environment/config and confirm it reaches the client bundle.
3. Implement a Cloudflare siteverify client (POST to `https://challenges.cloudflare.com/turnstile/v0/siteverify` with the secret + token + remote IP) in `services/api` with an explicit timeout and the chosen fail-open/fail-closed behavior from the decision above.
4. Wire that client into `Register()` (`auth.go`) so it actually reads and verifies `msg.TurnstileToken`, returning `invalid_argument` or `failed_precondition` on failure.
5. Change `Contact.turnstileEnforced()` (`contact.go:167`) to return `true` and call the same siteverify client for the anonymous path at `contact.go:104`.
6. Add the Turnstile widget to the registration form (`apps/web/src/app/(site)/register/form.tsx` and `actions.ts`), populating `turnstileToken` in the `RegisterRequest`.
7. Add the Turnstile widget to the contact form's anonymous path (`apps/web/src/app/(site)/contact/form.tsx` and `actions.ts`), populating `turnstileToken` in `SubmitContactRequest`.
8. Add or update tests: a unit test for the siteverify client (mocked HTTP), and handler-level tests for `Register` and `SubmitContact` covering token-missing, token-invalid, and siteverify-unreachable cases per the chosen failure mode.
9. Update `docs/FSD.md` rows for `FR-AUTH-09` and `FR-CNT-22` from "partially shipped" to reflect the new state, and close the corresponding entries in `docs/fsd-open-items.md` and `docs/backlog.md`.
10. If `LoginRequest.turnstile_token` "required after repeated failures" (per the owner's decision #4 above) is in scope, implement the failure-count tracking and the `turnstile_required` `failed_precondition` response in `Login()`; otherwise remove or correct the proto comment at `auth.proto:275-277` so it does not describe unbuilt behavior as a control (the exact failure mode ADR 0031 exists to prevent).

## What could not be verified

- Whether a Turnstile account, site key, or secret key already exists for this domain outside the repository (e.g., in the Cloudflare dashboard or a password manager) — nothing in the repo indicates either way.
- Whether any infrastructure-level bot mitigation (e.g., Cloudflare's own WAF/bot-management rules at the edge, if the site is proxied through Cloudflare at all) provides any de facto protection independent of this application-level Turnstile gap. This document covers only the application code and config in this repository.
- Whether `docker-compose.prod.yml`'s actual deployed state (versus the repo file) differs in any way relevant to secret injection — only the repo file was read.
