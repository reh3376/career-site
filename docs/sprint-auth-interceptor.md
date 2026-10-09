# Sprint plan: the auth interceptor

**No code written.** This is the plan; the steps below say what gets
built and in what order.

The backlog entry: *an auth interceptor, so the proto's `AUTH_LEVEL_*`
becomes enforcement rather than documentation.*

It is the same shape as the Docker/UFW gap found on 2026-10-08. A
control exists, it is written down, it reads as protection, and nothing
is actually applying it. There the firewall described a rule it was not
enforcing; here the contract describes an interceptor that does not
exist.

---

## What is true today, checked rather than assumed

**Every method declares a level.** 141 of them, and none missing:

| level | methods |
|---|---|
| `AUTH_LEVEL_ADMIN` | 81 |
| `AUTH_LEVEL_MEMBER` | 44 |
| `AUTH_LEVEL_PUBLIC` | 16 |

**Nothing reads those declarations.** There are no Connect interceptors
in the codebase at all. Enforcement is 81 hand-written `requireAdmin`
calls and 17 `LookupSessionUser` call sites, one per handler that
remembered.

**Three of the four options are enforced nowhere:**

| option | declared on | enforced |
|---|---|---|
| `auth` | 141 methods | by hand, per handler |
| `mfa_fresh` | **79 methods** | never |
| `rate_limit_per_minute` | 35 methods | never |
| `allow_unverified` | 3 methods | never |

**`options.proto` describes the interceptor in the present tense.** Its
own doc comment reads: *"The Go server reads these options from the
method descriptor in a Connect interceptor and enforces them uniformly,
so no handler carries its own auth check."* Every clause of that is
currently false. It is the same kind of untrue claim as the run page's
line about the answer key, and it should be fixed whether or not the
rest of this is built.

**There is no second factor.** `mfa_fresh` is declared on 79 methods and
there is no TOTP implementation anywhere in the service: the `sessions`
table has no verification timestamp and nothing issues or checks a code.
So that option is not merely unenforced, it is unenforceable until a
second factor exists.

**Nothing stops a new method shipping without a level.** The enum's
comment says *"Lint fails any method that leaves the level
unspecified"*; no such lint rule or test exists. That none are missing
today is luck and care, not a control.

---

## Design decisions

### D1. Scope is the auth level. Not MFA, not rate limiting.

`mfa_fresh` is out because the factor does not exist. Turning it on
today would deny 79 admin methods to the only admin, which is a lockout
rather than a hardening. It needs TOTP enrolment, a verification
timestamp on the session and a recovery path first, and that is its own
piece of work.

`rate_limit_per_minute` is out because it needs a counter store and has
entirely different failure modes. A shared counter that is unreachable
must fail *open* or the site goes down; an auth check that is uncertain
must fail *closed*. Putting two opposite failure policies in one
interceptor is how one of them ends up wrong.

Both stay declared in the contract. The plan makes the API reference
state plainly which options are enforced and which are aspirational, so
the contract stops overstating itself.

### D2. Fail closed, and refuse to start rather than guess

A procedure with no policy must be **denied**, not allowed. The whole
value of this change is removing the possibility that something is
unprotected by omission, and a default-allow interceptor reintroduces it
in one line.

Stronger still: the policy map is built once at startup from the
registered file descriptors, and **the server refuses to start** if any
registered method lacks a level. That turns the enum comment's claim
into something real. A missing level becomes a boot failure on a deploy,
which is loud, immediate and happens before any traffic, rather than a
method quietly serving the public.

### D3. The interceptor authenticates; handlers stop deciding

`requireAdmin` does two jobs: it decides, and it returns the
`*users.User` the handler then uses. Only the first moves.

The interceptor resolves the session once, enforces the level, and puts
the user in the request context. Handlers that need the user read it
from there. Without this, removing the handler checks would mean
handlers losing access to the user and each doing its own lookup anyway,
which is the same duplication with none of the enforcement.

### D4. Both enforce before either is removed

The interceptor goes in while every handler check stays exactly where it
is. For a period both are live and must agree. Only once that has held
do the handler checks come out.

This is not caution for its own sake. Switching 141 methods from one
enforcement model to another in a single change has two failure modes
and they are not symmetric: breaking the admin console is obvious within
a minute, and quietly making an admin method public is not. The overlap
makes the second impossible during the transition.

### D5. The non-Connect endpoints are out of scope, and stay that way

Three routes bypass Connect entirely and so cannot be covered by a
Connect interceptor:

- `POST /api/admin/decision` — one-click approve/decline, where the
  token in the link is the authentication
- `GET /api/jd/resume/{file}` — the result token is the authentication
- `GET /api/meetings/{file}` — an ICS file a calendar client fetches

