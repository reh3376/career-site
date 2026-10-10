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
| `AUTH_LEVEL_MEMBER` | 43 |
| `AUTH_LEVEL_PUBLIC` | 17 |

(44 and 16 until 2026-10-09, when `Logout` was moved to PUBLIC. See S1.)

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
minus the 10 AdminService methods nobody has written yet, and all 15
ungated handlers are the implemented PUBLIC methods. `Logout` was the
sixteenth until it was declared PUBLIC on 2026-10-09; see S1.

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

**Nothing stopped a new method shipping without a level.** The enum's
comment claimed *"Lint fails any method that leaves the level
unspecified"* and no such lint rule or test existed; that none were
missing was care, not a control. **Fixed by S2 on 2026-10-09:** the
server now builds the policy at boot and refuses to start, naming any
method that declares no level. The enum comment was corrected to describe
that instead.

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

**One real disagreement, since resolved: `AuthService.Logout`.** It
declared MEMBER and gates nothing. It revokes whatever session token the caller presents and
clears their own cookie; with no token it is a no-op returning success.
That is defensible, because the token *is* the thing being revoked and
there is no way to log out anybody but yourself. But enforcing MEMBER on
it at S4 would make logout fail with `unauthenticated` exactly when a
session has already expired, which is when somebody most wants their
cookie cleared. **Decision needed before S4: declare it PUBLIC.** It is
**Settled 2026-10-09: declared PUBLIC.** That is what it always was, and
the declaration was the wrong half. `knownUngated` in the test is now
empty, which is the point: every declaration matches its handler with no
carve-outs, so S4 has nothing to special-case.

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

#### The gates do not agree on what a level means

Found while preparing S3, after the comparison above had already passed.
The first version of S1 matched a handler to a gate *by name*, which
answers "is this method gated at all" and nothing more. The gates
themselves check different things:

| gate | session | active status | admin role | handlers |
|---|---|---|---|---|
| `requireAdmin` | yes | **no** | yes | 69 |
| `requireChatAdmin` | yes | yes | yes | 2 |
| `requireMember` | yes | yes | no | 16 |
| a bare `LookupSessionUser` | yes | **no** | no | 3 |

`Login` refuses every non-active status, so a session only exists for an
account that was active when it was minted. The status checks are
therefore about what happens *afterwards*: an admin suspending,
declining or expiring a member mid-session. `requireMember` revokes
access on that member's next call. `requireAdmin` and the three bare
lookups do not.

Concretely, today: a suspended member can still read their own profile
(`GetMe`), read their history (`GetHistory`) and record activity events
(`RecordEvents`), and a suspended admin would keep all 69 admin methods.
The second is theoretical, since there is one admin and he is active.

**This means S4 cannot be behaviour-preserving on every method, whatever
it does.** One definition has to win:

- **MEMBER and ADMIN both require an active status** (recommended). Then
  the three bare-lookup methods and the 69 admin methods get *stricter*,
  which is the safe direction, and nothing is opened. It also makes S5
  safe: removing `requireMember` cannot loosen anything, because the
  interceptor already checks what it checked.
- **MEMBER means session only.** Then nothing changes at S4, because the
  handler checks are still in place, and S5 silently removes the status
  check from 16 methods. That is the dangerous direction, deferred to the
  step least likely to be scrutinised.

The recommendation costs one visible thing: a member whose access was
revoked mid-session currently sees their own `/settings` and `/home`
until they act, and would instead be routed to sign-in. That is arguably
what revoked access should do, and it is the same trade-off
`requireMember` already made for Ask Roger, the JD upload and meetings.

Two tests now pin this, in `authpolicy_test.go`: the per-gate semantics,
parsed from the AST rather than asserted from this table, and that all
**three** definitions of `requireMember` (chat.go, jd.go, meetings.go)
check the same things, which is the precondition for one interceptor
replacing all three. Dropping the status check from any one of them fails
both, naming the file.

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

### S3. The interceptor in observe mode — **built 2026-10-09, awaiting the observation period**

`services/api/internal/authpolicy/interceptor.go`, installed on all
eleven mounted services in `routes()`.

Installed on every Connect handler, computing the decision and
**enforcing nothing**. When its decision would differ from what the
handler did, it logs at warn with the procedure and both answers.

