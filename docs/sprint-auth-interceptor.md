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
in the codebase at all. Enforcement is hand-written, one call per handler
that remembered. Measured by S1's test across the 105 handlers that exist
and are reachable:

| gate | handlers |
|---|---|
| `requireAdmin` | 69 |
| `requireMember` | 16 |
| `requireChatAdmin` | 2 |
| a bare `LookupSessionUser` used to deny | 3 |
| nothing | 15 |

The 71 admin-enforcing handlers are exactly the 81 declared ADMIN methods
minus the 10 AdminService methods nobody has written yet, and 14 of the
15 ungated handlers are the implemented PUBLIC methods. The remaining one
is `Logout`, discussed under S1.

**Three of the four options are enforced nowhere:**

| option | declared on | enforced |
|---|---|---|
| `auth` | 141 methods | by hand, per handler |
| `mfa_fresh` | ~~79 methods~~ **0, removed 2026-10-09** | never, and now declared nowhere |
| `rate_limit_per_minute` | 35 methods | never |
| `allow_unverified` | 3 methods | never |

**Four documents described the interceptor as if it existed.**
`options.proto` said *"The Go server reads these options from the method
descriptor in a Connect interceptor and enforces them uniformly, so no
handler carries its own auth check"*; `proto/README.md` said the server
refuses to start without an auth level; ADR 0017 and FSD D-17 both said
a Connect interceptor enforces the policy before handlers run. Every one
of those was false. All four were corrected on 2026-10-09, before any of
the work below, because they were the kind of claim a security review
takes at face value.

**There is no second factor, and there will not be one.** `mfa_fresh`
was declared on 79 methods with no TOTP implementation anywhere in the
service: the `sessions` table has no verification timestamp and nothing
issues or checks a code. It was unenforceable rather than unenforced.
The owner settled it on 2026-10-09 by deciding not to build a second
factor, so all 79 usages were removed and the option is deprecated.
`buf breaking` will not allow deleting a published extension, so the
field remains in `options.proto`, declared on nothing and marked.

**Nothing stops a new method shipping without a level.** The enum's
comment says *"Lint fails any method that leaves the level
unspecified"*; no such lint rule or test exists. That none are missing
today is luck and care, not a control.

---

## Design decisions

### D1. Scope is the auth level. Not MFA, not rate limiting.

`mfa_fresh` is gone rather than out of scope. It would have needed TOTP
enrolment, a verification timestamp on the session and a recovery path
before it could be enforced at all, and enforcing it before that would
have denied 79 admin methods to the only admin. The owner decided on
2026-10-09 not to build a second factor, so the option was removed from
every method instead of being carried as a control that did not exist.

`rate_limit_per_minute` is out because it needs a counter store and has
entirely different failure modes. A shared counter that is unreachable
must fail *open* or the site goes down; an auth check that is uncertain
must fail *closed*. Putting two opposite failure policies in one
interceptor is how one of them ends up wrong.

`rate_limit_per_minute` stays declared. The API reference and
`options.proto` now state plainly which options are enforced and which
are not, so the contract stops overstating itself either way.

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

### S1. Prove the contract and the code already agree — **done 2026-10-09**

`services/api/internal/handlers/authpolicy_test.go`. It passes, and the
findings are below.

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
S3. No behaviour change. **Met.**

#### What the comparison found

All 141 methods resolve, and they account for themselves exactly:

| | methods |
|---|---|
| declared | 141 (ADMIN 81, MEMBER 44, PUBLIC 16, unspecified 0) |
| on services `server.go` never mounts | 8 |
| declared, mounted, no handler written | 28 |
| handlers found and compared | 105 |

The 81/44/16 split matches the table at the top of this plan, which
matters because this count comes from the registered descriptors rather
than from reading the `.proto` files, so the two numbers are independent.

**Enforcement matches the declaration on all 105.** There is no method
that declares ADMIN and fails to enforce it, which was the outcome worth
checking first: it would have meant an admin method open today.

**One real disagreement: `AuthService.Logout`.** It declares MEMBER and
gates nothing. It revokes whatever session token the caller presents and
clears their own cookie; with no token it is a no-op returning success.
That is defensible, because the token *is* the thing being revoked and
there is no way to log out anybody but yourself. But enforcing MEMBER on
it at S4 would make logout fail with `unauthenticated` exactly when a
session has already expired, which is when somebody most wants their
cookie cleared. **Decision needed before S4: declare it PUBLIC.** It is
recorded in `knownUngated` in the test with that reasoning, so the test
passes today and the choice is not silently lost.

**The 8 unmounted methods are a trap worth naming.** `HomeService`,
`ContentService` and `DownloadService` are declared in the proto and have
generated code, and `server.go` registers none of them, so their routes
do not exist and the methods are unreachable. Unreachable is safe, so
this is not a hole. It is worth knowing because the day one of them is
mounted it arrives with no enforcement whatsoever, and because an
unmounted service otherwise shows up in this comparison as "declares
ADMIN, enforces nothing", which reads alarming and is not true.

**The 28 unwritten methods are pinned by name in the test**, not counted.
That bucket is where a broken matcher drains: a method the AST walk fails
to find is indistinguishable from a method nobody wrote, and a comparison
over nothing passes. Pinning the set makes that a failure. Among them are
`MfaEnroll` and `MfaVerify`, which is the same gap that removing
`mfa_fresh` from 79 methods addressed from the other end.

#### On trusting this result

The check reads both sides from the real artefacts: declared levels from
the registered protobuf descriptors, enforcement from the Go AST. That is
not fastidiousness. The first four versions of it were wrong, each time
in the direction of a confident answer:

1. a regex that matched 0 of 141 handlers, reported as 141 mismatches
2. a regex that matched 89, then 105, each time reporting the rest as
   ungated
3. two admin methods reported as ungated; they call `requireChatAdmin`,
   a gate the pattern did not know about
4. four methods reported as "PUBLIC but enforces MEMBER", the dangerous
   direction; all four use `LookupSessionUser` to *attribute* an
   optional session, not to deny

The last one is why session lookups are now classified by shape rather
than by name. `u, err := Lookup(...)` followed by `if err != nil {
return }` denies; `if u, err := Lookup(...); err == nil && u != nil`
carries on without a user when there is none. Only the first is a gate,
and the second is how four public endpoints legitimately record who
submitted something.

Four deliberate mutations confirm it fails when it should, each checked
for failing with the *right* message and not incidentally: breaking the
AST walk, removing the gate from `Activity.RecordEvents`, mis-mapping a
receiver to the wrong service, and claiming an unwritten method is
written. The `RecordEvents` mutation initially "passed" the mutation test
by failing to compile, which proved nothing; it was redone so it
compiles.

### S2. The policy map, and a server that will not start without one — **done 2026-10-09**

`services/api/internal/authpolicy`, built in `cmd/api/main.go`.

- Build `procedure -> policy` once at startup by walking the registered
  file descriptors and reading the `career.v1.auth` extension.
- Refuse to start if any registered method has no level, naming the
  methods.
- Log the distribution at boot, so the counts above are visible rather
  than discovered by reading the proto.
- No interception yet.

*Exit:* the server starts, logs 81/44/16, and fails to start against a
deliberately unannotated method in a test. **Met.**

#### What it does

The map is keyed by `connect.Spec.Procedure`
(`/career.v1.AdminService/ListMembers`), so the S3 interceptor does one
lookup with no string surgery and cannot derive the key differently from
how it was built. `Level()` reports an unknown procedure as unknown
rather than as a level, which is what lets the interceptor fail closed
per D2.

It is built in `main` immediately after config load, before migrations
and before any connection is opened, so an incomplete policy is the
cheapest possible failure. It exits rather than warns: once S4 is live,
"no policy" has to mean denied, and a server that started anyway would
serve that method as a hole or a 500.

This also makes a claim in `options.proto` true for the first time. The
`AuthLevel` enum's comment said *"Lint fails any method that leaves the
level unspecified"* and no such lint rule ever existed. Now it is a boot
failure instead, which is a stronger control than a lint rule and runs in
the one place that matters.

The boot log, from the real descriptors:

```
auth policy loaded   policy="141 methods: admin=81 member=44 public=16"
auth policy coverage policy="141 methods: ..." mounted_services=11 procedures_served=133
declared services that are not mounted, so unreachable
                     services=career.v1.ContentService,career.v1.DownloadService,career.v1.HomeService
```

The third line is S1's unmounted-service finding turned into a runtime
signal rather than a note in this document. A mounted service with *no*
policy entries logs at **error**, because once S4 enforces, every
procedure on it would be denied and the cause should not have to be
inferred from a wall of `permission_denied`.

#### What is tested

Ten tests, and two of them exist because the alternative was asserting
behaviour by reading it:

- The exact distribution, 81/44/16, asserted rather than floored. "At
  least 141" would be satisfied by a map that had quietly gained a
  service.
- The key format, pinned against one real procedure per level, so a map
  that collapsed every method to one level fails.
- **Fail-to-start**, run against a synthetic registry holding a
  `career.v1` service with one annotated and one unannotated method,
  since every method in the real binary declares a level. The test also
  checks the error names the offending method and not the healthy one: an
  error that says "some method is missing a level" is useless on a
  141-method API at boot during a deploy. It guards itself too, by
  asserting the annotated method really does read as ADMIN, so the test
  cannot pass by having both methods read as unannotated.
- **An empty registry is an error, not a clean result.** If the
  descriptors were not linked in, the walk finds nothing, reports no
  missing levels, and looks exactly like success; enforcing on that map
  would deny every request for an invisible reason.
- The coverage log, asserted on its real output, including that a mounted
  service with no policy is reported at error level.
- The production shape, with all eleven services mounted, pinning
  `mounted_services=11` and `procedures_served=133` and that the
  unmounted set is exactly Content, Download and Home. 133 is the number
  of procedures the S4 interceptor will decide on, so it should not drift
  quietly.

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

S1 gates everything; nothing else starts until its list is empty. **S1 is
done and its list has one item, `Logout`,** which is a declaration change
rather than a code change and is only load-bearing at S4, so S2 and S3
can proceed. S2 is inert and can land any time after. S3 must run for long enough to see
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
2. ~~Is TOTP worth building?~~ **Settled 2026-10-09: no.** The option
   was removed from all 79 methods and deprecated in `options.proto`,
   which `buf breaking` will not let us delete outright. So D1's
   exclusion of `mfa_fresh` is now permanent rather than deferred, and
   the contract no longer describes a control that does not exist.
3. **`Logout`: declare it PUBLIC?** S1's one finding. It declares MEMBER
   and gates nothing, deliberately. The recommendation is to change the
   declaration to PUBLIC, because the alternative is logout failing for
   people whose session has already expired. Needed before S4, and it is
   a one-line proto change plus removing the `knownUngated` entry.
4. **Does rate limiting matter yet?** 35 methods declare a budget. With
   the current traffic it is theoretical, and it needs a counter store;
   it may be right to drop the option rather than carry it unenforced.