Each is a deliberate token-as-auth design for something that has to work
from an email client or a calendar app with no session. They are named
here so that "all auth is in the interceptor" is never believed without
qualification.

---

## The steps

### S1. Prove the contract and the code already agree

Before anything changes behaviour, a test that reads both sides and
compares them: every method's declared level against what its handler
actually enforces.

This is the step that makes the rest safe, and it is first because its
failure mode is the dangerous one. If some method declares
`AUTH_LEVEL_ADMIN` while its handler never calls `requireAdmin`, then
today it is an open method and turning on the interceptor **closes** it,
which looks like the interceptor breaking something. If the reverse,
turning on the interceptor **opens** it. Either way the disagreement has
to be found and resolved before enforcement moves, not after.

The existing `admin_authgate_test.go` already does a narrow version of
this for the 81 admin methods. This generalises it: parse the proto for
declared levels, parse the handlers for enforcement, and report every
method where the two disagree.

*Exit:* the comparison runs over all 141 methods and either passes or
produces a list, and every disagreement on that list is resolved before
S3. No behaviour change.

### S2. The policy map, and a server that will not start without one

- Build `procedure -> policy` once at startup by walking the registered
  file descriptors and reading the `career.v1.auth` extension.
- Refuse to start if any registered method has no level, naming the
  methods.
- Log the distribution at boot, so the counts above are visible rather
  than discovered by reading the proto.
- No interception yet.

*Exit:* the server starts, logs 81/44/16, and fails to start against a
deliberately unannotated method in a test.

### S3. The interceptor in observe mode

Installed on every Connect handler, computing the decision and
**enforcing nothing**. When its decision would differ from what the
handler did, it logs at warn with the procedure and both answers.

A week of the admin console, the member surfaces and the public decision
test is enough to cover the real paths. S1 proves the static picture;
this proves the dynamic one, including the session edge cases a source
parse cannot see.

*Exit:* a period of real use with no disagreements logged. A single
disagreement stops the sprint and is investigated.

### S4. Enforce

Flip observe mode off. The interceptor now denies, with
`unauthenticated` where there is no session and `permission_denied`
where the session lacks the role, which is what the enum already
promises.

Handler checks stay in place and become unreachable: they cannot fire,
because nothing that would trip them now reaches the handler.

*Exit:* the admin console, a member surface and the public decision test
all work on production; an unauthenticated call to an admin procedure
returns `unauthenticated`; a member session calling an admin procedure
returns `permission_denied`. Verified by actually making those calls,
not by reading the code.

### S5. Remove the handler checks

81 `requireAdmin` calls and the member-side lookups come out, replaced
by reading the user from context where the handler needs it.

The auth-gate test inverts: it stops asserting that every admin method
calls `requireAdmin` and starts asserting that none of them do, because
after this a handler carrying its own check is a handler that might
disagree with the interceptor.

*Exit:* no handler decides its own access; the API reference and
`options.proto` describe what the code does.

### S6. Make the contract honest about the rest

`options.proto`'s doc comment is corrected to say which options are
enforced and which are declared only, and `docs/api/README.md` carries
the same distinction. `mfa_fresh` on 79 methods reading as a live
control when no second factor exists is the kind of thing that gets
believed in a security review.

*Exit:* nothing in the contract claims an enforcement that does not
exist.

---

## Order and risk

S1 gates everything; nothing else starts until its list is empty. S2 is
inert and can land any time after. S3 must run for long enough to see
real traffic, which is the only part of this that takes calendar time
rather than work. S4 is the behaviour change. S5 is cleanup and is
optional in the sense that the system is correct without it, though
leaving two enforcement paths in place indefinitely recreates the
original problem in a new form.

**The risk that matters** is not breaking the admin console; that is
obvious and recoverable in one rollback. It is a method becoming more
open than it was, silently. D2's fail-closed default, S1's comparison
and S4's overlap with the existing checks each exist to make that
specific outcome impossible rather than unlikely.

**Second risk: locking the owner out.** There is one admin and no second
factor. Every step keeps the existing checks working until the new path
is proven, and `--rollback` restores the previous image in a single
command.

## Decisions that are the owner's

1. **Is a week of observe mode right?** Shorter gets to enforcement
   sooner on a site with few users; longer sees more of the admin
   surface, much of which is used rarely.
2. **Is TOTP worth building?** It decides whether `mfa_fresh` ever
   becomes real or should be removed from the 79 methods that declare
   it. A contract that permanently describes an unenforced control is
   worse than one that does not mention it.
3. **Does rate limiting matter yet?** 35 methods declare a budget. With
   the current traffic it is theoretical, and it needs a counter store;
   it may be right to drop the option rather than carry it unenforced.