A week of the admin console, the member surfaces and the public decision
test is enough to cover the real paths. S1 proves the static picture;
this proves the dynamic one, including the session edge cases a source
parse cannot see.

*Exit:* a period of real use with no disagreements logged. A single
disagreement stops the sprint and is investigated.

#### The levels, as settled

The owner settled the open question on 2026-10-09: **an active account is
required for both MEMBER and ADMIN.**

```
PUBLIC   no session
MEMBER   a session, on an active account
ADMIN    a session, on an active account, holding the admin role
```

Status is checked before the role, so a suspended admin is refused for
being suspended rather than for lacking a role. This is stricter than 72
of the 105 gated handlers are today, and strictness is the safe
direction: it only ever closes access, and it is what makes S5 safe,
because removing `requireMember` then cannot loosen anything.

#### Keeping the signal clean

Observe mode is worth nothing if its output is noise. Two things would
have made it noise:

**A handler error is not a handler denial.** It may be validating input
or reporting that the database is down. Only `unauthenticated` and
`permission_denied` count as the handler refusing access.

**A denial on a PUBLIC procedure is never about the auth level.** The
interceptor always allows those, so `Login` rejecting a declined account
with `permission_denied` would read as "the handler refused something the
policy allows" and would put a warning in the log on *every failed
sign-in*. PUBLIC procedures are therefore not compared at all, which S1
already justified statically: no PUBLIC handler gates on a session.

What is left is three outcomes, and they are not equally meaningful:

| kind | meaning | level |
|---|---|---|
| `would_close` | the handler served a request the interceptor would refuse | warn |
| `would_open` | the handler demanded a session the interceptor did not | warn |
| `handler_denied_permission` | the interceptor allowed, the handler refused with `permission_denied` | info |

The third is ambiguous on purpose. It may be authorisation the auth level
does not describe, such as "not your resource", so it is recorded without
burying the two that are unambiguous. `would_open` is the dangerous
direction and should never appear.

**One `would_close` is expected** and is the known `Logout` finding: a
logout with no valid session is served today and the interceptor would
refuse it. Seeing it in the log is confirmation that observe mode works
on real traffic, not a new problem.

#### Mode is configuration, not code

`AUTH_INTERCEPTOR_MODE` is `observe` (the default) or `enforce`, so S4 is
a line in `.env.prod` and a redeploy. Anything else is a **boot failure**
rather than a default: a typo selecting `enforce` would deny the admin
console, and a typo selecting `observe` would leave enforcement off while
the deploy meant to turn it on reported success. Neither is acceptable as
a guess.

#### Judging a quiet week

Observe mode went live on `8928c45ef8ac` at 18:15 UTC on 2026-10-09. The
window closes **2026-10-16**.

Declaring `Logout` PUBLIC removed the one disagreement that was expected
to appear, which is the right outcome and creates a problem: **the
expected log for the week is now empty, and an empty log is also what an
interceptor installed on nothing produces.** That is the failure mode
this sprint keeps running into, from the Docker rule that filtered
nothing to the `make breaking` check that compared against a stale
baseline. "No disagreements logged" is not evidence unless something was
compared.

Checked on the first deploy: the rollout's live check makes five
authenticated admin calls, and the log recorded zero disagreements. True,
and it says nothing, because nothing in the log distinguishes five
comparisons from none.

So the interceptor counts what it did and the server logs a summary every
15 minutes, but only when the counters have moved, so an idle night stays
quiet:

```
auth policy observation  reason=periodic mode=observe compared=412
    public_allowed=1983 would_close=0 would_open=0 ambiguous=0
    unknown_procedure=0
```

`compared` is the number that matters: MEMBER and ADMIN calls where the
interceptor reached a decision and the handler then ran, so the two could
be compared. The summary is logged at **warn** when there is anything to
act on and at info otherwise, so the line worth seeing is not the same
severity as the routine one. A final summary is written at shutdown, so a
restart does not lose the window since the last tick.

**S4 is gated on `compared` being a real number with no disagreements,**
not on the date alone. If the week ends with `compared` near zero, the
answer is that the week proved nothing, and the observation continues or
the admin surfaces get exercised deliberately.

#### Cost during S3 and S4

