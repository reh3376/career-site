# 0031. A declared control is an enforced control

Status: Accepted 2026-10-12

## Context

Between 2026-10-08 and 2026-10-12, ten separate controls turned out to
be declared somewhere and applied nowhere. They were not found by a
test or a review; each surfaced because somebody went looking at the
thing itself rather than at what was written about it.

| control | where it was declared | what it did |
|---|---|---|
| UFW allowing only 22, 80, 443 | the firewall config, and the deploy doc | nothing: Docker publishes past UFW, and `DOCKER-USER` was empty |
| `career.v1.auth` on 141 methods | the proto, four documents, an ADR | nothing: enforcement was ~90 hand-written gates, one of which had been forgotten |
| `mfa_fresh` on 79 methods | the proto | nothing, and nothing could: no TOTP existed |
| `rate_limit_per_minute` on 35 methods | the proto | nothing |
| "Lint fails any method that leaves the level unspecified" | the `AuthLevel` enum comment | nothing: no such lint rule existed |
| `AUTH_INTERCEPTOR_MODE` | `.env.prod` | nothing: the variable was never passed to the container |
| `buf.validate` rules on 420 messages | the proto, and `proto/README.md` | nothing at runtime: no validator is wired |
| Turnstile on registration and anonymous contact | the proto and two FSD requirements | nothing: `turnstileEnforced()` returns false, and `Register` never reads the token |
| CSRF via Origin/Referer and `X-Requested-With` | FSD §8.9 | nothing: neither exists |
| `audit_log` | three FSD requirements and the console copy | nothing: no code has ever inserted a row |

Two of those were worse than inert. `proto/README.md` told authors that
invalid requests are rejected before handlers run, "so handlers can
assume well-formed input", which is advice that gets a check skipped.
The FSD's security architecture named three CSRF mechanisms that do not
exist, which is the paragraph a security reviewer reads first.

The common shape is not carelessness. Every one of these was written by
someone describing what the system *would* do, in a place that reads as
a description of what it *does*, and the gap was invisible because the
declaration and the enforcement lived in different files and nothing
compared them.

The auth interceptor is the counter-example and the model. `career.v1.auth`
stopped being documentation when one interceptor started reading the
declaration and deciding from it. The declaration did not get a
companion; it became the mechanism. After that, "is this method
protected" has one answer and it is in one place.

## Decision

**A control that is declared is enforced, or it is not declared.**

Three consequences follow immediately, and the owner settled them on
2026-10-12:

1. **`buf.validate` rules become enforced**, by wiring `protovalidate`
   as an interceptor. The alternative — declaring them documentation
   and spot-testing the ones that carry weight — was rejected: 420
   messages of rules that may or may not have a hand-written
   counterpart is exactly the drift this record is about.
2. **Turnstile is verified** on registration and on anonymous contact,
   or both declarations come out. It is currently declared in the proto,
   required by two FSD rows, and checked nowhere.
3. **`audit_log` is written** on the mutating admin paths, where the
   `reason` fields already exist and are being discarded, or the table
   and the three requirements that promise it are removed.

And a rule for what comes next:

**A control is declared and enforced in the same change.** Where
enforcement is genuinely deferred, the deferral is stated at the point
of declaration — in the proto comment, next to the config field, in the
requirement row — and not only in a planning document that the reader of
the declaration will never open. A reader of the declaration must not be
able to believe it is live when it is not.

**Prefer making the declaration the mechanism** over keeping the two in
sync. Sync is a promise; a single source is a property. `career.v1.auth`
is enforced by reading the descriptor, so a new method is covered the
day it is declared and nobody has to remember.

**Where the two must stay separate, a test compares them.** The checks
that already exist are the pattern: `authpolicy_test.go` compares every
declaration with what its handler does, `compose_env_test.go` fails when
a setting the code reads is not passed to the container,
`TestTheInterceptorIsAttachedToEveryMountedService` drives real HTTP at
every service because counting mount calls would not catch the one that
was missed.

## Consequences

**This costs real work, and the cost is the point.** Enforcing a control
that has been inert changes behaviour, and each of the three above wants
the treatment the auth interceptor got rather than a flip:

- `protovalidate` will start rejecting requests that are currently
  accepted. Some of those requests are real traffic. It needs the
  observe-then-enforce sequence, because "no caller sends an invalid
  request" is a belief, not a measurement.
- Turnstile needs a Cloudflare secret in `.env.prod` and a verification
  call, and it fails closed on an outage unless designed otherwise,
  which is a decision in itself.
- `audit_log` is the cheapest of the three and the easiest to do badly:
  a log nobody reads is another empty control.

**Not everything written down is a control.** A roadmap item, a design
intention, an FSD row describing something unbuilt — none of those are
covered by this record, and treating them as violations would make it
useless. The test is whether a reasonable reader would take the text as
a statement that the system currently does this. `FR-CNT-01` saying the
site "shall present roles chronologically" is a requirement. The
`AuthLevel` comment saying "lint fails any method that leaves the level
unspecified" was a claim about a control, and it was false.

**The prior record of this failure is kept, not tidied away.** Each of
the ten corrections made on 2026-10-11 and 2026-10-12 says what the old
text claimed as well as what is true. A document that quietly becomes
correct teaches nobody; the ones above were wrong for months and the
reason they survived is that nothing in the repository disagreed with
them.

**This record will itself go stale** if the three items above are not
done. They are in `docs/backlog.md`, and until each is resolved this ADR
is a decision and not a description — which is the exact failure mode it
exists to name.