A MEMBER or ADMIN request now resolves the session **twice**: once in the
interceptor and once in the handler's own gate, which stays until S5.
That is one extra query on those paths and none on the public ones, since
the interceptor resolves nothing for a PUBLIC procedure. Deliberate, and
temporary: D4 requires both to be live before either is removed, and the
public decision test, which is the busiest surface, is unaffected.

#### What is tested

Seventeen test functions, in `internal/authpolicy/interceptor_test.go`
and `internal/server/authpolicy_wiring_test.go`, several table-driven so
the executed count is higher. The ones that carry weight:

- **The full decision table in enforce mode**, driven through a real
  Connect server over HTTP with the session arriving as a cookie, so
  `Spec().Procedure` and header propagation are real rather than
  stubbed. Includes both suspended cases, which are the behaviour change
  the owner approved, and the streaming path via
  `ChatService.SendMessage`.
- **That a denial never reaches the handler.** Every refused case asserts
  the stub handler ran zero times. Without it, the handler's own gate
  could be doing the denying and the test would prove nothing about
  enforcement.
- **That observe mode enforces nothing**: every case the enforce table
  refuses must succeed in observe mode, or S3 is not inert.
- **That the interceptor is attached to all eleven mounted services.**
  `routes()` applies it per mount, so there are eleven places to leave it
  off, and the next service added is the one that gets missed. This
  drives real HTTP at a non-public procedure on each, derived from the
  policy map rather than listed, and requires a 401. Several probes land
  on procedures no handler implements, which is the point: Connect would
  answer `unimplemented`, so a 401 proves the interceptor decided first.
  Dropping `opts...` from one mount fails it with `501 unimplemented`,
  naming the service.
- **That a public procedure costs no session lookup**, with a member call
  in the same test to show the resolver is reachable, so the zero means
  "skipped" rather than "never called".
- **The counters and the heartbeat.** `observeInterval` is a var so the
  goroutine is driven at 10ms rather than shipped unexercised, and the
  test waits for a real `reason=periodic` line instead of sleeping. It
  also asserts the summary is not a warning when there is nothing to act
  on, and that nothing starts in enforce mode. Writing it surfaced a
  data race in the test itself, reading the log buffer while the
  heartbeat wrote to it, which only `-race` reports.

Verified by mutation: making `decide` always allow, dropping the active
check, making observe mode enforce, and resolving a session on public
procedures each fail the test that should catch them.

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
can proceed. S2 is inert and can land any time after. **S2 and S3 are
done**; what remains before S4 is calendar time, not work: the
observation period, and the `Logout` declaration. S3 must run for long enough to see
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

1. ~~**Is a week of observe mode right?**~~ **Settled 2026-10-09: one
   week.** Observe mode went live in production on `8928c45ef8ac` at
   18:15 UTC on 2026-10-09, so the window closes **2026-10-16**. What
   closes it is the periodic summary showing a non-trivial number of
   comparisons and no disagreements, not merely the date: see "Judging a
   quiet week" under S3.
2. ~~Is TOTP worth building?~~ **Settled 2026-10-09: no.** The option
   was removed from all 79 methods and deprecated in `options.proto`,
   which `buf breaking` will not let us delete outright. So D1's
   exclusion of `mfa_fresh` is now permanent rather than deferred, and
   the contract no longer describes a control that does not exist.
3. ~~**Does MEMBER require an active account?**~~ **Settled 2026-10-09:
   yes, for both MEMBER and ADMIN.** Implemented in S3's interceptor,
   where status is checked before the role. The original question and its
   reasoning: the gates disagree today:
   `requireMember` revokes access when a member is suspended mid-session,
   the three bare lookups and all 69 `requireAdmin` methods do not. S4
   has to pick one, and the recommendation is that both MEMBER and ADMIN
   require an active status, because it only ever closes access and it is
   what makes S5 safe. The visible cost is that a revoked member is
   routed to sign-in rather than seeing their own settings page. See
   "The gates do not agree on what a level means" under S1.
4. ~~**`Logout`: declare it PUBLIC?**~~ **Settled 2026-10-09: yes.** Done
   in the proto, `knownUngated` is now empty, and the declared split
   moved to 81/43/17.
5. **Does rate limiting matter yet?** 35 methods declare a budget. With
   the current traffic it is theoretical, and it needs a counter store;
   it may be right to drop the option rather than carry it unenforced.
