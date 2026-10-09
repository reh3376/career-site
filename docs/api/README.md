# career-site API reference

> Generated from `proto/` by `scripts/gen_api_docs.py`. Do not edit by hand — change the `.proto` files and run `make docs-api`. CI fails when this file drifts from the contracts.

The site's API is a set of [ConnectRPC](https://connectrpc.com) services defined in Protobuf under `proto/career/v1/`. The Go server implements them; the Next.js app calls them through the generated TypeScript client in `apps/web/src/gen/`. The same definitions drive request validation, the auth interceptor, this reference, and the UATS contract specs, so the contract is declared once.

## Conventions

### Transport

| Aspect | Rule |
|---|---|
| Base path | `/api` (same origin as the site; Caddy routes it to the Go API) |
| Method URL | `POST /api/{package}.{Service}/{Method}`, e.g. `POST /api/career.v1.AuthService/Login` |
| Encoding | JSON (`Content-Type: application/json`); the TypeScript client can switch to binary Protobuf with `useBinaryFormat: true` without any server change |
| Required header | `Connect-Protocol-Version: 1` — the generated clients add it; `curl` must add it by hand. Being a non-simple header it also blocks cross-site request forgery, since a cross-origin page cannot send it without a CORS preflight that the server refuses |
| Streaming | Server-streaming methods (`ChatService.SendMessage`) return a stream of enveloped messages over the same `POST`; the TypeScript client exposes it as an `AsyncIterable`, so the UI never handles framing |
| Timeouts | Clients may send `Connect-Timeout-Ms`; the server caps unary calls at 30 s and the chat stream at 120 s |
| Versioning | The package is `career.v1`; additive changes (new fields, methods, enum values) do not bump it; `buf breaking` in CI blocks incompatible changes |

### JSON encoding of Protobuf

| Protobuf | JSON | Notes |
|---|---|---|
| field names | lowerCamelCase | `stated_role` → `statedRole`; the tables below show the JSON names |
| `int64`, `uint64` | string | e.g. `"sizeBytes": "123456"` |
| `float`, `double`, `int32` | number | |
| enum | string name | e.g. `"type": "CONTENT_TYPE_PROJECT"`; unknown names are rejected |
| `google.protobuf.Timestamp` | RFC 3339 string, UTC | `"2026-09-18T12:00:00Z"` |
| `google.protobuf.FieldMask` | comma-separated paths | `"updateMask": "name,profile.seniority"` |
| `bytes` | base64 string | |
| `oneof` | at most one member present | sending two is `invalid_argument` |
| unset / default values | omitted from responses | clients must treat a missing field as the default (`""`, `0`, `false`, `[]`) |

### Authentication and sessions

- Sessions live in the `career_session` cookie (`HttpOnly; Secure; SameSite=Lax`; 30-day absolute lifetime, 7-day idle timeout). The browser sends it automatically on same-origin requests; the client never handles a token.
- **Auth levels** (declared per method in the `.proto` and enforced by an interceptor before the handler runs):
  - **Public** — no session.
  - **Member** — a session whose member is `ACTIVE`. Methods marked *unverified OK* also accept a session whose member is `UNVERIFIED` (verification, resend, sign-out, deletion).
  - **Admin** — a session with role `ADMIN`; methods marked *fresh MFA* additionally require a TOTP verification within the freshness window (default 12 hours), otherwise `permission_denied` with reason `mfa_fresh_required`.
- Sign-in and verification responses set the cookie; `Logout` clears it. Social sign-in is a browser redirect flow (see *Plain HTTP endpoints*).
- Rate limits are per account (per IP for public methods). Responses carry `RateLimit-Limit` and `RateLimit-Remaining`; a `resource_exhausted` error carries `Retry-After`.

### Errors

Errors are Connect errors: a non-200 status and a JSON body `{"code": "...", "message": "...", "details": [...]}`. Branch on `code`, not on the HTTP status. Validation failures carry a `buf.validate.Violations` detail listing every violated field and rule.

| `code` | HTTP | When |
|---|---|---|
| `invalid_argument` | 400 | Request failed validation; `details` carries `buf.validate.Violations` listing each field |
| `unauthenticated` | 401 | No session, expired session, or unverified member calling a verified-only method |
| `permission_denied` | 403 | Session lacks the role, or admin MFA is not fresh |
| `not_found` | 404 | Unknown conversation, content item, export, or job |
| `already_exists` | 409 | Duplicate where duplicates are rejected |
| `failed_precondition` | 412 | State does not allow the call (e.g. `turnstile_required`, questionnaire pending) |
| `resource_exhausted` | 429 | Rate limit or assistant quota exceeded; honor `Retry-After` |
| `unavailable` | 503 | A dependency is down; retry with backoff |
| `deadline_exceeded` | 408 | Server-side deadline exceeded |
| `unimplemented` | 404 | Method reserved for a later phase |
| `internal` | 500 | Bug; the response `message` carries a request ID to quote |

### Plain HTTP endpoints

Browsers navigate to these, so they are ordinary `GET` handlers rather than RPCs.

| Endpoint | Auth | Behavior |
|---|---|---|
| `GET /api/healthz` | Public | `200 {"status":"ok"}` when the process is up |
| `GET /api/readyz` | Public | `200 {"status":"ok","checks":{...},"version":"..."}` when database, object storage, and sidecar are reachable; `503` with the failing checks otherwise |
| `GET /api/auth/oauth/{provider}/start` | Public | Redirects to LinkedIn (v1; `github` and `google` later) with PKCE and `state` |
| `GET /api/auth/oauth/{provider}/callback` | Public | Completes the exchange, creates or links the account by verified email, sets the session cookie, redirects to `/me/interests` or `/home` |
| `GET /api/downloads/{variant}` | Member | Records the download and `302`s to a short-lived signed URL; `401` without a session, `404` for an unknown variant |

### TypeScript quick start

```ts
// apps/web: one transport for the whole app
import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";
import { AuthService } from "@/gen/career/v1/auth_pb";
import { ChatService } from "@/gen/career/v1/chat_pb";

const transport = createConnectTransport({ baseUrl: "/api" }); // same origin; cookie sent automatically

const auth = createClient(AuthService, transport);
const { me, questionnairePending } = await auth.login({ email, password });

// Streaming chat: iterate the events as they arrive
const chat = createClient(ChatService, transport);
let text = "";
for await (const res of chat.sendMessage({ conversationId, text: question })) {
  switch (res.event.case) {
    case "start":     /* res.event.value.assistantMessageId */ break;
    case "delta":     text += res.event.value.text; break;
    case "citations": /* res.event.value.citations */ break;
    case "usage":     /* res.event.value.remainingToday */ break;
    case "done":      /* res.event.value.message — the stored message */ break;
  }
}
```

Errors surface as `ConnectError` with `.code` (a `Code` enum) and `.findDetails(ViolationsSchema)` for validation details. Server components can call the same client with the incoming request's cookie forwarded in a custom fetch.

### curl

```sh
curl -sS -X POST https://<host>/api/career.v1.SystemService/GetVersion \
  -H 'Content-Type: application/json' -H 'Connect-Protocol-Version: 1' -d '{}'
```

---

## Services

| Service | Purpose | Methods |
|---|---|---|
| [`AuthService`](#authservice) | Registration, verification, sign-in, and credential management. | 12 |
| [`MemberService`](#memberservice) | Profile, interests, history, saved items, export, and deletion for the calling member. | 10 |
| [`ContentService`](#contentservice) | Read access to the content catalog. | 6 |
| [`HomeService`](#homeservice) | One-call assembly of everything the home page renders. | 1 |
| [`DownloadService`](#downloadservice) | Lists what can be downloaded. | 1 |
| [`ContactService`](#contactservice) | Reaching the owner outside the assistant. | 2 |
| [`JdService`](#jdservice) | JD-upload flow, members only: a signed-in session is required to submit or poll, and the submitting member is recorded on the row. | 4 |
| [`MeetingService`](#meetingservice) | Meeting scheduling, members only: a signed-in session is required to see availability or to book, and the booking member is recorded on the row. | 5 |
| [`ChatService`](#chatservice) | Conversations with the assistant. | 11 |
| [`ActivityService`](#activityservice) | Batched, fire-and-forget activity reporting. | 1 |
| [`EventService`](#eventservice) | Accepts browser-minted events. | 1 |
| [`AdminService`](#adminservice) | Owner console. | 79 |
| [`SystemService`](#systemservice) | Version and governance status. | 3 |
| [`DecisionTestService`](#decisiontestservice) | Runs one sitting of the decision test. | 5 |
| [`SidecarService`](#sidecarservice) | Embedding, reranking, classification, and batch jobs. _(internal)_ | 8 |

## AuthService

Registration, verification, sign-in, and credential management.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`Register`](#authservice-register) | `/api/career.v1.AuthService/Register` | Public | default | `RegisterRequest` → `RegisterResponse` | Creates an unverified account and sends a verification email. |
| [`Verify`](#authservice-verify) | `/api/career.v1.AuthService/Verify` | Public | default | `VerifyRequest` → `VerifyResponse` | Confirms control of the email address with the emailed link token or the six-digit code, activates the account, and starts a session (cookie set in the response). |
| [`ResendVerification`](#authservice-resendverification) | `/api/career.v1.AuthService/ResendVerification` | Public | default | `ResendVerificationRequest` → `ResendVerificationResponse` | Re-sends the verification email for an unverified account. |
| [`Login`](#authservice-login) | `/api/career.v1.AuthService/Login` | Public | default | `LoginRequest` → `LoginResponse` | Signs in with email and password and sets the session cookie. |
| [`Logout`](#authservice-logout) | `/api/career.v1.AuthService/Logout` | Public (unverified OK) | default | `LogoutRequest` → `LogoutResponse` | Ends the current session and clears the cookie. |
| [`LogoutAll`](#authservice-logoutall) | `/api/career.v1.AuthService/LogoutAll` | Member | default | `LogoutAllRequest` → `LogoutAllResponse` | Ends every session of the member ("sign out of all devices"). |
| [`ForgotPassword`](#authservice-forgotpassword) | `/api/career.v1.AuthService/ForgotPassword` | Public | default | `ForgotPasswordRequest` → `ForgotPasswordResponse` | Emails a single-use password-reset link (valid one hour). |
| [`ResetPassword`](#authservice-resetpassword) | `/api/career.v1.AuthService/ResetPassword` | Public | default | `ResetPasswordRequest` → `ResetPasswordResponse` | Sets a new password using a reset token and signs the member in on success. |
| [`ChangePassword`](#authservice-changepassword) | `/api/career.v1.AuthService/ChangePassword` | Member | default | `ChangePasswordRequest` → `ChangePasswordResponse` | Changes the password of the signed-in member; requires the current password. |
| [`ChangeEmail`](#authservice-changeemail) | `/api/career.v1.AuthService/ChangeEmail` | Member | default | `ChangeEmailRequest` → `ChangeEmailResponse` | Starts an email change: a verification email goes to the new address and the change applies when it is confirmed via Verify. |
| [`MfaEnroll`](#authservice-mfaenroll) | `/api/career.v1.AuthService/MfaEnroll` | Member | default | `MfaEnrollRequest` → `MfaEnrollResponse` | Enrols TOTP MFA for an admin account that has none. |
| [`MfaVerify`](#authservice-mfaverify) | `/api/career.v1.AuthService/MfaVerify` | Member | default | `MfaVerifyRequest` → `MfaVerifyResponse` | Verifies a TOTP code (or a recovery code) for the current admin session. |

### AuthService.Register

`POST /api/career.v1.AuthService/Register` · **Auth:** Public · **Rate limit:** default/min

Creates an unverified account and sends a verification email. The
response is identical whether or not the email address already exists,
to prevent account enumeration; an existing verified account receives a
"someone tried to register with your address" email instead.

**Request** — [`RegisterRequest`](#registerrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string | `string: min_len: 1 max_len: 120` | Display name. |
| `email` | `string` | string | `string: max_len: 254 email: true` | Email address; becomes the sign-in identifier. |
| `password` | `string` | string | `string: min_len: 12 max_len: 256` | Password: at least 12 characters, no composition rules; rejected if it appears in a breached-password list. |
| `organization` | `string` | string | `string: max_len: 200` | Organization, optional. |
| `statedRole` | `string` | string | `string: max_len: 120` | Stated role or title, optional. |
| `consentVersion` | `string` | string | `string: min_len: 1 max_len: 32` | Version identifier of the consent text shown; must match the current version served on the registration page. |
| `consentAccepted` | `bool` | boolean | `bool: const: true` | Explicit acknowledgement that activity and assistant conversations are stored and visible to the owner. Must be true. |
| `turnstileToken` | `string` | string | `string: min_len: 1 max_len: 2048` | Cloudflare Turnstile response token from the registration page. |

**Response** — [`RegisterResponse`](#registerresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | `string` | string |  | Fixed text for display: "Check your email for a verification link." |

<details><summary>Example request body</summary>

```json
{
  "name": "string",
  "email": "string",
  "password": "string",
  "organization": "string",
  "statedRole": "string",
  "consentVersion": "string",
  "consentAccepted": true,
  "turnstileToken": "string"
}
```

</details>

### AuthService.Verify

`POST /api/career.v1.AuthService/Verify` · **Auth:** Public · **Rate limit:** default/min

Confirms control of the email address with the emailed link token or the
six-digit code, activates the account, and starts a session (cookie set
in the response).

**Request** — [`VerifyRequest`](#verifyrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `token` | `string` | string | `string: min_len: 16 max_len: 256` | _(oneof `credential`)_ Single-use token from the emailed link. |
| `code` | `string` | string | `string: pattern: "^[0-9]{6$"` | _(oneof `credential`)_ Six-digit code from the email; requires `email`. |
| `email` | `string` | string | `string: max_len: 254` | Email address the code was sent to; required with `code`, ignored with `token`. |

**Response** — [`VerifyResponse`](#verifyresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The now-active member. |
| `questionnairePending` | `bool` | boolean |  | True when the first-visit questionnaire has not been completed; the client should route to `/me/interests`. |

<details><summary>Example request body</summary>

```json
{
  "token": "string",
  "email": "string"
}
```

</details>

### AuthService.ResendVerification

`POST /api/career.v1.AuthService/ResendVerification` · **Auth:** Public · **Rate limit:** default/min

Re-sends the verification email for an unverified account. Identical
response whether or not the address is known.

**Request** — [`ResendVerificationRequest`](#resendverificationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `email` | `string` | string | `string: max_len: 254 email: true` | Email address of the unverified account. |

**Response** — [`ResendVerificationResponse`](#resendverificationresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | `string` | string |  | Fixed text for display. |

<details><summary>Example request body</summary>

```json
{
  "email": "string"
}
```

</details>

### AuthService.Login

`POST /api/career.v1.AuthService/Login` · **Auth:** Public · **Rate limit:** default/min

Signs in with email and password and sets the session cookie. For an
admin account the session is created but flagged `mfa_required`; admin
methods stay unavailable until MfaVerify succeeds.

**Request** — [`LoginRequest`](#loginrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `email` | `string` | string | `string: max_len: 254 email: true` | Email address. |
| `password` | `string` | string | `string: min_len: 1 max_len: 256` | Password. |
| `turnstileToken` | `string` | string | `string: max_len: 2048` | Turnstile token; required after repeated failures for the account or IP (the server responds `failed_precondition` with reason `turnstile_required` when it is missing but needed). |

**Response** — [`LoginResponse`](#loginresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The signed-in member. `status` is UNVERIFIED for an account that has not confirmed its email; such a session can only call verification methods. |
| `mfaRequired` | `bool` | boolean |  | True for an admin session that must complete MfaVerify (or MfaEnroll then MfaVerify) before admin methods are available. |
| `questionnairePending` | `bool` | boolean |  | True when the first-visit questionnaire has not been completed. |

<details><summary>Example request body</summary>

```json
{
  "email": "string",
  "password": "string",
  "turnstileToken": "string"
}
```

</details>

### AuthService.Logout

`POST /api/career.v1.AuthService/Logout` · **Auth:** Public (unverified OK) · **Rate limit:** default/min

Ends the current session and clears the cookie.

Public, not MEMBER. It declared MEMBER until 2026-10-09 and the
handler never enforced it, which S1 of
docs/sprint-auth-interceptor.md found. The declaration was the
wrong half: there is nothing here to protect, because the session
token the caller presents IS the thing being revoked, so nobody
can log out anyone but themselves. With no token it is a no-op that
returns success.

Requiring a session would also break the case that needs this
most. Once a session has expired the cookie is still in the
browser, and that is exactly when someone wants it cleared;
enforcing MEMBER would answer `unauthenticated` and leave it there.

**Request** — [`LogoutRequest`](#logoutrequest)

_No fields; send `{}`._

**Response** — [`LogoutResponse`](#logoutresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AuthService.LogoutAll

`POST /api/career.v1.AuthService/LogoutAll` · **Auth:** Member · **Rate limit:** default/min

Ends every session of the member ("sign out of all devices").

**Request** — [`LogoutAllRequest`](#logoutallrequest)

_No fields; send `{}`._

**Response** — [`LogoutAllResponse`](#logoutallresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `revoked` | `int32` | number |  | Sessions revoked, including the current one. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AuthService.ForgotPassword

`POST /api/career.v1.AuthService/ForgotPassword` · **Auth:** Public · **Rate limit:** default/min

Emails a single-use password-reset link (valid one hour). Identical
response whether or not the address is known.

**Request** — [`ForgotPasswordRequest`](#forgotpasswordrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `email` | `string` | string | `string: max_len: 254 email: true` | Email address of the account. |

**Response** — [`ForgotPasswordResponse`](#forgotpasswordresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | `string` | string |  | Fixed text for display. |

<details><summary>Example request body</summary>

```json
{
  "email": "string"
}
```

</details>

### AuthService.ResetPassword

`POST /api/career.v1.AuthService/ResetPassword` · **Auth:** Public · **Rate limit:** default/min

Sets a new password using a reset token and signs the member in on
success. All other sessions are revoked.

**Request** — [`ResetPasswordRequest`](#resetpasswordrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `token` | `string` | string | `string: min_len: 16 max_len: 256` | Single-use token from the reset email. |
| `newPassword` | `string` | string | `string: min_len: 12 max_len: 256` | New password, same rules as registration. |

**Response** — [`ResetPasswordResponse`](#resetpasswordresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The signed-in member. |

<details><summary>Example request body</summary>

```json
{
  "token": "string",
  "newPassword": "string"
}
```

</details>

### AuthService.ChangePassword

`POST /api/career.v1.AuthService/ChangePassword` · **Auth:** Member · **Rate limit:** default/min

Changes the password of the signed-in member; requires the current
password. Other sessions are revoked.

**Request** — [`ChangePasswordRequest`](#changepasswordrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `currentPassword` | `string` | string | `string: min_len: 1 max_len: 256` | Current password. |
| `newPassword` | `string` | string | `string: min_len: 12 max_len: 256` | New password, same rules as registration. |

**Response** — [`ChangePasswordResponse`](#changepasswordresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `revoked` | `int32` | number |  | Other sessions revoked. |

<details><summary>Example request body</summary>

```json
{
  "currentPassword": "string",
  "newPassword": "string"
}
```

</details>

### AuthService.ChangeEmail

`POST /api/career.v1.AuthService/ChangeEmail` · **Auth:** Member · **Rate limit:** default/min

Starts an email change: a verification email goes to the new address and
the change applies when it is confirmed via Verify. Requires the current
password (or, for social-only accounts, a fresh social sign-in within
the last ten minutes).

**Request** — [`ChangeEmailRequest`](#changeemailrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `newEmail` | `string` | string | `string: max_len: 254 email: true` | New email address; a verification email is sent there. |
| `currentPassword` | `string` | string | `string: max_len: 256` | Current password; required unless the account is social-only. |

**Response** — [`ChangeEmailResponse`](#changeemailresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | `string` | string |  | Fixed text for display. |

<details><summary>Example request body</summary>

```json
{
  "newEmail": "string",
  "currentPassword": "string"
}
```

</details>

### AuthService.MfaEnroll

`POST /api/career.v1.AuthService/MfaEnroll` · **Auth:** Member · **Rate limit:** default/min

Enrols TOTP MFA for an admin account that has none. Returns the shared
secret as an otpauth URI plus one-time recovery codes; enrolment
completes with the first successful MfaVerify.

**Request** — [`MfaEnrollRequest`](#mfaenrollrequest)

_No fields; send `{}`._

**Response** — [`MfaEnrollResponse`](#mfaenrollresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `otpauthUri` | `string` | string |  | otpauth:// URI for authenticator apps. |
| `secretBase32` | `string` | string |  | The same secret in Base32 for manual entry. |
| `recoveryCodes` | `string`[] | array of string |  | One-time recovery codes. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AuthService.MfaVerify

`POST /api/career.v1.AuthService/MfaVerify` · **Auth:** Member · **Rate limit:** default/min

Verifies a TOTP code (or a recovery code) for the current admin session.

**Request** — [`MfaVerifyRequest`](#mfaverifyrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `code` | `string` | string | `string: min_len: 6 max_len: 32` | Six-digit TOTP code or a recovery code. |

**Response** — [`MfaVerifyResponse`](#mfaverifyresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The admin member with `mfa_enrolled` true. |
| `recoveryCodesRemaining` | `int32` | number |  | Recovery codes remaining, when a recovery code was used. |

<details><summary>Example request body</summary>

```json
{
  "code": "string"
}
```

</details>

## MemberService

Profile, interests, history, saved items, export, and deletion for the
calling member. Every method operates on the session's own member; there is
no member ID parameter.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`GetMe`](#memberservice-getme) | `/api/career.v1.MemberService/GetMe` | Member (unverified OK) | default | `GetMeRequest` → `GetMeResponse` | Returns the calling member. |
| [`UpdateMe`](#memberservice-updateme) | `/api/career.v1.MemberService/UpdateMe` | Member | default | `UpdateMeRequest` → `UpdateMeResponse` | Updates profile fields named in `update_mask`. |
| [`SetInterests`](#memberservice-setinterests) | `/api/career.v1.MemberService/SetInterests` | Member | default | `SetInterestsRequest` → `SetInterestsResponse` | Replaces the member's track interests and tailoring setting. |
| [`GetHistory`](#memberservice-gethistory) | `/api/career.v1.MemberService/GetHistory` | Member | default | `GetHistoryRequest` → `GetHistoryResponse` | Returns the member's activity history, newest first. |
| [`ListSaved`](#memberservice-listsaved) | `/api/career.v1.MemberService/ListSaved` | Member | default | `ListSavedRequest` → `ListSavedResponse` | Lists saved items, newest first. |
| [`SaveItem`](#memberservice-saveitem) | `/api/career.v1.MemberService/SaveItem` | Member | default | `SaveItemRequest` → `SaveItemResponse` | Saves a content item with an optional private note. |
| [`UnsaveItem`](#memberservice-unsaveitem) | `/api/career.v1.MemberService/UnsaveItem` | Member | default | `UnsaveItemRequest` → `UnsaveItemResponse` | Removes a saved item. |
| [`RequestExport`](#memberservice-requestexport) | `/api/career.v1.MemberService/RequestExport` | Member | default | `RequestExportRequest` → `RequestExportResponse` | Requests an export of all data held about the member (profile, interests, activity, saved items, conversations) as JSON. |
| [`GetExport`](#memberservice-getexport) | `/api/career.v1.MemberService/GetExport` | Member | default | `GetExportRequest` → `GetExportResponse` | Returns the status of an export and, when ready, a short-lived download URL. |
| [`DeleteAccount`](#memberservice-deleteaccount) | `/api/career.v1.MemberService/DeleteAccount` | Member (unverified OK) | default | `DeleteAccountRequest` → `DeleteAccountResponse` | Schedules deletion of the account and all personal data. |

### MemberService.GetMe

`POST /api/career.v1.MemberService/GetMe` · **Auth:** Member (unverified OK) · **Rate limit:** default/min

Returns the calling member.

**Request** — [`GetMeRequest`](#getmerequest)

_No fields; send `{}`._

**Response** — [`GetMeResponse`](#getmeresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### MemberService.UpdateMe

`POST /api/career.v1.MemberService/UpdateMe` · **Auth:** Member · **Rate limit:** default/min

Updates profile fields named in `update_mask`. Fields outside the mask
are left unchanged; an empty mask is an error.

**Request** — [`UpdateMeRequest`](#updatemerequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string | `string: max_len: 120` | Display name. |
| `organization` | `string` | string | `string: max_len: 200` | Organization. |
| `statedRole` | `string` | string | `string: max_len: 120` | Stated role or title. |
| `profile` | [`Profile`](#profile) | object |  | Questionnaire answers and tailoring setting. |
| `updateMask` | `FieldMask` | string (comma-separated paths) | `required: true` | Which fields to update, using paths such as `name`, `organization`, `stated_role`, `profile.seniority`, `profile.hiring_for`, `profile.priorities`, `profile.heard_from`, `profile.tailoring_enabled`. |

**Response** — [`UpdateMeResponse`](#updatemeresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member after the update. |

<details><summary>Example request body</summary>

```json
{
  "name": "string",
  "organization": "string",
  "statedRole": "string",
  "profile": {
    "seniority": "string",
    "hiringFor": "string",
    "priorities": [
      "PRIORITY_LEADERSHIP"
    ],
    "heardFrom": "string",
    "tailoringEnabled": true,
    "questionnaireCompletedAt": "2026-09-18T12:00:00Z"
  },
  "updateMask": "name,organization"
}
```

</details>

### MemberService.SetInterests

`POST /api/career.v1.MemberService/SetInterests` · **Auth:** Member · **Rate limit:** default/min

Replaces the member's track interests and tailoring setting. Also used
to complete the first-visit questionnaire (an empty list with
`questionnaire_completed` true records a skip).

**Request** — [`SetInterestsRequest`](#setinterestsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `interests` | [`TrackInterest`](#trackinterest)[] | array of object | `repeated: max_items: 8` | Interests, at most eight; the questionnaire offers up to three. |
| `tailoringEnabled` | `bool` | boolean |  | Whether tailoring is enabled. |
| `questionnaireCompleted` | `bool` | boolean |  | Marks the first-visit questionnaire as completed (or skipped). |

**Response** — [`SetInterestsResponse`](#setinterestsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member after the update. |

<details><summary>Example request body</summary>

```json
{
  "interests": [
    {
      "trackId": "string",
      "weight": 0.5,
      "source": "INTEREST_SOURCE_QUESTIONNAIRE"
    }
  ],
  "tailoringEnabled": true,
  "questionnaireCompleted": true
}
```

</details>

### MemberService.GetHistory

`POST /api/career.v1.MemberService/GetHistory` · **Auth:** Member · **Rate limit:** default/min

Returns the member's activity history, newest first.

**Request** — [`GetHistoryRequest`](#gethistoryrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`ActivityEvent.Kind`](#activityeventkind) | string (enum name) | `enum: defined_only: true` | Restrict to one kind; unspecified returns all kinds. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`GetHistoryResponse`](#gethistoryresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `events` | [`ActivityEvent`](#activityevent)[] | array of object |  | Events, newest first, with `content` populated where applicable. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

<details><summary>Example request body</summary>

```json
{
  "kind": "KIND_VIEW",
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

### MemberService.ListSaved

`POST /api/career.v1.MemberService/ListSaved` · **Auth:** Member · **Rate limit:** default/min

Lists saved items, newest first.

**Request** — [`ListSavedRequest`](#listsavedrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`ListSavedResponse`](#listsavedresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`SavedItem`](#saveditem)[] | array of object |  | Items, newest first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

<details><summary>Example request body</summary>

```json
{
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

### MemberService.SaveItem

`POST /api/career.v1.MemberService/SaveItem` · **Auth:** Member · **Rate limit:** default/min

Saves a content item with an optional private note. Saving an already
saved item updates the note.

**Request** — [`SaveItemRequest`](#saveitemrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `contentId` | `string` | string | `string: min_len: 1 max_len: 128` | Content ID to save. |
| `note` | `string` | string | `string: max_len: 2000` | Optional private note (≤ 2,000 characters). |

**Response** — [`SaveItemResponse`](#saveitemresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `item` | [`SavedItem`](#saveditem) | object |  | The saved item as stored. |

<details><summary>Example request body</summary>

```json
{
  "contentId": "string",
  "note": "string"
}
```

</details>

### MemberService.UnsaveItem

`POST /api/career.v1.MemberService/UnsaveItem` · **Auth:** Member · **Rate limit:** default/min

Removes a saved item. Succeeds if the item was not saved.

**Request** — [`UnsaveItemRequest`](#unsaveitemrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `contentId` | `string` | string | `string: min_len: 1 max_len: 128` | Content ID to remove from the saved list. |

**Response** — [`UnsaveItemResponse`](#unsaveitemresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "contentId": "string"
}
```

</details>

### MemberService.RequestExport

`POST /api/career.v1.MemberService/RequestExport` · **Auth:** Member · **Rate limit:** default/min

Requests an export of all data held about the member (profile,
interests, activity, saved items, conversations) as JSON. The export is
prepared asynchronously; poll GetExport or wait for the email.

**Request** — [`RequestExportRequest`](#requestexportrequest)

_No fields; send `{}`._

**Response** — [`RequestExportResponse`](#requestexportresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `exportId` | `string` | string |  | Export identifier for GetExport. |
| `estimatedReadyAt` | `Timestamp` | string (RFC 3339, UTC) |  | Expected readiness; exports usually complete within a minute. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### MemberService.GetExport

`POST /api/career.v1.MemberService/GetExport` · **Auth:** Member · **Rate limit:** default/min

Returns the status of an export and, when ready, a short-lived download
URL.

**Request** — [`GetExportRequest`](#getexportrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `exportId` | `string` | string | `string: min_len: 1 max_len: 64` | Export identifier from RequestExport. |

**Response** — [`GetExportResponse`](#getexportresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`GetExportResponse.Status`](#getexportresponsestatus) | string (enum name) |  | Current state. |
| `downloadUrl` | `string` | string |  | Signed download URL, valid for `download_url_expires_at`; empty until READY. |
| `downloadUrlExpiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | Expiry of the download URL. |
| `sizeBytes` | `int64` | string (decimal) |  | Size of the export in bytes when READY. |

<details><summary>Example request body</summary>

```json
{
  "exportId": "string"
}
```

</details>

### MemberService.DeleteAccount

`POST /api/career.v1.MemberService/DeleteAccount` · **Auth:** Member (unverified OK) · **Rate limit:** default/min

Schedules deletion of the account and all personal data. The session
ends immediately; data is purged within 30 days (conversations and
activity are deleted or irreversibly anonymized).

**Request** — [`DeleteAccountRequest`](#deleteaccountrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `confirmation` | `string` | string | `string: const: "DELETE"` | Must be exactly "DELETE". |
| `currentPassword` | `string` | string | `string: max_len: 256` | Current password; required unless the account is social-only. |

**Response** — [`DeleteAccountResponse`](#deleteaccountresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `purgeBy` | `Timestamp` | string (RFC 3339, UTC) |  | Latest time by which personal data is purged. |

<details><summary>Example request body</summary>

```json
{
  "confirmation": "string",
  "currentPassword": "string"
}
```

</details>

## ContentService

Read access to the content catalog.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`ListContent`](#contentservice-listcontent) | `/api/career.v1.ContentService/ListContent` | Member | default | `ListContentRequest` → `ListContentResponse` | Lists content items with optional filters, ordered by relevance to the member (tailoring on) or by date (tailoring off). |
| [`GetContent`](#contentservice-getcontent) | `/api/career.v1.ContentService/GetContent` | Member | default | `GetContentRequest` → `GetContentResponse` | Returns one content item with its body and type-specific details. |
| [`WhatsNew`](#contentservice-whatsnew) | `/api/career.v1.ContentService/WhatsNew` | Member | default | `WhatsNewRequest` → `WhatsNewResponse` | Lists items published or materially updated since a point in time (default: the member's previous visit, or 90 days for a first visit). |
| [`ListTracks`](#contentservice-listtracks) | `/api/career.v1.ContentService/ListTracks` | Member | default | `ListTracksRequest` → `ListTracksResponse` | Lists the track taxonomy with framing copy and reading paths. |
| [`GetSkills`](#contentservice-getskills) | `/api/career.v1.ContentService/GetSkills` | Member | default | `GetSkillsRequest` → `GetSkillsResponse` | Returns the skills matrix grouped by category, each skill with its evidence links. |
| [`Search`](#contentservice-search) | `/api/career.v1.ContentService/Search` | Member | default | `SearchRequest` → `SearchResponse` | Full-text search across titles, summaries, bodies, and tags, with a relevance boost for the member's tracks. |

### ContentService.ListContent

`POST /api/career.v1.ContentService/ListContent` · **Auth:** Member · **Rate limit:** default/min

Lists content items with optional filters, ordered by relevance to the
member (tailoring on) or by date (tailoring off).

**Request** — [`ListContentRequest`](#listcontentrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `type` | [`ContentType`](#contenttype) | string (enum name) | `enum: defined_only: true` | Restrict to one content type; unspecified returns all types. |
| `trackIds` | `string`[] | array of string | `repeated: max_items: 8 items: string: max_len: 64 pattern: "^[a-z0-9-]+$"` | Restrict to items with a non-zero weight on any of these tracks. |
| `tags` | `string`[] | array of string | `repeated: max_items: 8 items: string: max_len: 64` | Restrict to items carrying all of these tags. |
| `order` | [`ListContentRequest.Order`](#listcontentrequestorder) | string (enum name) | `enum: defined_only: true` | Ordering; DEFAULT follows the member's tailoring setting. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`ListContentResponse`](#listcontentresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`ContentSummary`](#contentsummary)[] | array of object |  | Items in the requested order. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |
| `tailored` | `bool` | boolean |  | True when the order was tailored to the member's tracks. |

<details><summary>Example request body</summary>

```json
{
  "type": "CONTENT_TYPE_ROLE",
  "trackIds": [
    "string"
  ],
  "tags": [
    "string"
  ],
  "order": "ORDER_RELEVANCE",
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

### ContentService.GetContent

`POST /api/career.v1.ContentService/GetContent` · **Auth:** Member · **Rate limit:** default/min

Returns one content item with its body and type-specific details.

**Request** — [`GetContentRequest`](#getcontentrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `slug` | `string` | string | `string: min_len: 1 max_len: 128` | _(oneof `key`)_ URL slug, e.g. `mdemg`. |
| `id` | `string` | string | `string: min_len: 1 max_len: 128` | _(oneof `key`)_ Content ID, e.g. `prj-mdemg`. |

**Response** — [`GetContentResponse`](#getcontentresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `item` | [`ContentItem`](#contentitem) | object |  | The item. |

<details><summary>Example request body</summary>

```json
{
  "slug": "string"
}
```

</details>

### ContentService.WhatsNew

`POST /api/career.v1.ContentService/WhatsNew` · **Auth:** Member · **Rate limit:** default/min

Lists items published or materially updated since a point in time
(default: the member's previous visit, or 90 days for a first visit).

**Request** — [`WhatsNewRequest`](#whatsnewrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `since` | `Timestamp` | string (RFC 3339, UTC) |  | Return items changed after this time; unset uses the member's previous visit, or 90 days ago on a first visit. |
| `limit` | `int32` | number | `int32: lte: 50 gte: 0` | Maximum items, 1–50; zero selects 20. |

**Response** — [`WhatsNewResponse`](#whatsnewresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`ContentSummary`](#contentsummary)[] | array of object |  | Items, newest change first. |
| `since` | `Timestamp` | string (RFC 3339, UTC) |  | The effective `since` used. |

<details><summary>Example request body</summary>

```json
{
  "since": "2026-09-18T12:00:00Z",
  "limit": 0
}
```

</details>

### ContentService.ListTracks

`POST /api/career.v1.ContentService/ListTracks` · **Auth:** Member · **Rate limit:** default/min

Lists the track taxonomy with framing copy and reading paths.

**Request** — [`ListTracksRequest`](#listtracksrequest)

_No fields; send `{}`._

**Response** — [`ListTracksResponse`](#listtracksresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `tracks` | [`Track`](#track)[] | array of object |  | Tracks in display order. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### ContentService.GetSkills

`POST /api/career.v1.ContentService/GetSkills` · **Auth:** Member · **Rate limit:** default/min

Returns the skills matrix grouped by category, each skill with its
evidence links.

**Request** — [`GetSkillsRequest`](#getskillsrequest)

_No fields; send `{}`._

**Response** — [`GetSkillsResponse`](#getskillsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `categories` | [`SkillCategory`](#skillcategory)[] | array of object |  | Categories in display order, tailored to the member's tracks when tailoring is on. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### ContentService.Search

`POST /api/career.v1.ContentService/Search` · **Auth:** Member · **Rate limit:** default/min

Full-text search across titles, summaries, bodies, and tags, with a
relevance boost for the member's tracks.

**Request** — [`SearchRequest`](#searchrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: min_len: 1 max_len: 200` | Query text. |
| `type` | [`ContentType`](#contenttype) | string (enum name) | `enum: defined_only: true` | Restrict to one content type; unspecified searches all types. |
| `trackIds` | `string`[] | array of string | `repeated: max_items: 8 items: string: max_len: 64 pattern: "^[a-z0-9-]+$"` | Restrict to items weighted on any of these tracks. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`SearchResponse`](#searchresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `hits` | [`SearchHit`](#searchhit)[] | array of object |  | Hits in descending score. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

<details><summary>Example request body</summary>

```json
{
  "query": "string",
  "type": "CONTENT_TYPE_ROLE",
  "trackIds": [
    "string"
  ],
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

## HomeService

One-call assembly of everything the home page renders.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`GetHome`](#homeservice-gethome) | `/api/career.v1.HomeService/GetHome` | Member | default | `GetHomeRequest` → `GetHomeResponse` | Returns the welcome-back panel, "Start here" path, highlight sections, recommended résumé, hero image, and suggested assistant questions for the calling member. |

### HomeService.GetHome

`POST /api/career.v1.HomeService/GetHome` · **Auth:** Member · **Rate limit:** default/min

Returns the welcome-back panel, "Start here" path, highlight sections,
recommended résumé, hero image, and suggested assistant questions for
the calling member.

**Request** — [`GetHomeRequest`](#gethomerequest)

_No fields; send `{}`._

**Response** — [`GetHomeResponse`](#gethomeresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member. |
| `welcomeBack` | [`WelcomeBack`](#welcomeback) | object |  | Welcome-back panel; unset on the first visit. |
| `startHere` | [`PathItem`](#pathitem)[] | array of object |  | "Start here" path for the member's primary track (5–8 items); empty when the member has no interests. |
| `startHereTrackId` | `string` | string |  | Track the path belongs to. |
| `highlights` | [`HighlightSection`](#highlightsection)[] | array of object |  | Highlight sections in display order. |
| `resume` | [`RecommendedResume`](#recommendedresume) | object |  | Recommended résumé variant. |
| `hero` | [`Hero`](#hero) | object |  | Hero image. |
| `suggestedQuestions` | `string`[] | array of string |  | Suggested assistant questions for the member's tracks. |
| `questionnairePending` | `bool` | boolean |  | True when the first-visit questionnaire is still pending. |
| `tailored` | `bool` | boolean |  | True when the page was tailored (tailoring on and interests present). |

<details><summary>Example request body</summary>

```json
{}
```

</details>

## DownloadService

Lists what can be downloaded.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`ListDownloads`](#downloadservice-listdownloads) | `/api/career.v1.DownloadService/ListDownloads` | Member | default | `ListDownloadsRequest` → `ListDownloadsResponse` | Lists downloadable files with the variant matching the member's tracks first. |

### DownloadService.ListDownloads

`POST /api/career.v1.DownloadService/ListDownloads` · **Auth:** Member · **Rate limit:** default/min

Lists downloadable files with the variant matching the member's tracks
first.

**Request** — [`ListDownloadsRequest`](#listdownloadsrequest)

_No fields; send `{}`._

**Response** — [`ListDownloadsResponse`](#listdownloadsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`DownloadItem`](#downloaditem)[] | array of object |  | Items, recommended first. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

## ContactService

Reaching the owner outside the assistant.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`GetContactOptions`](#contactservice-getcontactoptions) | `/api/career.v1.ContactService/GetContactOptions` | Member | default | `GetContactOptionsRequest` → `GetContactOptionsResponse` | Returns the owner's availability statement and the contact channels he has chosen to publish. |
| [`SubmitContact`](#contactservice-submitcontact) | `/api/career.v1.ContactService/SubmitContact` | Public | default | `SubmitContactRequest` → `SubmitContactResponse` | Sends a message to the owner. |

### ContactService.GetContactOptions

`POST /api/career.v1.ContactService/GetContactOptions` · **Auth:** Member · **Rate limit:** default/min

Returns the owner's availability statement and the contact channels he
has chosen to publish.

**Request** — [`GetContactOptionsRequest`](#getcontactoptionsrequest)

_No fields; send `{}`._

**Response** — [`GetContactOptionsResponse`](#getcontactoptionsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `availability` | `string` | string |  | Availability statement in the owner's words. |
| `locationPreferences` | `string` | string |  | Location and remote preferences. |
| `channels` | [`ContactChannel`](#contactchannel)[] | array of object |  | Channels in display order. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### ContactService.SubmitContact

`POST /api/career.v1.ContactService/SubmitContact` · **Auth:** Public · **Rate limit:** default/min

Sends a message to the owner. Accepts both signed-in members (identity
read from the session cookie) and anonymous visitors (name + email
supplied on the form, Turnstile required). The owner receives the
message with the sender's context and replies from their mailbox.

**Request** — [`SubmitContactRequest`](#submitcontactrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `subject` | `string` | string | `string: min_len: 1 max_len: 200` | Subject line. |
| `message` | `string` | string | `string: min_len: 1 max_len: 5000` | Message body. |
| `replyChannel` | [`SubmitContactRequest.ReplyChannel`](#submitcontactrequestreplychannel) | string (enum name) | `enum: defined_only: true not_in: 0` | Preferred reply channel. |
| `conversationId` | `string` | string | `string: max_len: 64` | Attach a conversation transcript for context; optional. |
| `category` | [`SupportCategory`](#supportcategory) | string (enum name) | `enum: defined_only: true not_in: 0` | Categorises the message so the owner can filter the support inbox (FR-CNT-22 / FR-ADM-14). Required. |
| `name` | `string` | string | `string: max_len: 200` | Anonymous sender's display name. Required when the request has no session cookie; ignored when it does (the member's name wins). |
| `email` | `string` | string | `string: max_len: 320` | Anonymous sender's email. Required + validated as an email address when the request has no session cookie; ignored when it does. |
| `turnstileToken` | `string` | string | `string: max_len: 4096` | Cloudflare Turnstile response token. Required on anonymous submissions; ignored for members. |
| `hiringRole` | `string` | string | `string: max_len: 200` | The role the sender is considering Roger for. Populated by the form when category = HIRING_INQUIRY; ignored otherwise. Free text; used for admin triage. |
| `hiringJdUrl` | `string` | string | `string: max_len: 2000` | Absolute URL of the job posting. Populated by the form when category = HIRING_INQUIRY; ignored otherwise. Not validated beyond length so recruiters can drop URLs from ATS systems that include tokens / query strings. |
| `hiringTargetStart` | `string` | string | `string: max_len: 100` | Free-text "target start" the sender's hiring cycle is aiming at, e.g. "ASAP", "Q1 2027", "flexible". Populated by the form when category = HIRING_INQUIRY; ignored otherwise. |

**Response** — [`SubmitContactResponse`](#submitcontactresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `ticketId` | `string` | string |  | Ticket identifier quoted in the owner's reply. |

<details><summary>Example request body</summary>

```json
{
  "subject": "string",
  "message": "string",
  "replyChannel": "REPLY_CHANNEL_EMAIL",
  "conversationId": "string",
  "category": "SUPPORT_CATEGORY_GENERAL_QUESTION",
  "name": "string",
  "email": "string",
  "turnstileToken": "string",
  "hiringRole": "string",
  "hiringJdUrl": "string",
  "hiringTargetStart": "string"
}
```

</details>

## JdService

JD-upload flow, members only: a signed-in session is required to
submit or poll, and the submitting member is recorded on the row.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`SubmitJd`](#jdservice-submitjd) | `/api/career.v1.JdService/SubmitJd` | Member | default | `SubmitJdRequest` → `SubmitJdResponse` | Accepts a job description and stores it for scoring. |
| [`GetJdResult`](#jdservice-getjdresult) | `/api/career.v1.JdService/GetJdResult` | Member | default | `GetJdResultRequest` → `GetJdResultResponse` | Returns the current state of a submission (received / scoring / below-threshold / generating / ready / failed) with progress, the score and fit category once known, and, for the submitting member or a caller holding the result token, the requirement verdicts plus the résumé and its PDF link when status is `ready`. |
| [`ListMySubmissions`](#jdservice-listmysubmissions) | `/api/career.v1.JdService/ListMySubmissions` | Member | default | `ListMySubmissionsRequest` → `ListMySubmissionsResponse` | Lists the signed-in member's own submissions, newest first, so a review can be reopened after the tab that submitted it is gone. |
| [`GetJdReviewConfig`](#jdservice-getjdreviewconfig) | `/api/career.v1.JdService/GetJdReviewConfig` | Member | default | `GetJdReviewConfigRequest` → `GetJdReviewConfigResponse` | Returns the fit bands in force (the gate is the "strong" edge), so the JD pages quote the numbers the pipeline actually uses. |

### JdService.SubmitJd

`POST /api/career.v1.JdService/SubmitJd` · **Auth:** Member · **Rate limit:** default/min

Accepts a job description and stores it for scoring. Returns the
submission id the caller uses to poll GetJdResult. Never blocks
on the actual scoring / generation — those run out of band.

**Request** — [`SubmitJdRequest`](#submitjdrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jdText` | `string` | string | `string: max_len: 50000` | Full JD text pasted into the textarea. Capped at 50 000 chars by the RPC. Mutually exclusive with the byte-body fields below. |
| `source` | [`JdSource`](#jdsource) | string (enum name) | `enum: defined_only: true not_in: 0` | Where the text came from — the frontend sets this so the backend knows what to record. |
| `roleHint` | `string` | string | `string: max_len: 200` | Optional role / title the member is considering Roger for. Free-form; shown in admin triage and passed as a hint to the requirement and résumé prompts. |
| `employerHint` | `string` | string | `string: max_len: 200` | Optional employer name (e.g. "Anthropic"). Same free-form triage aid and prompt hint as role_hint. |
| `contactEmail` | `string` | string | `string: max_len: 254` | Optional extra address for the finished-review email; the member's account email always receives it. Never surfaced publicly. |
| `applyUrl` | `string` | string | `string: max_len: 2048` | Optional link to apply for the position (http or https). Shown to Roger in the admin triage view; never surfaced publicly. |

**Response** — [`SubmitJdResponse`](#submitjdresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string |  | Server-issued id (numeric, stringified over the wire). |
| `status` | [`JdStatus`](#jdstatus) | string (enum name) |  | Current status — usually RECEIVED right at submit time. |
| `message` | `string` | string |  | Fixed human-readable acknowledgement text the /jd-upload page renders back to the caller so the copy stays server-controlled. |
| `resultToken` | `string` | string |  | Secret issued once per submission (hex). Present it on GetJdResult to receive the verdicts and the generated résumé. The submitting member's own session releases the same things without it, so a review can be reopened from /jd-upload/<id> later. |
| `quota` | [`JdQuota`](#jdquota) | object |  | What is left of the member's daily allowance after this submission, so the page can say so rather than let them discover the limit by hitting it. |

<details><summary>Example request body</summary>

```json
{
  "jdText": "string",
  "source": "JD_SOURCE_PASTE",
  "roleHint": "string",
  "employerHint": "string",
  "contactEmail": "string",
  "applyUrl": "string"
}
```

</details>

### JdService.GetJdResult

`POST /api/career.v1.JdService/GetJdResult` · **Auth:** Member · **Rate limit:** default/min

Returns the current state of a submission (received / scoring /
below-threshold / generating / ready / failed) with progress, the
score and fit category once known, and, for the submitting member
or a caller holding the result token, the requirement verdicts plus
the résumé and its PDF link when status is `ready`.

**Request** — [`GetJdResultRequest`](#getjdresultrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | ID from SubmitJdResponse. |
| `resultToken` | `string` | string | `string: max_len: 64` | Token from SubmitJdResponse; optional. Gates the verdicts and the résumé body unless the caller is the submitting member. |

**Response** — [`GetJdResultResponse`](#getjdresultresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`JdStatus`](#jdstatus) | string (enum name) |  | Current status. |
| `matchScore` | `double` | number |  | _(oneof `_match_score`)_ Match score in [0, 1] once known; unset before scoring runs and after a failure. |
| `generatedResumeUrl` | `string` | string |  | Download path of the locked résumé PDF, with the result token appended, when status is READY and the caller may see the résumé; empty otherwise. |
| `errorMessage` | `string` | string |  | Human-readable error text when status is FAILED; empty otherwise. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the submission was first accepted. |
| `completedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the terminal state (ready / below_threshold / failed) was reached. Unset while the pipeline is still running. |
| `resumeMarkdown` | `string` | string |  | Generated résumé in markdown, only when status is READY and the caller is the submitting member or carried the result_token. |
| `verdicts` | [`RequirementVerdict`](#requirementverdict)[] | array of object |  | Requirement-by-requirement verdicts behind the score, released to the submitting member (or with the result_token) whatever the outcome, so a below-threshold result shows what was and was not evidenced instead of a bare number (the opposite of an ATS musts-and-misses filter). |
| `metCount` | `int32` | number |  | Number of requirements judged met. |
| `partialCount` | `int32` | number |  | Number judged partially met. |
| `unmetCount` | `int32` | number |  | Number judged not evidenced. |
| `matchThreshold` | `double` | number |  | The résumé gate in force: the "strong" fit band (owner-editable; JD_MATCH_THRESHOLD only seeds it the first time). |
| `progressPct` | `int32` | number |  | Pipeline progress, 0 to 100, while the submission is live; 100 once it has finished. |
| `progressStage` | `string` | string |  | Short human-readable stage ("judging requirement 4 of 12"). |
| `fitCategory` | `string` | string |  | Fit category derived from the score once known: very_strong, strong, possible, weak, very_weak; empty before scoring. |

<details><summary>Example request body</summary>

```json
{
  "submissionId": "string",
  "resultToken": "string"
}
```

</details>

### JdService.ListMySubmissions

`POST /api/career.v1.JdService/ListMySubmissions` · **Auth:** Member · **Rate limit:** default/min

Lists the signed-in member's own submissions, newest first, so a
review can be reopened after the tab that submitted it is gone.

**Request** — [`ListMySubmissionsRequest`](#listmysubmissionsrequest)

_No fields; send `{}`._

**Response** — [`ListMySubmissionsResponse`](#listmysubmissionsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissions` | [`MySubmission`](#mysubmission)[] | array of object |  | Up to 100 rows. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### JdService.GetJdReviewConfig

`POST /api/career.v1.JdService/GetJdReviewConfig` · **Auth:** Member · **Rate limit:** default/min

Returns the fit bands in force (the gate is the "strong" edge), so
the JD pages quote the numbers the pipeline actually uses.

**Request** — [`GetJdReviewConfigRequest`](#getjdreviewconfigrequest)

_No fields; send `{}`._

**Response** — [`GetJdReviewConfigResponse`](#getjdreviewconfigresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `bands` | [`JdFitBands`](#jdfitbands) | object |  | Current bands. |
| `quota` | [`JdQuota`](#jdquota) | object |  | The signed-in member's allowance as it stands now, so the upload page can state the limit before anyone spends it. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

## MeetingService

Meeting scheduling, members only: a signed-in session is required to
see availability or to book, and the booking member is recorded on
the row.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`GetMeetingOptions`](#meetingservice-getmeetingoptions) | `/api/career.v1.MeetingService/GetMeetingOptions` | Member | default | `GetMeetingOptionsRequest` → `GetMeetingOptionsResponse` | Returns what the member is allowed to choose before they choose it: the meeting lengths on offer, the zone every time is quoted in, and how far ahead the calendar runs. |
| [`GetAvailability`](#meetingservice-getavailability) | `/api/career.v1.MeetingService/GetAvailability` | Member | default | `GetAvailabilityRequest` → `GetAvailabilityResponse` | Returns the start times a member may book for one meeting length. |
| [`BookMeeting`](#meetingservice-bookmeeting) | `/api/career.v1.MeetingService/BookMeeting` | Member | default | `BookMeetingRequest` → `BookMeetingResponse` | Books one slot. |
| [`ListMyMeetings`](#meetingservice-listmymeetings) | `/api/career.v1.MeetingService/ListMyMeetings` | Member | default | `ListMyMeetingsRequest` → `ListMyMeetingsResponse` | Lists the calling member's own meetings, upcoming first, so a booking can be found again after the tab that made it is gone. |
| [`CancelMeeting`](#meetingservice-cancelmeeting) | `/api/career.v1.MeetingService/CancelMeeting` | Member | default | `CancelMeetingRequest` → `CancelMeetingResponse` | Cancels the caller's own meeting and frees the time. |

### MeetingService.GetMeetingOptions

`POST /api/career.v1.MeetingService/GetMeetingOptions` · **Auth:** Member · **Rate limit:** default/min

Returns what the member is allowed to choose before they choose it:
the meeting lengths on offer, the zone every time is quoted in, and
how far ahead the calendar runs. Fetched once when the page opens
so the form is built from the owner's live settings rather than
from constants compiled into the frontend.

**Request** — [`GetMeetingOptionsRequest`](#getmeetingoptionsrequest)

_No fields; send `{}`._

**Response** — [`GetMeetingOptionsResponse`](#getmeetingoptionsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `durationMinutes` | `int32`[] | array of number |  | Meeting lengths on offer, in minutes, shortest first. The owner offers 15, 30 and 45; the length is the member's decision because only they know whether they need a question answered or a real conversation. |
| `zone` | `string` | string |  | IANA zone name every time in this API is rendered in, for example "America/New_York". Never an offset: an offset is correct for half the year. Show it beside the times rather than converting. |
| `zoneLabel` | `string` | string |  | Human label for the zone to print next to a time, for example "Eastern time". Sent by the server so the wording is the owner's and not the frontend's guess from the IANA name. |
| `horizonDays` | `int32` | number |  | How many days ahead the calendar is offered. |
| `leadHours` | `int32` | number |  | How far ahead the earliest bookable slot sits, in hours, so the page can say why today is not on offer. |
| `hoursSummary` | `string` | string |  | Plain-language summary of the owner's stated hours, for example "Tuesday to Thursday, mornings and early afternoons". Rendered rather than derived, so a member reads intent instead of a grid. |
| `available` | `bool` | boolean |  | False when the calendar is not connected or is unreachable, in which case no slots can be offered and the page should say so rather than show an empty calendar that reads as "never free". |
| `unavailableReason` | `string` | string |  | Why booking is unavailable, for the member, when available is false. Empty otherwise. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### MeetingService.GetAvailability

`POST /api/career.v1.MeetingService/GetAvailability` · **Auth:** Member · **Rate limit:** default/min

Returns the start times a member may book for one meeting length.
Computed per length rather than once, because a 45-minute meeting
has fewer places to go than a 15-minute one and offering a start
that cannot fit is worse than offering nothing.

**Request** — [`GetAvailabilityRequest`](#getavailabilityrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `durationMinutes` | `int32` | number | `int32: gt: 0` | Meeting length in minutes. Must be one of the lengths returned by GetMeetingOptions; a length nobody offered is a bad request rather than something to round to the nearest. |
| `from` | `Timestamp` | string (RFC 3339, UTC) |  | Start of the range to search, inclusive. Defaults to now when unset. Clamped to the lead time regardless of what is sent. |
| `to` | `Timestamp` | string (RFC 3339, UTC) |  | End of the range, exclusive. Defaults to the horizon when unset, and is clamped to it, so a caller cannot ask about next year. |

**Response** — [`GetAvailabilityResponse`](#getavailabilityresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `slots` | [`MeetingSlot`](#meetingslot)[] | array of object |  | Offered start times, in order. Empty means nothing is free in the range, which is a normal answer and not an error. |
| `zone` | `string` | string |  | IANA zone the slots should be rendered in, repeated here so a response is self-describing without the options call. |
| `available` | `bool` | boolean |  | False when the calendar could not be read. Distinguishes "nothing is free" from "we do not know", which must not look the same. |
| `unavailableReason` | `string` | string |  | Why availability could not be computed, when available is false. |

<details><summary>Example request body</summary>

```json
{
  "durationMinutes": 0,
  "from": "2026-09-18T12:00:00Z",
  "to": "2026-09-18T12:00:00Z"
}
```

</details>

### MeetingService.BookMeeting

`POST /api/career.v1.MeetingService/BookMeeting` · **Auth:** Member · **Rate limit:** default/min

Books one slot. Re-reads the calendar for the claimed interval
first, because the list the member saw is stale by the time they
submit, and returns ALREADY_EXISTS if the time went while they were
deciding. Never partially succeeds: a claim whose calendar event
cannot be created is released.

**Request** — [`BookMeetingRequest`](#bookmeetingrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `start` | `Timestamp` | string (RFC 3339, UTC) | `required: true` | The chosen start, exactly as returned by GetAvailability. The server re-checks it: this is a claim, not an instruction. |
| `durationMinutes` | `int32` | number | `int32: gt: 0` | Meeting length in minutes; must be one of the offered lengths. |
| `topic` | `string` | string | `string: max_len: 500` | What the member wants to discuss, in their words. Shown to the owner on the calendar entry so he arrives knowing the subject. |
| `contactPreference` | `string` | string | `string: max_len: 200` | How the member would like to meet, in their words, for example a phone number or "your call, send a link". Optional. |
| `meetingType` | [`MeetingType`](#meetingtype) | string (enum name) |  | Video or phone. Required on new bookings. |
| `videoProvider` | [`VideoProvider`](#videoprovider) | string (enum name) |  | Which service, when meeting_type is video. Rejected otherwise. |
| `phoneNumber` | `string` | string | `string: max_len: 32` | The number the member will call FROM, when meeting_type is phone. Country code, a space, then the ten-digit number, for example "+1 5135551234". May be left empty to say the number is in the comments instead, which is the escape hatch for anyone whose number does not fit that shape. |

**Response** — [`BookMeetingResponse`](#bookmeetingresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meeting` | [`Meeting`](#meeting) | object |  | The meeting as booked. |

<details><summary>Example request body</summary>

```json
{
  "start": "2026-09-18T12:00:00Z",
  "durationMinutes": 0,
  "topic": "string",
  "contactPreference": "string",
  "meetingType": "MEETING_TYPE_VIDEO",
  "videoProvider": "VIDEO_PROVIDER_GOOGLE_MEET",
  "phoneNumber": "string"
}
```

</details>

### MeetingService.ListMyMeetings

`POST /api/career.v1.MeetingService/ListMyMeetings` · **Auth:** Member · **Rate limit:** default/min

Lists the calling member's own meetings, upcoming first, so a
booking can be found again after the tab that made it is gone.

**Request** — [`ListMyMeetingsRequest`](#listmymeetingsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includePast` | `bool` | boolean |  | Include meetings that have already happened or been cancelled. False returns only what is still ahead. |

**Response** — [`ListMyMeetingsResponse`](#listmymeetingsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meetings` | [`Meeting`](#meeting)[] | array of object |  | Meetings, soonest first. |

<details><summary>Example request body</summary>

```json
{
  "includePast": true
}
```

</details>

### MeetingService.CancelMeeting

`POST /api/career.v1.MeetingService/CancelMeeting` · **Auth:** Member · **Rate limit:** default/min

Cancels the caller's own meeting and frees the time. Cancelling
something already cancelled succeeds, so a double click or a stale
tab is not an error the member has to understand.

**Request** — [`CancelMeetingRequest`](#cancelmeetingrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) | `int64: gt: 0` | Which meeting to cancel. Must belong to the calling member. |

**Response** — [`CancelMeetingResponse`](#cancelmeetingresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meeting` | [`Meeting`](#meeting) | object |  | The meeting, with cancelled_at set. |

<details><summary>Example request body</summary>

```json
{
  "id": "0"
}
```

</details>

## ChatService

Conversations with the assistant.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`CreateConversation`](#chatservice-createconversation) | `/api/career.v1.ChatService/CreateConversation` | Member | default | `CreateConversationRequest` → `CreateConversationResponse` | Starts a conversation. |
| [`ListConversations`](#chatservice-listconversations) | `/api/career.v1.ChatService/ListConversations` | Member | default | `ListConversationsRequest` → `ListConversationsResponse` | Lists the member's conversations, most recent first. |
| [`GetConversation`](#chatservice-getconversation) | `/api/career.v1.ChatService/GetConversation` | Member | default | `GetConversationRequest` → `GetConversationResponse` | Returns a conversation with all of its messages. |
| [`SendMessage`](#chatservice-sendmessage) | `/api/career.v1.ChatService/SendMessage` | Member | default | `SendMessageRequest` → `SendMessageResponse` (server-streaming) | Sends a member message and streams the assistant's reply: a `start` event, then `delta` events with text as it is generated, then `citations`, `usage`, and a final `done` carrying the complete message. |
| [`DeleteConversation`](#chatservice-deleteconversation) | `/api/career.v1.ChatService/DeleteConversation` | Member | default | `DeleteConversationRequest` → `DeleteConversationResponse` | Deletes a conversation. |
| [`RateMessage`](#chatservice-ratemessage) | `/api/career.v1.ChatService/RateMessage` | Member | default | `RateMessageRequest` → `RateMessageResponse` | Rates an assistant message up or down with an optional comment. |
| [`Escalate`](#chatservice-escalate) | `/api/career.v1.ChatService/Escalate` | Member | default | `EscalateRequest` → `EscalateResponse` | Sends a question, with the conversation context, to the owner. |
| [`GetSuggestions`](#chatservice-getsuggestions) | `/api/career.v1.ChatService/GetSuggestions` | Member | default | `GetSuggestionsRequest` → `GetSuggestionsResponse` | Returns suggested questions for the current page and the member's tracks (at least three). |
| [`GetQuota`](#chatservice-getquota) | `/api/career.v1.ChatService/GetQuota` | Member | default | `GetQuotaRequest` → `GetQuotaResponse` | Returns the member's remaining allowance and the global budget mode so the panel can show limits before they are hit. |
| [`ListAdminQueries`](#chatservice-listadminqueries) | `/api/career.v1.ChatService/ListAdminQueries` | Admin | default | `ListAdminQueriesRequest` → `ListAdminQueriesResponse` | Lists the database queries an admin may run from the assistant, for the dropdown. |
| [`RunAdminQuery`](#chatservice-runadminquery) | `/api/career.v1.ChatService/RunAdminQuery` | Admin | default | `RunAdminQueryRequest` → `RunAdminQueryResponse` | Runs one named query and returns its result. |

### ChatService.CreateConversation

`POST /api/career.v1.ChatService/CreateConversation` · **Auth:** Member · **Rate limit:** default/min

Starts a conversation. The first assistant message is the AI disclosure
(FSD §15.C) and is included in the response so the panel can render it
without a round trip.

**Request** — [`CreateConversationRequest`](#createconversationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `contextContentId` | `string` | string | `string: max_len: 128` | Content item the panel was opened from, used for suggestions and retrieval bias; optional. |

**Response** — [`CreateConversationResponse`](#createconversationresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversation` | [`Conversation`](#conversation) | object |  | The conversation. |
| `disclosure` | [`Message`](#message) | object |  | The disclosure message (role ASSISTANT). |

<details><summary>Example request body</summary>

```json
{
  "contextContentId": "string"
}
```

</details>

### ChatService.ListConversations

`POST /api/career.v1.ChatService/ListConversations` · **Auth:** Member · **Rate limit:** default/min

Lists the member's conversations, most recent first.

**Request** — [`ListConversationsRequest`](#listconversationsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`ListConversationsResponse`](#listconversationsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversations` | [`Conversation`](#conversation)[] | array of object |  | Conversations, most recent first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

<details><summary>Example request body</summary>

```json
{
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

### ChatService.GetConversation

`POST /api/career.v1.ChatService/GetConversation` · **Auth:** Member · **Rate limit:** default/min

Returns a conversation with all of its messages.

**Request** — [`GetConversationRequest`](#getconversationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation ID. |

**Response** — [`GetConversationResponse`](#getconversationresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversation` | [`Conversation`](#conversation) | object |  | The conversation. |
| `messages` | [`Message`](#message)[] | array of object |  | Messages in chronological order. |

<details><summary>Example request body</summary>

```json
{
  "conversationId": "string"
}
```

</details>

### ChatService.SendMessage

`POST /api/career.v1.ChatService/SendMessage` · **Auth:** Member · **Rate limit:** default/min · **Server-streaming**

Sends a member message and streams the assistant's reply: a `start`
event, then `delta` events with text as it is generated, then
`citations`, `usage`, and a final `done` carrying the complete message.
Quota and budget checks happen before the first event; when exceeded the
call fails with `resource_exhausted` and no message is stored.

**Request** — [`SendMessageRequest`](#sendmessagerequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation to append to. |
| `text` | `string` | string | `string: min_len: 1 max_len: 4000` | The question or message text. |
| `contextContentId` | `string` | string | `string: max_len: 128` | Content item currently on screen, used to bias retrieval; optional. |

**Response (one event per stream message)** — [`SendMessageResponse`](#sendmessageresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `start` | [`SendMessageResponse.Start`](#sendmessageresponsestart) | object |  | _(oneof `event`)_ First event: identifiers for the stored member message and the assistant message being generated. |
| `delta` | [`SendMessageResponse.Delta`](#sendmessageresponsedelta) | object |  | _(oneof `event`)_ A fragment of assistant text, in order; concatenate to build the reply. |
| `citations` | [`SendMessageResponse.Citations`](#sendmessageresponsecitations) | object |  | _(oneof `event`)_ Sources for the reply; sent once, after the text. |
| `usage` | [`Usage`](#usage) | object |  | _(oneof `event`)_ Usage and remaining allowance; sent once. |
| `done` | [`SendMessageResponse.Done`](#sendmessageresponsedone) | object |  | _(oneof `event`)_ Final event: the complete assistant message as stored. |

<details><summary>Example request body</summary>

```json
{
  "conversationId": "string",
  "text": "string",
  "contextContentId": "string"
}
```

</details>

### ChatService.DeleteConversation

`POST /api/career.v1.ChatService/DeleteConversation` · **Auth:** Member · **Rate limit:** default/min

Deletes a conversation. It disappears immediately for the member and is
hard-deleted within 30 days.

**Request** — [`DeleteConversationRequest`](#deleteconversationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation ID. |

**Response** — [`DeleteConversationResponse`](#deleteconversationresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "conversationId": "string"
}
```

</details>

### ChatService.RateMessage

`POST /api/career.v1.ChatService/RateMessage` · **Auth:** Member · **Rate limit:** default/min

Rates an assistant message up or down with an optional comment.

**Request** — [`RateMessageRequest`](#ratemessagerequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `messageId` | `string` | string | `string: min_len: 1 max_len: 64` | Assistant message ID. |
| `rating` | [`Rating`](#rating) | string (enum name) | `enum: defined_only: true` | The rating; UNSPECIFIED clears a previous rating. |
| `comment` | `string` | string | `string: max_len: 1000` | Optional comment (≤ 1,000 characters). |

**Response** — [`RateMessageResponse`](#ratemessageresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "messageId": "string",
  "rating": "RATING_UP",
  "comment": "string"
}
```

</details>

### ChatService.Escalate

`POST /api/career.v1.ChatService/Escalate` · **Auth:** Member · **Rate limit:** default/min

Sends a question, with the conversation context, to the owner. The owner
replies inside the conversation (as a message with role OWNER) and the
member is notified by email.

**Request** — [`EscalateRequest`](#escalaterequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation to escalate. |
| `question` | `string` | string | `string: max_len: 4000` | The question for the owner; defaults to the last member message when empty. |
| `messageId` | `string` | string | `string: max_len: 64` | Assistant message that prompted the escalation, when any. |

**Response** — [`EscalateResponse`](#escalateresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `escalation` | [`Escalation`](#escalation) | object |  | The escalation. |

<details><summary>Example request body</summary>

```json
{
  "conversationId": "string",
  "question": "string",
  "messageId": "string"
}
```

</details>

### ChatService.GetSuggestions

`POST /api/career.v1.ChatService/GetSuggestions` · **Auth:** Member · **Rate limit:** default/min

Returns suggested questions for the current page and the member's
tracks (at least three).

**Request** — [`GetSuggestionsRequest`](#getsuggestionsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `path` | `string` | string | `string: max_len: 256` | Site path of the current page, e.g. `/projects/mdemg`; optional. |
| `contentId` | `string` | string | `string: max_len: 128` | Content item on screen, when any; optional. |

**Response** — [`GetSuggestionsResponse`](#getsuggestionsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `questions` | `string`[] | array of string |  | Questions, most specific first (at least three). |

<details><summary>Example request body</summary>

```json
{
  "path": "string",
  "contentId": "string"
}
```

</details>

### ChatService.GetQuota

`POST /api/career.v1.ChatService/GetQuota` · **Auth:** Member · **Rate limit:** default/min

Returns the member's remaining allowance and the global budget mode so
the panel can show limits before they are hit.

**Request** — [`GetQuotaRequest`](#getquotarequest)

_No fields; send `{}`._

**Response** — [`GetQuotaResponse`](#getquotaresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `dailyLimit` | `int32` | number |  | Daily message limit. |
| `usedToday` | `int32` | number |  | Messages used today. |
| `conversationLimit` | `int32` | number |  | Per-conversation message limit. |
| `budgetMode` | [`GetQuotaResponse.BudgetMode`](#getquotaresponsebudgetmode) | string (enum name) |  | Global budget mode. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### ChatService.ListAdminQueries

`POST /api/career.v1.ChatService/ListAdminQueries` · **Auth:** Admin · **Rate limit:** default/min

Lists the database queries an admin may run from the assistant, for
the dropdown. Names and descriptions only; the SQL never leaves the
server.

**Request** — [`ListAdminQueriesRequest`](#listadminqueriesrequest)

_No fields; send `{}`._

**Response** — [`ListAdminQueriesResponse`](#listadminqueriesresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `queries` | [`AdminQuery`](#adminquery)[] | array of object |  | Offered queries, in display order. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### ChatService.RunAdminQuery

`POST /api/career.v1.ChatService/RunAdminQuery` · **Auth:** Admin · **Rate limit:** default/min

Runs one named query and returns its result.

The id selects a fixed statement and is never interpolated into
one, so there is nothing to inject into and an unknown id is simply
refused. Results are counts, never rows about a person.

**Request** — [`RunAdminQueryRequest`](#runadminqueryrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Id from ListAdminQueries. Anything else is refused. |

**Response** — [`RunAdminQueryResponse`](#runadminqueryresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | [`AdminQuery`](#adminquery) | object |  | The query that ran, echoed so the surface can label the result. |
| `result` | `string` | string |  | A one-line summary of the table below. Deprecated. This was the whole result before it became rows and columns, and it is kept because the breaking-change rules here are FILE, the strictest set, which forbids removing a field even when its number is reserved. That is the right default: a client built against the old contract keeps working rather than silently reading nothing. Still populated rather than emptied, for the same reason. One row becomes its cells joined; several become a count. |
| `ranAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it ran. |
| `columns` | `string`[] | array of string |  | Column headers, taken from the statement itself so a query and its header cannot drift apart. |
| `rows` | [`AdminQueryRow`](#adminqueryrow)[] | array of object |  | Rows, capped server side. Every query here aggregates, so a long result means a GROUP BY went wider than expected. |

<details><summary>Example request body</summary>

```json
{
  "id": "string"
}
```

</details>

## ActivityService

Batched, fire-and-forget activity reporting.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`RecordEvents`](#activityservice-recordevents) | `/api/career.v1.ActivityService/RecordEvents` | Member | default | `RecordEventsRequest` → `RecordEventsResponse` | Records a batch of events. |

### ActivityService.RecordEvents

`POST /api/career.v1.ActivityService/RecordEvents` · **Auth:** Member · **Rate limit:** default/min

Records a batch of events. Duplicate `client_event_id`s are ignored, so
clients may retry a failed batch safely. Failures never affect page
rendering; clients should send with `keepalive` on page hide.

**Request** — [`RecordEventsRequest`](#recordeventsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `events` | [`ActivityEvent`](#activityevent)[] | array of object | `repeated: min_items: 1 max_items: 50` | Events, 1–50 per batch. |

**Response** — [`RecordEventsResponse`](#recordeventsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `accepted` | `int32` | number |  | Events accepted (new). |
| `duplicates` | `int32` | number |  | Events ignored as duplicates. |

<details><summary>Example request body</summary>

```json
{
  "events": [
    {
      "kind": "KIND_VIEW",
      "contentId": "string",
      "occurredAt": "2026-09-18T12:00:00Z",
      "dwellMs": 0,
      "query": "string",
      "conversationId": "string",
      "variant": "string",
      "clientEventId": "string",
      "content": {
        "id": "string",
        "slug": "string",
        "type": "CONTENT_TYPE_ROLE",
        "title": "string",
        "summary": "string",
        "published": "2026-09-18T12:00:00Z",
        "updated": "2026-09-18T12:00:00Z",
        "tags": [
          "string"
        ],
        "tracks": [
          {}
        ],
        "relevance": 0.5,
        "reason": "string",
        "isNew": true,
        "viewed": true,
        "saved": true,
        "path": "string"
      }
    }
  ]
}
```

</details>

## EventService

Accepts browser-minted events. Public so the landing funnel is
visible before sign-in; the api attaches identity (session member,
anonymous id cookie), the client address hash and the device class
itself and never trusts those from the client.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`Record`](#eventservice-record) | `/api/career.v1.EventService/Record` | Public | default | `RecordRequest` → `RecordResponse` | Records up to 50 events. |

### EventService.Record

`POST /api/career.v1.EventService/Record` · **Auth:** Public · **Rate limit:** default/min

Records up to 50 events. Unknown names and oversized props are
dropped, never errors; the response says how many were stored.

**Request** — [`RecordRequest`](#recordrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `events` | [`BrowserEvent`](#browserevent)[] | array of object | `repeated: min_items: 1 max_items: 50` | Up to 50 events. |

**Response** — [`RecordResponse`](#recordresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `accepted` | `int32` | number |  | Events stored (duplicates and rejected events are not counted). |

<details><summary>Example request body</summary>

```json
{
  "events": [
    {
      "eventId": "string",
      "name": "string",
      "clientTsMs": "0",
      "path": "string",
      "referrer": "string",
      "uiMode": "string",
      "propsJson": "string"
    }
  ]
}
```

</details>

## AdminService

Owner console.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`ListMembers`](#adminservice-listmembers) | `/api/career.v1.AdminService/ListMembers` | Admin | default | `ListMembersRequest` → `ListMembersResponse` | Lists members with search, filters, and pagination. |
| [`ResendNotification`](#adminservice-resendnotification) | `/api/career.v1.AdminService/ResendNotification` | Admin | default | `ResendNotificationRequest` → `ResendNotificationResponse` | Re-sends the approval or decline email to a member and returns the audited attempt so the console can show the provider's verdict inline. |
| [`GetMember`](#adminservice-getmember) | `/api/career.v1.AdminService/GetMember` | Admin | default | `GetMemberRequest` → `GetMemberResponse` | Returns one member with recent activity, conversations, admin notes, and email delivery history. |
| [`AddMemberNote`](#adminservice-addmembernote) | `/api/career.v1.AdminService/AddMemberNote` | Admin | default | `AddMemberNoteRequest` → `AddMemberNoteResponse` | Adds a private admin note to a member. |
| [`SetMemberStatus`](#adminservice-setmemberstatus) | `/api/career.v1.AdminService/SetMemberStatus` | Admin | default | `SetMemberStatusRequest` → `SetMemberStatusResponse` | Approves or rejects a registration waiting in PENDING_APPROVAL (approval mode only), or disables/re-enables an account. |
| [`GetReviewQueue`](#adminservice-getreviewqueue) | `/api/career.v1.AdminService/GetReviewQueue` | Admin | default | `GetReviewQueueRequest` → `GetReviewQueueResponse` | Lists items needing the owner's attention: negative feedback, "I don't know" answers, escalations, and out-of-scope refusals. |
| [`ResolveReviewItem`](#adminservice-resolvereviewitem) | `/api/career.v1.AdminService/ResolveReviewItem` | Admin | default | `ResolveReviewItemRequest` → `ResolveReviewItemResponse` | Resolves a review item, optionally recording that it was converted into a Q&A-bank entry (the entry itself is authored in `content/qa/`). |
| [`ReplyEscalation`](#adminservice-replyescalation) | `/api/career.v1.AdminService/ReplyEscalation` | Admin | default | `ReplyEscalationRequest` → `ReplyEscalationResponse` | Replies to an escalation. |
| [`GetMemberConversation`](#adminservice-getmemberconversation) | `/api/career.v1.AdminService/GetMemberConversation` | Admin | default | `GetMemberConversationRequest` → `GetMemberConversationResponse` | Returns any member's conversation with messages, sources, and persona versions (read-only). |
| [`GetCorpusStatus`](#adminservice-getcorpusstatus) | `/api/career.v1.AdminService/GetCorpusStatus` | Admin | default | `GetCorpusStatusRequest` → `GetCorpusStatusResponse` | Returns corpus statistics: documents, chunks, Q&A entries, last ingest, embedding model, and persona version. |
| [`TestRetrieval`](#adminservice-testretrieval) | `/api/career.v1.AdminService/TestRetrieval` | Admin | default | `TestRetrievalRequest` → `TestRetrievalResponse` | Runs retrieval for a question and returns the chunks the assistant would see, with scores ("what would the assistant retrieve?"). |
| [`RunJob`](#adminservice-runjob) | `/api/career.v1.AdminService/RunJob` | Admin | default | `RunJobRequest` → `RunJobResponse` | Starts a background job inside the api (corpus reindex of the public or private mount, embed sweep) and returns its id at once; the console polls GetJob for progress. |
| [`GetJob`](#adminservice-getjob) | `/api/career.v1.AdminService/GetJob` | Admin | default | `GetJobRequest` → `GetJobResponse` | Returns the status and progress of a job started by RunJob. |
| [`GetPersona`](#adminservice-getpersona) | `/api/career.v1.AdminService/GetPersona` | Admin | default | `GetPersonaRequest` → `GetPersonaResponse` | Returns the active persona version and its history. |
| [`GetAnalytics`](#adminservice-getanalytics) | `/api/career.v1.AdminService/GetAnalytics` | Admin | default | `GetAnalyticsRequest` → `GetAnalyticsResponse` | Returns aggregate analytics for a date range. |
| [`GetAudit`](#adminservice-getaudit) | `/api/career.v1.AdminService/GetAudit` | Admin | default | `GetAuditRequest` → `GetAuditResponse` | Lists audit-log entries. |
| [`ListContactMessages`](#adminservice-listcontactmessages) | `/api/career.v1.AdminService/ListContactMessages` | Admin | default | `ListContactMessagesRequest` → `ListContactMessagesResponse` | Lists messages sent via the public contact form (FR-ADM-14). |
| [`ResolveContactMessage`](#adminservice-resolvecontactmessage) | `/api/career.v1.AdminService/ResolveContactMessage` | Admin | default | `ResolveContactMessageRequest` → `ResolveContactMessageResponse` | Marks a contact message as resolved (or re-opens it). |
| [`ApproveRegistration`](#adminservice-approveregistration) | `/api/career.v1.AdminService/ApproveRegistration` | Admin | default | `ApproveRegistrationRequest` → `ApproveRegistrationResponse` | Approves a pending registration from the admin console. |
| [`DeclineRegistration`](#adminservice-declineregistration) | `/api/career.v1.AdminService/DeclineRegistration` | Admin | default | `DeclineRegistrationRequest` → `DeclineRegistrationResponse` | Declines a pending registration from the admin console. |
| [`ExtendAccess`](#adminservice-extendaccess) | `/api/career.v1.AdminService/ExtendAccess` | Admin | default | `ExtendAccessRequest` → `ExtendAccessResponse` | Extends an active member's access period by a fixed duration (`extend_days`), or sets a specific new `expires_at`. |
| [`ListDbTables`](#adminservice-listdbtables) | `/api/career.v1.AdminService/ListDbTables` | Admin | default | `ListDbTablesRequest` → `ListDbTablesResponse` | Returns the public tables + columns of the API database, from information_schema. |
| [`RunDbQuery`](#adminservice-rundbquery) | `/api/career.v1.AdminService/RunDbQuery` | Admin | default | `RunDbQueryRequest` → `RunDbQueryResponse` | Runs a SQL query against the API database from the /admin/db console. |
| [`ListAccessGrants`](#adminservice-listaccessgrants) | `/api/career.v1.AdminService/ListAccessGrants` | Admin | default | `ListAccessGrantsRequest` → `ListAccessGrantsResponse` | Lists the access whitelist entries. |
| [`UpsertAccessGrant`](#adminservice-upsertaccessgrant) | `/api/career.v1.AdminService/UpsertAccessGrant` | Admin | default | `UpsertAccessGrantRequest` → `UpsertAccessGrantResponse` | Creates a new access whitelist entry or updates an existing one by email (email is the natural key). |
| [`DeleteAccessGrant`](#adminservice-deleteaccessgrant) | `/api/career.v1.AdminService/DeleteAccessGrant` | Admin | default | `DeleteAccessGrantRequest` → `DeleteAccessGrantResponse` | Removes a whitelist entry. |
| [`ListSavedQueries`](#adminservice-listsavedqueries) | `/api/career.v1.AdminService/ListSavedQueries` | Admin | default | `ListSavedQueriesRequest` → `ListSavedQueriesResponse` | Returns the calling admin's saved SQL statements from /admin/db. |
| [`UpsertSavedQuery`](#adminservice-upsertsavedquery) | `/api/career.v1.AdminService/UpsertSavedQuery` | Admin | default | `UpsertSavedQueryRequest` → `UpsertSavedQueryResponse` | Creates or updates a saved query for the caller. |
| [`DeleteSavedQuery`](#adminservice-deletesavedquery) | `/api/career.v1.AdminService/DeleteSavedQuery` | Admin | default | `DeleteSavedQueryRequest` → `DeleteSavedQueryResponse` | Removes one of the caller's saved queries by id. |
| [`ListMemberActivity`](#adminservice-listmemberactivity) | `/api/career.v1.AdminService/ListMemberActivity` | Admin | default | `ListMemberActivityRequest` → `ListMemberActivityResponse` | Returns one row per member with engagement aggregates (session count, total active time, ask-roger count, last event). |
| [`IngestCorpusText`](#adminservice-ingestcorpustext) | `/api/career.v1.AdminService/IngestCorpusText` | Admin | default | `IngestCorpusTextRequest` → `IngestCorpusTextResponse` | Ingests one text document into the Ask Roger corpus. |
| [`ListCorpusDocuments`](#adminservice-listcorpusdocuments) | `/api/career.v1.AdminService/ListCorpusDocuments` | Admin | default | `ListCorpusDocumentsRequest` → `ListCorpusDocumentsResponse` | Returns every document currently in the corpus with a per-row chunk count. |
| [`ReindexCorpus`](#adminservice-reindexcorpus) | `/api/career.v1.AdminService/ReindexCorpus` | Admin | default | `ReindexCorpusRequest` → `ReindexCorpusResponse` | Walks a corpus mount for markdown files and runs each through IngestCorpusText, so the corpus can be seeded from committed content instead of paste-by-paste. |
| [`SweepCorpusEmbeddings`](#adminservice-sweepcorpusembeddings) | `/api/career.v1.AdminService/SweepCorpusEmbeddings` | Admin | default | `SweepCorpusEmbeddingsRequest` → `SweepCorpusEmbeddingsResponse` | Re-embeds every chunk whose vector is missing or was produced by a different embedder than the sidecar's current one. |
| [`ListJdSubmissions`](#adminservice-listjdsubmissions) | `/api/career.v1.AdminService/ListJdSubmissions` | Admin | default | `ListJdSubmissionsRequest` → `ListJdSubmissionsResponse` | Returns every JD submission with score + status. |
| [`GetJdSubmission`](#adminservice-getjdsubmission) | `/api/career.v1.AdminService/GetJdSubmission` | Admin | default | `GetJdSubmissionRequest` → `GetJdSubmissionResponse` | Returns one JD submission in full: the JD text, both scores, the assessment derivation (requirements, evidence, verdicts) and the generated résumé when present. |
| [`RescoreJd`](#adminservice-rescorejd) | `/api/career.v1.AdminService/RescoreJd` | Admin | default | `RescoreJdRequest` → `RescoreJdResponse` | Re-runs the scoring pipeline (retrieval pre-score for diagnostics, per-requirement assessment, score in code, résumé and locked PDF when the fit is strong or better) for one submission in the background, e.g. |
| [`SetJdOutcome`](#adminservice-setjdoutcome) | `/api/career.v1.AdminService/SetJdOutcome` | Admin | default | `SetJdOutcomeRequest` → `SetJdOutcomeResponse` | Records what happened in the world after a review: applied, interview, offer, no response. |
| [`RecordJdFeedback`](#adminservice-recordjdfeedback) | `/api/career.v1.AdminService/RecordJdFeedback` | Admin | default | `RecordJdFeedbackRequest` → `RecordJdFeedbackResponse` | Records the owner's judgment of one run's output: whether the score was accurate, too generous or too harsh, and whether the résumé is sendable. |
| [`ListGoldenPostings`](#adminservice-listgoldenpostings) | `/api/career.v1.AdminService/ListGoldenPostings` | Admin | default | `ListGoldenPostingsRequest` → `ListGoldenPostingsResponse` | Lists the golden set: fixed postings with a stated expectation, re-scored to measure whether a prompt or model change helped. |
| [`UpsertGoldenPosting`](#adminservice-upsertgoldenposting) | `/api/career.v1.AdminService/UpsertGoldenPosting` | Admin | default | `UpsertGoldenPostingRequest` → `UpsertGoldenPostingResponse` | Adds or replaces a golden posting, keyed by name. |
| [`SetGoldenActive`](#adminservice-setgoldenactive) | `/api/career.v1.AdminService/SetGoldenActive` | Admin | default | `SetGoldenActiveRequest` → `SetGoldenActiveResponse` | Retires or restores a golden posting. |
| [`LabelGoldenPosting`](#adminservice-labelgoldenposting) | `/api/career.v1.AdminService/LabelGoldenPosting` | Admin | default | `LabelGoldenPostingRequest` → `LabelGoldenPostingResponse` | Records which side of the gate a posting belongs on. |
| [`ListEvalRuns`](#adminservice-listevalruns) | `/api/career.v1.AdminService/ListEvalRuns` | Admin | default | `ListEvalRunsRequest` → `ListEvalRunsResponse` | Lists evaluations, newest first, without their per-posting results. |
| [`GetEvalRun`](#adminservice-getevalrun) | `/api/career.v1.AdminService/GetEvalRun` | Admin | default | `GetEvalRunRequest` → `GetEvalRunResponse` | Returns one evaluation with every posting's result. |
| [`GetMetrics`](#adminservice-getmetrics) | `/api/career.v1.AdminService/GetMetrics` | Admin | default | `GetMetricsRequest` → `GetMetricsResponse` | Returns the state of the reviewer, read from the SQL views that define each metric once. |
| [`GetOpsStatus`](#adminservice-getopsstatus) | `/api/career.v1.AdminService/GetOpsStatus` | Admin | default | `GetOpsStatusRequest` → `GetOpsStatusResponse` | Returns what the box is doing right now: jobs the runner knows about, the submission pipeline, recent model activity and host load. |
| [`GetJobDetail`](#adminservice-getjobdetail) | `/api/career.v1.AdminService/GetJobDetail` | Admin | default | `GetJobDetailRequest` → `GetJobDetailResponse` | Returns one job with every progress report it made, for the detail view on /admin/ops. |
| [`GetGate`](#adminservice-getgate) | `/api/career.v1.AdminService/GetGate` | Admin | default | `GetGateRequest` → `GetGateResponse` | Returns the criteria as a gate: one row per criterion with pass, value, target and as_of, read from the views that define them. |
| [`ListDecisionLog`](#adminservice-listdecisionlog) | `/api/career.v1.AdminService/ListDecisionLog` | Admin | default | `ListDecisionLogRequest` → `ListDecisionLogResponse` | Lists logged reviewer decisions (per-requirement verdicts, gate outcomes) with the evidence each was made from, for the owner's human-in-the-loop review. |
| [`ReviewDecision`](#adminservice-reviewdecision) | `/api/career.v1.AdminService/ReviewDecision` | Admin | default | `ReviewDecisionRequest` → `ReviewDecisionResponse` | Records the owner's own verdict and note on one logged decision. |
| [`ExportDecisionLog`](#adminservice-exportdecisionlog) | `/api/career.v1.AdminService/ExportDecisionLog` | Admin | default | `ExportDecisionLogRequest` → `ExportDecisionLogResponse` | Exports decisions as JSON Lines for adapter training and evaluation; reviewed rows carry the human label. |
| [`GetJdFitBands`](#adminservice-getjdfitbands) | `/api/career.v1.AdminService/GetJdFitBands` | Admin | default | `GetJdFitBandsRequest` → `GetJdFitBandsResponse` | Reads the JD fit bands (the numbers that classify a review as very strong / strong / possible / weak / very weak; "strong" is the gate). |
| [`SetJdFitBands`](#adminservice-setjdfitbands) | `/api/career.v1.AdminService/SetJdFitBands` | Admin | default | `SetJdFitBandsRequest` → `SetJdFitBandsResponse` | Sets the JD fit bands (stored in app_settings; the api caches them for 15 s). |
| [`GetDecisionTestSettings`](#adminservice-getdecisiontestsettings) | `/api/career.v1.AdminService/GetDecisionTestSettings` | Admin | default | `GetDecisionTestSettingsRequest` → `GetDecisionTestSettingsResponse` | Reads the decision test's timings. |
| [`SetDecisionTestSettings`](#adminservice-setdecisiontestsettings) | `/api/career.v1.AdminService/SetDecisionTestSettings` | Admin | default | `SetDecisionTestSettingsRequest` → `SetDecisionTestSettingsResponse` | Sets the decision test's timings. |
| [`ListDecisionTestRuns`](#adminservice-listdecisiontestruns) | `/api/career.v1.AdminService/ListDecisionTestRuns` | Admin | default | `ListDecisionTestRunsRequest` → `ListDecisionTestRunsResponse` | Lists decision test runs, newest first. |
| [`GetDecisionTestRun`](#adminservice-getdecisiontestrun) | `/api/career.v1.AdminService/GetDecisionTestRun` | Admin | default | `GetDecisionTestRunRequest` → `GetDecisionTestRunResponse` | One run in full: every answer, every recall, and the block summary. |
| [`ExportDecisionTestData`](#adminservice-exportdecisiontestdata) | `/api/career.v1.AdminService/ExportDecisionTestData` | Admin | default | `ExportDecisionTestDataRequest` → `ExportDecisionTestDataResponse` | The curated dataset as CSV, one row per question presented. |
| [`GetDecisionTestAnalysis`](#adminservice-getdecisiontestanalysis) | `/api/career.v1.AdminService/GetDecisionTestAnalysis` | Admin | default | `GetDecisionTestAnalysisRequest` → `GetDecisionTestAnalysisResponse` | Reads the decision test's three analysis views. |
| [`ExportDecisionTestRun`](#adminservice-exportdecisiontestrun) | `/api/career.v1.AdminService/ExportDecisionTestRun` | Admin | default | `ExportDecisionTestRunRequest` → `ExportDecisionTestRunResponse` | Downloads one run's blocks and answers as two CSV files. |
| [`ReviewDecisionTestRun`](#adminservice-reviewdecisiontestrun) | `/api/career.v1.AdminService/ReviewDecisionTestRun` | Admin | default | `ReviewDecisionTestRunRequest` → `ReviewDecisionTestRunResponse` | Records the owner's judgement about a run, or about one block of one, and returns the run as it now reads. |
| [`GetSchedulerSettings`](#adminservice-getschedulersettings) | `/api/career.v1.AdminService/GetSchedulerSettings` | Admin | default | `GetSchedulerSettingsRequest` → `GetSchedulerSettingsResponse` | Reads the meeting-scheduler settings: the weekly windows a member may book into, the lengths on offer, the clearance between meetings, and the zone all of it is quoted in. |
| [`SetSchedulerSettings`](#adminservice-setschedulersettings) | `/api/career.v1.AdminService/SetSchedulerSettings` | Admin | default | `SetSchedulerSettingsRequest` → `SetSchedulerSettingsResponse` | Replaces the meeting-scheduler settings (stored in app_settings; the api caches them for 15 s). |
| [`ListQaEntries`](#adminservice-listqaentries) | `/api/career.v1.AdminService/ListQaEntries` | Admin | default | `ListQaEntriesRequest` → `ListQaEntriesResponse` | Lists the Q&A bank: the owner's own answers, served verbatim by Ask Roger with no model involved. |
| [`CreateQaEntry`](#adminservice-createqaentry) | `/api/career.v1.AdminService/CreateQaEntry` | Admin | default | `CreateQaEntryRequest` → `CreateQaEntryResponse` | Writes a new bank entry along with its canonical phrasing. |
| [`UpdateQaEntry`](#adminservice-updateqaentry) | `/api/career.v1.AdminService/UpdateQaEntry` | Admin | default | `UpdateQaEntryRequest` → `UpdateQaEntryResponse` | Replaces an entry's editable fields. |
| [`SetQaEntryEnabled`](#adminservice-setqaentryenabled) | `/api/career.v1.AdminService/SetQaEntryEnabled` | Admin | default | `SetQaEntryEnabledRequest` → `SetQaEntryEnabledResponse` | Approves or withdraws an entry. |
| [`DeleteQaEntry`](#adminservice-deleteqaentry) | `/api/career.v1.AdminService/DeleteQaEntry` | Admin | default | `DeleteQaEntryRequest` → `DeleteQaEntryResponse` | Deletes an entry and its phrasings. |
| [`AddQaPhrasing`](#adminservice-addqaphrasing) | `/api/career.v1.AdminService/AddQaPhrasing` | Admin | default | `AddQaPhrasingRequest` → `AddQaPhrasingResponse` | Adds another way of asking an existing entry's question. |
| [`DeleteQaPhrasing`](#adminservice-deleteqaphrasing) | `/api/career.v1.AdminService/DeleteQaPhrasing` | Admin | default | `DeleteQaPhrasingRequest` → `DeleteQaPhrasingResponse` | Removes one variant phrasing. |
| [`GetCalendarConnectURL`](#adminservice-getcalendarconnecturl) | `/api/career.v1.AdminService/GetCalendarConnectURL` | Admin | default | `GetCalendarConnectURLRequest` → `GetCalendarConnectURLResponse` | Returns the Google consent URL the owner visits to connect his calendar, carrying a signed, short-lived state so the callback cannot be driven by anyone else. |
| [`ConnectCalendar`](#adminservice-connectcalendar) | `/api/career.v1.AdminService/ConnectCalendar` | Admin | default | `ConnectCalendarRequest` → `ConnectCalendarResponse` | Completes the handshake: exchanges the authorisation code for a refresh token and stores it encrypted at rest. |
| [`GetCalendarStatus`](#adminservice-getcalendarstatus) | `/api/career.v1.AdminService/GetCalendarStatus` | Admin | default | `GetCalendarStatusRequest` → `GetCalendarStatusResponse` | Reads the calendar connection: which account, when it was connected, and whether it is currently working. |
| [`DisconnectCalendar`](#adminservice-disconnectcalendar) | `/api/career.v1.AdminService/DisconnectCalendar` | Admin | default | `DisconnectCalendarRequest` → `DisconnectCalendarResponse` | Forgets the stored credential. |
| [`ListMeetings`](#adminservice-listmeetings) | `/api/career.v1.AdminService/ListMeetings` | Admin | default | `ListMeetingsRequest` → `ListMeetingsResponse` | Lists booked meetings, soonest first, so the owner can see what has been taken without opening Google. |
| [`CancelMeetingAsAdmin`](#adminservice-cancelmeetingasadmin) | `/api/career.v1.AdminService/CancelMeetingAsAdmin` | Admin | default | `CancelMeetingAsAdminRequest` → `CancelMeetingAsAdminResponse` | Cancels a meeting on the member's behalf and frees the slot. |
| [`GetJdSubmissionLimit`](#adminservice-getjdsubmissionlimit) | `/api/career.v1.AdminService/GetJdSubmissionLimit` | Admin | default | `GetJdSubmissionLimitRequest` → `GetJdSubmissionLimitResponse` | Reads how many postings one member may submit per rolling day. |
| [`SetJdSubmissionLimit`](#adminservice-setjdsubmissionlimit) | `/api/career.v1.AdminService/SetJdSubmissionLimit` | Admin | default | `SetJdSubmissionLimitRequest` → `SetJdSubmissionLimitResponse` | Sets how many postings one member may submit per rolling day (stored in app_settings; the api caches it for 15 s). |

### AdminService.ListMembers

`POST /api/career.v1.AdminService/ListMembers` · **Auth:** Admin · **Rate limit:** default/min

Lists members with search, filters, and pagination.

**Request** — [`ListMembersRequest`](#listmembersrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: max_len: 200` | Case-insensitive match on name, email, or organization. |
| `status` | [`MemberStatus`](#memberstatus) | string (enum name) | `enum: defined_only: true` | Restrict to one status; unspecified returns all. |
| `trackId` | `string` | string | `string: max_len: 64` | Restrict to members interested in this track. |
| `sort` | [`ListMembersRequest.Sort`](#listmembersrequestsort) | string (enum name) | `enum: defined_only: true` | Sort order. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`ListMembersResponse`](#listmembersresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `members` | [`MemberRecord`](#memberrecord)[] | array of object |  | Members. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

<details><summary>Example request body</summary>

```json
{
  "query": "string",
  "status": "MEMBER_STATUS_UNVERIFIED",
  "trackId": "string",
  "sort": "SORT_NEWEST",
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

### AdminService.ResendNotification

`POST /api/career.v1.AdminService/ResendNotification` · **Auth:** Admin · **Rate limit:** default/min

Re-sends the approval or decline email to a member and returns
the audited attempt so the console can show the provider's verdict
inline. Backs the "Resend" button on /admin/registrations/[id].

**Request** — [`ResendNotificationRequest`](#resendnotificationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID. |
| `kind` | `string` | string | `string: min_len: 1 max_len: 40` | Which mail to re-fire. Only `user_approved` (member must be active) and `user_declined` (member must be declined) are resendable; token-bearing mails (verify, reset) are not. |

**Response** — [`ResendNotificationResponse`](#resendnotificationresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `delivery` | [`NotificationDelivery`](#notificationdelivery) | object |  | The audited attempt, including any provider error. |

<details><summary>Example request body</summary>

```json
{
  "memberId": "string",
  "kind": "string"
}
```

</details>

### AdminService.GetMember

`POST /api/career.v1.AdminService/GetMember` · **Auth:** Admin · **Rate limit:** default/min

Returns one member with recent activity, conversations, admin notes,
and email delivery history.

**Request** — [`GetMemberRequest`](#getmemberrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID. |

**Response** — [`GetMemberResponse`](#getmemberresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | The member. |
| `recentActivity` | [`ActivityEvent`](#activityevent)[] | array of object |  | Most recent 50 activity events, newest first. |
| `conversations` | [`Conversation`](#conversation)[] | array of object |  | Conversations, most recent first. |
| `notes` | [`AdminNote`](#adminnote)[] | array of object |  | Admin notes, newest first. |
| `deliveries` | [`NotificationDelivery`](#notificationdelivery)[] | array of object |  | Most recent 20 email attempts, newest first. |

<details><summary>Example request body</summary>

```json
{
  "memberId": "string"
}
```

</details>

### AdminService.AddMemberNote

`POST /api/career.v1.AdminService/AddMemberNote` · **Auth:** Admin · **Rate limit:** default/min

Adds a private admin note to a member.

**Request** — [`AddMemberNoteRequest`](#addmembernoterequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID. |
| `text` | `string` | string | `string: min_len: 1 max_len: 4000` | Note text. |

**Response** — [`AddMemberNoteResponse`](#addmembernoteresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `note` | [`AdminNote`](#adminnote) | object |  | The note. |

<details><summary>Example request body</summary>

```json
{
  "memberId": "string",
  "text": "string"
}
```

</details>

### AdminService.SetMemberStatus

`POST /api/career.v1.AdminService/SetMemberStatus` · **Auth:** Admin · **Rate limit:** default/min

Approves or rejects a registration waiting in PENDING_APPROVAL (approval
mode only), or disables/re-enables an account.

**Request** — [`SetMemberStatusRequest`](#setmemberstatusrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID. |
| `status` | [`MemberStatus`](#memberstatus) | string (enum name) | `enum: in: 2 in: 4` | New status: ACTIVE (approve or re-enable) or DISABLED. |
| `reason` | `string` | string | `string: max_len: 1000` | Reason, recorded in the audit log. |

**Response** — [`SetMemberStatusResponse`](#setmemberstatusresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | The member after the change. |

<details><summary>Example request body</summary>

```json
{
  "memberId": "string",
  "status": "MEMBER_STATUS_UNVERIFIED",
  "reason": "string"
}
```

</details>

### AdminService.GetReviewQueue

`POST /api/career.v1.AdminService/GetReviewQueue` · **Auth:** Admin · **Rate limit:** default/min

Lists items needing the owner's attention: negative feedback,
"I don't know" answers, escalations, and out-of-scope refusals.

**Request** — [`GetReviewQueueRequest`](#getreviewqueuerequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`ReviewKind`](#reviewkind) | string (enum name) | `enum: defined_only: true` | Restrict to one kind. |
| `status` | [`ReviewStatus`](#reviewstatus) | string (enum name) | `enum: defined_only: true` | Restrict to one status; unspecified returns OPEN items. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`GetReviewQueueResponse`](#getreviewqueueresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`ReviewItem`](#reviewitem)[] | array of object |  | Items, oldest open first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |
| `openCount` | `int32` | number |  | Open items in total. |

<details><summary>Example request body</summary>

```json
{
  "kind": "REVIEW_KIND_NEGATIVE_FEEDBACK",
  "status": "REVIEW_STATUS_OPEN",
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

### AdminService.ResolveReviewItem

`POST /api/career.v1.AdminService/ResolveReviewItem` · **Auth:** Admin · **Rate limit:** default/min

Resolves a review item, optionally recording that it was converted into
a Q&A-bank entry (the entry itself is authored in `content/qa/`).

**Request** — [`ResolveReviewItemRequest`](#resolvereviewitemrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `reviewItemId` | `string` | string | `string: min_len: 1 max_len: 64` | Review item ID. |
| `resolution` | [`ReviewStatus`](#reviewstatus) | string (enum name) | `enum: in: 2 in: 3` | Resolution: RESOLVED or CONVERTED_TO_QA. |
| `note` | `string` | string | `string: max_len: 1000` | Note for the audit log; optional. |

**Response** — [`ResolveReviewItemResponse`](#resolvereviewitemresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "reviewItemId": "string",
  "resolution": "REVIEW_STATUS_OPEN",
  "note": "string"
}
```

</details>

### AdminService.ReplyEscalation

`POST /api/career.v1.AdminService/ReplyEscalation` · **Auth:** Admin · **Rate limit:** default/min

Replies to an escalation. The reply is appended to the member's
conversation as an OWNER message and the member is emailed.

**Request** — [`ReplyEscalationRequest`](#replyescalationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `escalationId` | `string` | string | `string: min_len: 1 max_len: 64` | Escalation ID. |
| `text` | `string` | string | `string: min_len: 1 max_len: 8000` | Reply text (Markdown). |
| `notifyMember` | `bool` | boolean |  | Also email the member (default true when unset in the UI). |

**Response** — [`ReplyEscalationResponse`](#replyescalationresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | [`Message`](#message) | object |  | The OWNER message appended to the conversation. |

<details><summary>Example request body</summary>

```json
{
  "escalationId": "string",
  "text": "string",
  "notifyMember": true
}
```

</details>

### AdminService.GetMemberConversation

`POST /api/career.v1.AdminService/GetMemberConversation` · **Auth:** Admin · **Rate limit:** default/min

Returns any member's conversation with messages, sources, and persona
versions (read-only).

**Request** — [`GetMemberConversationRequest`](#getmemberconversationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation ID. |

**Response** — [`GetMemberConversationResponse`](#getmemberconversationresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`Me`](#me) | object |  | The member. |
| `conversation` | [`Conversation`](#conversation) | object |  | The conversation. |
| `messages` | [`Message`](#message)[] | array of object |  | Messages in chronological order, with citations. |

<details><summary>Example request body</summary>

```json
{
  "conversationId": "string"
}
```

</details>

### AdminService.GetCorpusStatus

`POST /api/career.v1.AdminService/GetCorpusStatus` · **Auth:** Admin · **Rate limit:** default/min

Returns corpus statistics: documents, chunks, Q&A entries, last ingest,
embedding model, and persona version.

**Request** — [`GetCorpusStatusRequest`](#getcorpusstatusrequest)

_No fields; send `{}`._

**Response** — [`GetCorpusStatusResponse`](#getcorpusstatusresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `documents` | `int32` | number |  | Indexed documents. |
| `chunks` | `int32` | number |  | Chunks with embeddings. |
| `qaEntries` | `int32` | number |  | Q&A-bank entries. |
| `documentsByType` | map<`string`, `int32`> | object of number |  | Documents by content type, keyed by the enum name (e.g. `CONTENT_TYPE_ARTICLE`). |
| `privateDocuments` | `int32` | number |  | Private-corpus documents (not linked to site content). |
| `lastIngestAt` | `Timestamp` | string (RFC 3339, UTC) |  | Last successful ingest. |
| `embeddingModel` | `string` | string |  | Embedding model identifier. |
| `embeddingDimensions` | `int32` | number |  | Embedding dimensions. |
| `personaVersion` | `string` | string |  | Active persona version. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.TestRetrieval

`POST /api/career.v1.AdminService/TestRetrieval` · **Auth:** Admin · **Rate limit:** default/min

Runs retrieval for a question and returns the chunks the assistant
would see, with scores ("what would the assistant retrieve?").

**Request** — [`TestRetrievalRequest`](#testretrievalrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: min_len: 1 max_len: 4000` | The question to retrieve for. |
| `topK` | `int32` | number | `int32: lte: 50 gte: 0` | Chunks to return, 1–50; zero selects the assistant's default. |
| `rerank` | `bool` | boolean |  | Apply the sidecar reranker after hybrid retrieval. |

**Response** — [`TestRetrievalResponse`](#testretrievalresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `hits` | [`RetrievalHit`](#retrievalhit)[] | array of object |  | Chunks in rank order. |
| `qaMatch` | [`ContentSummary`](#contentsummary) | object |  | Matching Q&A-bank entry, when the question matched one above threshold. |
| `qaSimilarity` | `float` | number |  | Similarity of the Q&A match. |
| `latencyMs` | `int32` | number |  | Retrieval latency in milliseconds. |

<details><summary>Example request body</summary>

```json
{
  "query": "string",
  "topK": 0,
  "rerank": true
}
```

</details>

### AdminService.RunJob

`POST /api/career.v1.AdminService/RunJob` · **Auth:** Admin · **Rate limit:** default/min

Starts a background job inside the api (corpus reindex of the public
or private mount, embed sweep) and returns its id at once; the
console polls GetJob for progress. The sidecar JobKind values are
reserved and rejected as not runnable here. One job at a time.

**Request** — [`RunJobRequest`](#runjobrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`JobKind`](#jobkind) | string (enum name) | `enum: defined_only: true not_in: 0` | Which job. |
| `sourceKind` | `string` | string | `string: max_len: 40` | For the corpus reindex jobs: restrict the walk to one source_kind; empty walks every kind. |

**Response** — [`RunJobResponse`](#runjobresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string |  | Job ID. |

<details><summary>Example request body</summary>

```json
{
  "kind": "JOB_KIND_CONTENT_VALIDATE",
  "sourceKind": "string"
}
```

</details>

### AdminService.GetJob

`POST /api/career.v1.AdminService/GetJob` · **Auth:** Admin · **Rate limit:** default/min

Returns the status and progress of a job started by RunJob.

**Request** — [`GetJobRequest`](#getjobrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string | `string: min_len: 1 max_len: 64` | Job ID. |

**Response** — [`GetJobResponse`](#getjobresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string |  | Job ID. |
| `kind` | [`JobKind`](#jobkind) | string (enum name) |  | Which job. |
| `status` | [`JobStatus`](#jobstatus) | string (enum name) |  | Status. |
| `progressPct` | `int32` | number |  | Progress in percent, when the job reports it. |
| `startedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Start time. |
| `finishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Finish time. |
| `summary` | `string` | string |  | Human-readable summary or error. |

<details><summary>Example request body</summary>

```json
{
  "jobId": "string"
}
```

</details>

### AdminService.GetPersona

`POST /api/career.v1.AdminService/GetPersona` · **Auth:** Admin · **Rate limit:** default/min

Returns the active persona version and its history. The prompt text is
governed in the repository and pinned by its ULTS hash; this shows which
version is live.

**Request** — [`GetPersonaRequest`](#getpersonarequest)

_No fields; send `{}`._

**Response** — [`GetPersonaResponse`](#getpersonaresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `active` | [`PersonaVersion`](#personaversion) | object |  | The live version. |
| `history` | [`PersonaVersion`](#personaversion)[] | array of object |  | Previous versions, newest first. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.GetAnalytics

`POST /api/career.v1.AdminService/GetAnalytics` · **Auth:** Admin · **Rate limit:** default/min

Returns aggregate analytics for a date range.

**Request** — [`GetAnalyticsRequest`](#getanalyticsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `from` | `Timestamp` | string (RFC 3339, UTC) | `required: true` | Range start (inclusive). |
| `to` | `Timestamp` | string (RFC 3339, UTC) | `required: true` | Range end (exclusive). |

**Response** — [`GetAnalyticsResponse`](#getanalyticsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `registrations` | [`DailyCount`](#dailycount)[] | array of object |  | Registrations per day. |
| `verifications` | [`DailyCount`](#dailycount)[] | array of object |  | Verifications per day. |
| `activeMembers` | [`DailyCount`](#dailycount)[] | array of object |  | Active members per day (any activity). |
| `questionnairesCompleted` | [`DailyCount`](#dailycount)[] | array of object |  | Members who completed the questionnaire, per day. |
| `topContent` | [`RankedContent`](#rankedcontent)[] | array of object |  | Most viewed content. |
| `topDownloads` | [`RankedContent`](#rankedcontent)[] | array of object |  | Most downloaded files. |
| `topQuestions` | [`RankedQuestion`](#rankedquestion)[] | array of object |  | Most asked questions. |
| `assistant` | [`AssistantUsage`](#assistantusage) | object |  | Assistant usage and cost. |
| `membersByTrack` | map<`string`, `int32`> | object of number |  | Members by track interest, keyed by track ID. |

<details><summary>Example request body</summary>

```json
{
  "from": "2026-09-18T12:00:00Z",
  "to": "2026-09-18T12:00:00Z"
}
```

</details>

### AdminService.GetAudit

`POST /api/career.v1.AdminService/GetAudit` · **Auth:** Admin · **Rate limit:** default/min

Lists audit-log entries.

**Request** — [`GetAuditRequest`](#getauditrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `actorId` | `string` | string | `string: max_len: 64` | Restrict to one actor (member ID); empty for all. |
| `actionPrefix` | `string` | string | `string: max_len: 64` | Restrict to an action prefix, e.g. `auth.` or `admin.`. |
| `from` | `Timestamp` | string (RFC 3339, UTC) |  | Range start; optional. |
| `to` | `Timestamp` | string (RFC 3339, UTC) |  | Range end; optional. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`GetAuditResponse`](#getauditresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `entries` | [`AuditEntry`](#auditentry)[] | array of object |  | Entries, newest first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

<details><summary>Example request body</summary>

```json
{
  "actorId": "string",
  "actionPrefix": "string",
  "from": "2026-09-18T12:00:00Z",
  "to": "2026-09-18T12:00:00Z",
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

### AdminService.ListContactMessages

`POST /api/career.v1.AdminService/ListContactMessages` · **Auth:** Admin · **Rate limit:** default/min

Lists messages sent via the public contact form (FR-ADM-14). Backs
/admin/contacts in the console; every submission from
ContactService.SubmitContact lands in the same support_messages
table this reads from.

**Request** — [`ListContactMessagesRequest`](#listcontactmessagesrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: max_len: 200` | Case-insensitive substring match against subject / body / sender. |
| `status` | [`SupportStatus`](#supportstatus) | string (enum name) | `enum: defined_only: true` | Restrict to one status; unspecified returns all. |
| `category` | [`SupportCategory`](#supportcategory) | string (enum name) | `enum: defined_only: true` | Restrict to one category; unspecified returns all. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

**Response** — [`ListContactMessagesResponse`](#listcontactmessagesresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `messages` | [`SupportMessage`](#supportmessage)[] | array of object |  | Messages matching the filter, newest first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |
| `openCount` | `int32` | number |  | Unfiltered count of messages currently in the "open" state. |
| `resolvedCount` | `int32` | number |  | Unfiltered count of messages currently in the "resolved" state. |

<details><summary>Example request body</summary>

```json
{
  "query": "string",
  "status": "SUPPORT_STATUS_OPEN",
  "category": "SUPPORT_CATEGORY_GENERAL_QUESTION",
  "page": {
    "pageSize": 0,
    "pageToken": "string"
  }
}
```

</details>

### AdminService.ResolveContactMessage

`POST /api/career.v1.AdminService/ResolveContactMessage` · **Auth:** Admin · **Rate limit:** default/min

Marks a contact message as resolved (or re-opens it). Records the
acting admin's user_id + timestamp so the audit trail is clean.

**Request** — [`ResolveContactMessageRequest`](#resolvecontactmessagerequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 32` | ID from SupportMessage.id. |
| `status` | [`SupportStatus`](#supportstatus) | string (enum name) | `enum: defined_only: true` | Target status. UNSPECIFIED defaults to RESOLVED. |

**Response** — [`ResolveContactMessageResponse`](#resolvecontactmessageresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | [`SupportMessage`](#supportmessage) | object |  | The message row after the status flip. |

<details><summary>Example request body</summary>

```json
{
  "id": "string",
  "status": "SUPPORT_STATUS_OPEN"
}
```

</details>

### AdminService.ApproveRegistration

`POST /api/career.v1.AdminService/ApproveRegistration` · **Auth:** Admin · **Rate limit:** default/min

Approves a pending registration from the admin console. Same DB
transitions + emails as the one-click Accept link, but recorded
as decided_via="console" in the audit trail.

**Request** — [`ApproveRegistrationRequest`](#approveregistrationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID from MemberRecord.me.id. |
| `reason` | `string` | string | `string: max_len: 1000` | Optional free-text reason recorded in the audit log. |

**Response** — [`ApproveRegistrationResponse`](#approveregistrationresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | Updated member record. |

<details><summary>Example request body</summary>

```json
{
  "memberId": "string",
  "reason": "string"
}
```

</details>

### AdminService.DeclineRegistration

`POST /api/career.v1.AdminService/DeclineRegistration` · **Auth:** Admin · **Rate limit:** default/min

Declines a pending registration from the admin console. Same DB
transitions + emails as the one-click Decline link, but recorded
as decided_via="console" in the audit trail.

**Request** — [`DeclineRegistrationRequest`](#declineregistrationrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID from MemberRecord.me.id. |
| `reason` | `string` | string | `string: max_len: 1000` | Optional free-text reason recorded in the audit log. |

**Response** — [`DeclineRegistrationResponse`](#declineregistrationresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | Updated member record. |

<details><summary>Example request body</summary>

```json
{
  "memberId": "string",
  "reason": "string"
}
```

</details>

### AdminService.ExtendAccess

`POST /api/career.v1.AdminService/ExtendAccess` · **Auth:** Admin · **Rate limit:** default/min

Extends an active member's access period by a fixed duration
(`extend_days`), or sets a specific new `expires_at`. Absolute
and relative are exclusive; passing both is InvalidArgument.

**Request** — [`ExtendAccessRequest`](#extendaccessrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID from MemberRecord.me.id. |
| `extendDays` | `int32` | number | `int32: lte: 3650 gte: 0` | Add this many days to the current expires_at (or, if the member has no expires_at set, from now). 1..3650. |
| `newExpiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | Set expires_at to this exact moment. |
| `permanent` | `bool` | boolean |  | Make access permanent (clears expires_at). |
| `reason` | `string` | string | `string: max_len: 1000` | Optional free-text reason recorded in the audit log. |

**Response** — [`ExtendAccessResponse`](#extendaccessresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | Updated member record. |

<details><summary>Example request body</summary>

```json
{
  "memberId": "string",
  "extendDays": 0,
  "newExpiresAt": "2026-09-18T12:00:00Z",
  "permanent": true,
  "reason": "string"
}
```

</details>

### AdminService.ListDbTables

`POST /api/career.v1.AdminService/ListDbTables` · **Auth:** Admin · **Rate limit:** default/min

Returns the public tables + columns of the API database, from
information_schema. Backs the schema panel on /admin/db.

**Request** — [`ListDbTablesRequest`](#listdbtablesrequest)

_No fields; send `{}`._

**Response** — [`ListDbTablesResponse`](#listdbtablesresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `tables` | [`DbTable`](#dbtable)[] | array of object |  | Public-schema tables, ordered by name. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.RunDbQuery

`POST /api/career.v1.AdminService/RunDbQuery` · **Auth:** Admin · **Rate limit:** default/min

Runs a SQL query against the API database from the /admin/db
console. MVP is read-only: only SELECT statements are accepted;
any other statement kind returns InvalidArgument. A per-statement
timeout is enforced server-side and the result row-count is
capped (rows past the cap are dropped with `truncated=true`).

**Request** — [`RunDbQueryRequest`](#rundbqueryrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sql` | `string` | string | `string: min_len: 1 max_len: 4000` | SQL statement to execute. MVP: only SELECT is accepted. |
| `timeoutMs` | `int32` | number |  | Statement timeout in milliseconds; server-clamped to [100, 10000]. |
| `sort` | `string` | string | `string: max_len: 64` | Optional column of the result to sort by (informational only — the client can reorder locally; the server passes it back so a caller can round-trip UI state). |

**Response** — [`RunDbQueryResponse`](#rundbqueryresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `columns` | `string`[] | array of string |  | Column names (order matches DbRow.cells). |
| `columnTypes` | `string`[] | array of string |  | SQL types (order matches columns). |
| `rows` | [`DbRow`](#dbrow)[] | array of object |  | Rows, in whatever order the query returned them. |
| `truncated` | `bool` | boolean |  | True when the result was cut off at the server-side row cap. |
| `rowCount` | `int32` | number |  | Number of rows returned (before truncation, if applicable — matches len(rows) when truncated is false). |
| `elapsedMs` | `int32` | number |  | Milliseconds the query took on the server, wall-clock. |

<details><summary>Example request body</summary>

```json
{
  "sql": "string",
  "timeoutMs": 0,
  "sort": "string"
}
```

</details>

### AdminService.ListAccessGrants

`POST /api/career.v1.AdminService/ListAccessGrants` · **Auth:** Admin · **Rate limit:** default/min

Lists the access whitelist entries. An entry with an email means
a registration from that address is auto-approved for
`default_ttl`. Backs /admin/access.

**Request** — [`ListAccessGrantsRequest`](#listaccessgrantsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: max_len: 200` | Case-insensitive substring match against email or notes. |

**Response** — [`ListAccessGrantsResponse`](#listaccessgrantsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `grants` | [`AccessGrant`](#accessgrant)[] | array of object |  | Whitelist entries, newest first. |
| `activeCount` | `int32` | number |  | Count of entries currently active (entry_expires_at unset or in the future). |
| `expiredCount` | `int32` | number |  | Count of entries whose entry_expires_at has passed. |

<details><summary>Example request body</summary>

```json
{
  "query": "string"
}
```

</details>

### AdminService.UpsertAccessGrant

`POST /api/career.v1.AdminService/UpsertAccessGrant` · **Auth:** Admin · **Rate limit:** default/min

Creates a new access whitelist entry or updates an existing one
by email (email is the natural key). The default_ttl controls how
long access lasts once the user signs up; entry_expires_at (optional)
controls how long the whitelist entry itself stays active.

**Request** — [`UpsertAccessGrantRequest`](#upsertaccessgrantrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `email` | `string` | string | `string: min_len: 3 max_len: 254 email: true` | Email to whitelist. Lower-cased server-side. |
| `defaultTtl` | [`GrantTTL`](#grantttl) | string (enum name) | `enum: defined_only: true not_in: 0` | TTL that new registrations get on approval. |
| `notes` | `string` | string | `string: max_len: 1000` | Free-text admin notes. |
| `entryExpiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | When this whitelist entry itself should lapse; unset = never. |

**Response** — [`UpsertAccessGrantResponse`](#upsertaccessgrantresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `grant` | [`AccessGrant`](#accessgrant) | object |  | The stored entry (with fields filled in by the server). |
| `created` | `bool` | boolean |  | True when the request created a new row; false when it updated an existing one. |

<details><summary>Example request body</summary>

```json
{
  "email": "string",
  "defaultTtl": "GRANT_TTL_1D",
  "notes": "string",
  "entryExpiresAt": "2026-09-18T12:00:00Z"
}
```

</details>

### AdminService.DeleteAccessGrant

`POST /api/career.v1.AdminService/DeleteAccessGrant` · **Auth:** Admin · **Rate limit:** default/min

Removes a whitelist entry. Existing accounts already granted
access are unaffected — this only stops future auto-approvals.

**Request** — [`DeleteAccessGrantRequest`](#deleteaccessgrantrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 32` | ID from AccessGrant.id. |

**Response** — [`DeleteAccessGrantResponse`](#deleteaccessgrantresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "string"
}
```

</details>

### AdminService.ListSavedQueries

`POST /api/career.v1.AdminService/ListSavedQueries` · **Auth:** Admin · **Rate limit:** default/min

Returns the calling admin's saved SQL statements from /admin/db.
Scoped to the caller — one admin never sees another's slots.

**Request** — [`ListSavedQueriesRequest`](#listsavedqueriesrequest)

_No fields; send `{}`._

**Response** — [`ListSavedQueriesResponse`](#listsavedqueriesresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `queries` | [`SavedQuery`](#savedquery)[] | array of object |  | The caller's saved queries, ordered by name (case-insensitive). |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.UpsertSavedQuery

`POST /api/career.v1.AdminService/UpsertSavedQuery` · **Auth:** Admin · **Rate limit:** default/min

Creates or updates a saved query for the caller. The tuple
(caller, name) is the natural key: passing an existing name
overwrites the body.

**Request** — [`UpsertSavedQueryRequest`](#upsertsavedqueryrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string | `string: min_len: 1 max_len: 80` | Label to save under. Trimmed server-side; empty is rejected. |
| `sql` | `string` | string | `string: min_len: 1 max_len: 4000` | The SQL text. Enforcement (SELECT-only, single statement) is applied at execute time by RunDbQuery, not on save — so an admin can save a draft and finish it later. |

**Response** — [`UpsertSavedQueryResponse`](#upsertsavedqueryresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | [`SavedQuery`](#savedquery) | object |  | The stored row (fields populated by the server). |
| `created` | `bool` | boolean |  | True when the request created a new row; false when it overwrote an existing (owner, name) tuple. |

<details><summary>Example request body</summary>

```json
{
  "name": "string",
  "sql": "string"
}
```

</details>

### AdminService.DeleteSavedQuery

`POST /api/career.v1.AdminService/DeleteSavedQuery` · **Auth:** Admin · **Rate limit:** default/min

Removes one of the caller's saved queries by id.

**Request** — [`DeleteSavedQueryRequest`](#deletesavedqueryrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 32` | ID from SavedQuery.id. |

**Response** — [`DeleteSavedQueryResponse`](#deletesavedqueryresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "string"
}
```

</details>

### AdminService.ListMemberActivity

`POST /api/career.v1.AdminService/ListMemberActivity` · **Auth:** Admin · **Rate limit:** default/min

Returns one row per member with engagement aggregates (session
count, total active time, ask-roger count, last event). Backs
/admin/activity — Roger's request for a sortable "who's using
the site" surface.

**Request** — [`ListMemberActivityRequest`](#listmemberactivityrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sort` | [`ActivitySort`](#activitysort) | string (enum name) | `enum: defined_only: true` | Which column to sort by. |

**Response** — [`ListMemberActivityResponse`](#listmemberactivityresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `members` | [`MemberActivitySummary`](#memberactivitysummary)[] | array of object |  | One row per member (up to 500). |

<details><summary>Example request body</summary>

```json
{
  "sort": "ACTIVITY_SORT_LAST_EVENT_DESC"
}
```

</details>

### AdminService.IngestCorpusText

`POST /api/career.v1.AdminService/IngestCorpusText` · **Auth:** Admin · **Rate limit:** default/min

Ingests one text document into the Ask Roger corpus. Chunks
it, embeds via the sidecar, and stores under (source_kind,
source_path). Idempotent: an identical body with the same
(source_kind, source_path) is a no-op (skipped=true). Backs
the paste-a-document form on /admin/corpus.

**Request** — [`IngestCorpusTextRequest`](#ingestcorpustextrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sourceKind` | `string` | string | `string: min_len: 1 max_len: 40` | Where the text came from: 'article', 'resume', 'career_note', 'adr', 'other'. Kept as free text (not enum) so a new source kind is just a new value at the ingest form, not a proto edit. |
| `sourcePath` | `string` | string | `string: min_len: 1 max_len: 200` | Stable per-source_kind key. Filesystem sources pass the relative path; pasted documents pass a slug like `paste/2026-09-21-notes`. |
| `title` | `string` | string | `string: max_len: 300` | Human-readable title. Empty falls back to the first non-blank line of the body (with a leading `#` stripped) at ingest time. |
| `body` | `string` | string | `string: min_len: 1 max_len: 204800` | The document text, post-front-matter for markdown. Capped at 200 KiB so the form doesn't paste in an unbounded blob. |
| `visibility` | `string` | string | `string: max_len: 16` | `public` (default) or `corpus_only`. |

**Response** — [`IngestCorpusTextResponse`](#ingestcorpustextresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `documentId` | `string` | string |  | ID of the stored corpus_documents row. |
| `chunksInserted` | `int32` | number |  | Number of chunks written this run (0 if skipped). |
| `chunksEmbedded` | `int32` | number |  | Number of chunks successfully embedded this run. |
| `skipped` | `bool` | boolean |  | True when the body's content hash matched the stored row and no chunk / embed work ran. |
| `chunkerName` | `string` | string |  | Stable identifier of the chunker version used (e.g. "paragraph.v1"). |
| `embedderModel` | `string` | string |  | Embedder model reported by the sidecar (e.g. "stub", "ollama"). |

<details><summary>Example request body</summary>

```json
{
  "sourceKind": "string",
  "sourcePath": "string",
  "title": "string",
  "body": "string",
  "visibility": "string"
}
```

</details>

### AdminService.ListCorpusDocuments

`POST /api/career.v1.AdminService/ListCorpusDocuments` · **Auth:** Admin · **Rate limit:** default/min

Returns every document currently in the corpus with a per-row
chunk count. Backs the list on /admin/corpus.

**Request** — [`ListCorpusDocumentsRequest`](#listcorpusdocumentsrequest)

_No fields; send `{}`._

**Response** — [`ListCorpusDocumentsResponse`](#listcorpusdocumentsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `documents` | [`CorpusDocumentRow`](#corpusdocumentrow)[] | array of object |  | All documents, newest first. |
| `totalDocuments` | `int32` | number |  | Aggregate document count across the whole corpus (rendered as a header chip on /admin/corpus). |
| `totalChunks` | `int32` | number |  | Aggregate chunk count across every stored document. |
| `totalEmbedded` | `int32` | number |  | Aggregate count of chunks that have an embedding populated. |
| `totalPublic` | `int32` | number |  | Documents with visibility=public. |
| `totalCorpusOnly` | `int32` | number |  | Documents with visibility=corpus_only. |
| `embedderCounts` | [`EmbedderCount`](#embeddercount)[] | array of object |  | Chunk counts grouped by the embedder that produced their vector. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.ReindexCorpus

`POST /api/career.v1.AdminService/ReindexCorpus` · **Auth:** Admin · **Rate limit:** default/min

Walks a corpus mount for markdown files and runs each through
IngestCorpusText, so the corpus can be seeded from committed
content instead of paste-by-paste. Idempotent: files whose
content_hash matches the stored row are skipped. Root is resolved
from CORPUS_ROOT (public) or CORPUS_PRIVATE_ROOT (private) plus a
subdirectory per source_kind. Synchronous; the console now runs
the same walk as a job through RunJob (JOB_KIND_CORPUS_REINDEX_*)
because a private reindex outlives the proxy's response timeout.

**Request** — [`ReindexCorpusRequest`](#reindexcorpusrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sourceKind` | `string` | string | `string: max_len: 40` | Optional kind filter (e.g. `article`, `worksheet`). Empty walks every kind in the scope. Validated against the ingest allow-list. |
| `scope` | `string` | string | `string: max_len: 16` | `public` (default) or `private`. |

**Response** — [`ReindexCorpusResponse`](#reindexcorpusresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `root` | `string` | string |  | Absolute path the handler actually walked. |
| `visibility` | `string` | string |  | Visibility stamped on every document this run: public or corpus_only. |
| `kindsWalked` | `string`[] | array of string |  | Source kinds that were walked, sorted. |
| `filesScanned` | `int32` | number |  | Number of `.md` files the walker saw. |
| `docsIngested` | `int32` | number |  | Number of files handed to the ingester without error. Includes skipped-because-unchanged. |
| `docsSkipped` | `int32` | number |  | Subset of docs_ingested whose content_hash matched the stored row and did no chunk / embed work this run. |
| `chunksInserted` | `int32` | number |  | Sum of newly-inserted chunk rows across all files this run. |
| `chunksEmbedded` | `int32` | number |  | Sum of chunks successfully embedded across all files this run. |
| `errors` | `string`[] | array of string |  | Per-file error strings from files that failed mid-walk. Capped at 20 entries so a broken directory can't produce an unbounded response. |

<details><summary>Example request body</summary>

```json
{
  "sourceKind": "string",
  "scope": "string"
}
```

</details>

### AdminService.SweepCorpusEmbeddings

`POST /api/career.v1.AdminService/SweepCorpusEmbeddings` · **Auth:** Admin · **Rate limit:** default/min

Re-embeds every chunk whose vector is missing or was produced by a
different embedder than the sidecar's current one. Bounded per call
(max_chunks); the response's `remaining` says whether to call again.
This is the safe path for flipping SIDECAR_EMBED_PROVIDER or
changing the embedding model: nothing is deleted, the corpus is
walked until every chunk carries the current model's vector.
Synchronous and bounded; the console runs the sweep to completion
as a job through RunJob (JOB_KIND_EMBED_SWEEP) with progress.

**Request** — [`SweepCorpusEmbeddingsRequest`](#sweepcorpusembeddingsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `maxChunks` | `int32` | number | `int32: lte: 5000 gte: 0` | Upper bound on chunks to (re)embed in this call. 0 → 512. |

**Response** — [`SweepCorpusEmbeddingsResponse`](#sweepcorpusembeddingsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `model` | `string` | string |  | Embedder the sidecar is currently serving; every chunk is converged onto this. |
| `considered` | `int32` | number |  | Chunks selected as stale this call. |
| `embedded` | `int32` | number |  | Chunks successfully re-embedded this call. |
| `failed` | `int32` | number |  | Chunks that failed (left stale for the next sweep). |
| `remaining` | `int32` | number |  | Chunks still stale after this call; zero means converged. |

<details><summary>Example request body</summary>

```json
{
  "maxChunks": 0
}
```

</details>

### AdminService.ListJdSubmissions

`POST /api/career.v1.AdminService/ListJdSubmissions` · **Auth:** Admin · **Rate limit:** default/min

Returns every JD submission with score + status. Backs
/admin/jd — Roger's triage view for the JD-upload flow. Full
JD body is elided from the list; the detail lookup returns it.

**Request** — [`ListJdSubmissionsRequest`](#listjdsubmissionsrequest)

_No fields; send `{}`._

**Response** — [`ListJdSubmissionsResponse`](#listjdsubmissionsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissions` | [`JdSubmissionRow`](#jdsubmissionrow)[] | array of object |  | Rows, newest first (up to 500). |
| `readyCount` | `int32` | number |  | Number of rows currently in a terminal READY state. |
| `belowThresholdCount` | `int32` | number |  | Number that scored below the résumé gate (the strong band). |
| `failedCount` | `int32` | number |  | Number that failed during scoring / generation. |
| `inFlightCount` | `int32` | number |  | Number still in flight (received / scoring / generating). |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.GetJdSubmission

`POST /api/career.v1.AdminService/GetJdSubmission` · **Auth:** Admin · **Rate limit:** default/min

Returns one JD submission in full: the JD text, both scores, the
assessment derivation (requirements, evidence, verdicts) and the
generated résumé when present. Backs /admin/jd/[id].

**Request** — [`GetJdSubmissionRequest`](#getjdsubmissionrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | Submission id (numeric, stringified). |

**Response** — [`GetJdSubmissionResponse`](#getjdsubmissionresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `row` | [`JdSubmissionRow`](#jdsubmissionrow) | object |  | The list row for this submission (status, scores, hints). |
| `jdText` | `string` | string |  | Full JD text as submitted. |
| `assessmentJson` | `string` | string |  | Assessment derivation as JSON (requirements, evidence chunk ids, per-requirement verdicts, prompt versions, score formula); empty before scoring or when the assessor was not wired. |
| `resumeMarkdown` | `string` | string |  | Generated résumé in markdown; empty until generation lands. |
| `llmModel` | `string` | string |  | Model that produced the résumé. |
| `promptId` | `string` | string |  | Prompt id that produced the résumé. |
| `promptVersion` | `int32` | number |  | Prompt version that produced the résumé. |
| `downloadUrl` | `string` | string |  | Download path for the locked PDF, including the submission's result token, when a PDF was rendered; empty otherwise. Admin-only by virtue of this RPC's auth level. |
| `runs` | [`JdRun`](#jdrun)[] | array of object |  | Every pipeline run for this submission, newest attempt first. The fields above reflect only the most recent run, because a re-score overwrites them; these rows survive it, so a superseded verdict can still be read and compared with the one that replaced it. |
| `outcome` | [`JdOutcome`](#jdoutcome) | object |  | What happened in the world after this review; unset until recorded. |
| `feedback` | [`JdFeedback`](#jdfeedback)[] | array of object |  | Judgments recorded about this posting's output, newest first. |

<details><summary>Example request body</summary>

```json
{
  "submissionId": "string"
}
```

</details>

### AdminService.RescoreJd

`POST /api/career.v1.AdminService/RescoreJd` · **Auth:** Admin · **Rate limit:** default/min

Re-runs the scoring pipeline (retrieval pre-score for diagnostics,
per-requirement assessment, score in code, résumé and locked PDF
when the fit is strong or better) for one submission in the
background, e.g. after a failed run or a prompt change. Returns
immediately; poll GetJdSubmission for the outcome.

**Request** — [`RescoreJdRequest`](#rescorejdrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | Submission id (numeric, stringified). |

**Response** — [`RescoreJdResponse`](#rescorejdresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`JdStatus`](#jdstatus) | string (enum name) |  | Status the row was set to when the run was queued (`scoring`). |

<details><summary>Example request body</summary>

```json
{
  "submissionId": "string"
}
```

</details>

### AdminService.SetJdOutcome

`POST /api/career.v1.AdminService/SetJdOutcome` · **Auth:** Admin · **Rate limit:** default/min

Records what happened in the world after a review: applied,
interview, offer, no response. One per posting, revised in place.
This is the only signal that says whether a score predicted
anything, so nothing else can substitute for it.

**Request** — [`SetJdOutcomeRequest`](#setjdoutcomerequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | Submission id (numeric, stringified). |
| `status` | `string` | string | `string: min_len: 1 max_len: 32` | One of the outcome statuses; validated server-side. |
| `decidedOn` | `string` | string | `string: max_len: 10` | YYYY-MM-DD, or empty when the day is unknown. |
| `note` | `string` | string | `string: max_len: 4000` | Free note, trimmed to 4000 characters. |

**Response** — [`SetJdOutcomeResponse`](#setjdoutcomeresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `outcome` | [`JdOutcome`](#jdoutcome) | object |  | The stored outcome. |

<details><summary>Example request body</summary>

```json
{
  "submissionId": "string",
  "status": "string",
  "decidedOn": "string",
  "note": "string"
}
```

</details>

### AdminService.RecordJdFeedback

`POST /api/career.v1.AdminService/RecordJdFeedback` · **Auth:** Admin · **Rate limit:** default/min

Records the owner's judgment of one run's output: whether the score
was accurate, too generous or too harsh, and whether the résumé is
sendable. Attached to the run, so a later re-score does not inherit
an opinion of the thing it replaced.

**Request** — [`RecordJdFeedbackRequest`](#recordjdfeedbackrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | Submission id (numeric, stringified). |
| `runId` | `string` | string | `string: max_len: 64` | Run being judged; empty attaches the judgment to the newest run. |
| `target` | `string` | string | `string: min_len: 1 max_len: 16` | score | resume. |
| `rating` | `string` | string | `string: min_len: 1 max_len: 32` | Rating from the vocabulary for that target; validated server-side. |
| `note` | `string` | string | `string: max_len: 4000` | Free note, trimmed to 4000 characters. |

**Response** — [`RecordJdFeedbackResponse`](#recordjdfeedbackresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "submissionId": "string",
  "runId": "string",
  "target": "string",
  "rating": "string",
  "note": "string"
}
```

</details>

### AdminService.ListGoldenPostings

`POST /api/career.v1.AdminService/ListGoldenPostings` · **Auth:** Admin · **Rate limit:** default/min

Lists the golden set: fixed postings with a stated expectation,
re-scored to measure whether a prompt or model change helped.

**Request** — [`ListGoldenPostingsRequest`](#listgoldenpostingsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `activeOnly` | `bool` | boolean |  | True omits retired postings. |

**Response** — [`ListGoldenPostingsResponse`](#listgoldenpostingsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `postings` | [`GoldenPosting`](#goldenposting)[] | array of object |  | The set, expected-above first then by name. |

<details><summary>Example request body</summary>

```json
{
  "activeOnly": true
}
```

</details>

### AdminService.UpsertGoldenPosting

`POST /api/career.v1.AdminService/UpsertGoldenPosting` · **Auth:** Admin · **Rate limit:** default/min

Adds or replaces a golden posting, keyed by name.

**Request** — [`UpsertGoldenPostingRequest`](#upsertgoldenpostingrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string | `string: min_len: 1 max_len: 80` | Short unique label. |
| `jdText` | `string` | string | `string: min_len: 40 max_len: 50000` | The posting text. |
| `roleHint` | `string` | string | `string: max_len: 200` | Role hint. |
| `employerHint` | `string` | string | `string: max_len: 200` | Employer hint. |
| `expectedGate` | `string` | string | `string: max_len: 8` | above, below, or empty to add it unlabelled for review later. |
| `selection` | `string` | string | `string: max_len: 16` | chosen or random; defaults to chosen. |
| `source` | `string` | string | `string: max_len: 500` | Where the posting came from. |
| `expectedBand` | `string` | string | `string: max_len: 20` | Advisory band. |
| `note` | `string` | string | `string: max_len: 2000` | Why it is in the set. |

**Response** — [`UpsertGoldenPostingResponse`](#upsertgoldenpostingresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id of the created or replaced posting. |

<details><summary>Example request body</summary>

```json
{
  "name": "string",
  "jdText": "string",
  "roleHint": "string",
  "employerHint": "string",
  "expectedGate": "string",
  "selection": "string",
  "source": "string",
  "expectedBand": "string",
  "note": "string"
}
```

</details>

### AdminService.SetGoldenActive

`POST /api/career.v1.AdminService/SetGoldenActive` · **Auth:** Admin · **Rate limit:** default/min

Retires or restores a golden posting. Retired postings are kept,
because deleting one would silently change what every past
evaluation was measuring.

**Request** — [`SetGoldenActiveRequest`](#setgoldenactiverequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id. |
| `active` | `bool` | boolean |  | False retires it. |

**Response** — [`SetGoldenActiveResponse`](#setgoldenactiveresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "0",
  "active": true
}
```

</details>

### AdminService.LabelGoldenPosting

`POST /api/career.v1.AdminService/LabelGoldenPosting` · **Auth:** Admin · **Rate limit:** default/min

Records which side of the gate a posting belongs on. Separate from
the upsert so labelling a posting does not mean resending its text,
and so an unlabelled posting is a state the console can act on.

**Request** — [`LabelGoldenPostingRequest`](#labelgoldenpostingrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id. |
| `expectedGate` | `string` | string | `string: min_len: 1 max_len: 8` | above or below. |
| `note` | `string` | string | `string: max_len: 2000` | Why, in a sentence. Appended to the posting's note. |

**Response** — [`LabelGoldenPostingResponse`](#labelgoldenpostingresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "0",
  "expectedGate": "string",
  "note": "string"
}
```

</details>

### AdminService.ListEvalRuns

`POST /api/career.v1.AdminService/ListEvalRuns` · **Auth:** Admin · **Rate limit:** default/min

Lists evaluations, newest first, without their per-posting results.

**Request** — [`ListEvalRunsRequest`](#listevalrunsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number | `int32: lte: 100 gte: 0` | Maximum rows, default 25. |

**Response** — [`ListEvalRunsResponse`](#listevalrunsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `runs` | [`EvalRun`](#evalrun)[] | array of object |  | Evaluations, newest first. |

<details><summary>Example request body</summary>

```json
{
  "limit": 0
}
```

</details>

### AdminService.GetEvalRun

`POST /api/career.v1.AdminService/GetEvalRun` · **Auth:** Admin · **Rate limit:** default/min

Returns one evaluation with every posting's result.

**Request** — [`GetEvalRunRequest`](#getevalrunrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id. |

**Response** — [`GetEvalRunResponse`](#getevalrunresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `run` | [`EvalRun`](#evalrun) | object |  | The evaluation, with its per-posting results. |

<details><summary>Example request body</summary>

```json
{
  "id": "0"
}
```

</details>

### AdminService.GetMetrics

`POST /api/career.v1.AdminService/GetMetrics` · **Auth:** Admin · **Rate limit:** default/min

Returns the state of the reviewer, read from the SQL views that
define each metric once. Nothing here is computed in the api or in
a page: two definitions of the same number is how a dashboard
starts disagreeing with itself.

**Request** — [`GetMetricsRequest`](#getmetricsrequest)

_No fields; send `{}`._

**Response** — [`GetMetricsResponse`](#getmetricsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `recentRuns` | `int32` | number |  | Real runs in the last twenty. |
| `recentCompleted` | `int32` | number |  | Of those, how many finished without intervention. |
| `recentFailed` | `int32` | number |  | Of those, how many failed. |
| `recentStuck` | `int32` | number |  | Of those, how many are still claiming to run. |
| `reviewed` | `int32` | number |  | Requirement verdicts the owner has reviewed. |
| `agreed` | `int32` | number |  | Of those, how many the owner agreed with. |
| `agreementPct` | `double` | number |  | _(oneof `_agreement_pct`)_ Agreement as a percentage of the gradeable rows; unset when none. |
| `softDisagreements` | `int32` | number |  | Disagreements where one side said partial: the judge being unsure. |
| `hardDisagreements` | `int32` | number |  | Disagreements between met and unmet: the judge being wrong. |
| `gradeable` | `int32` | number |  | Reviewed rows the owner could actually judge. |
| `ungradeable` | `int32` | number |  | Rows the owner could not judge from what they were shown. Excluded from the agreement rate, and the most actionable number here: a pile of them means retrieval is failing, which no prompt fixes. |
| `tooHarsh` | `int32` | number |  | Disagreements where the model said unmet and the owner did not. |
| `tooGenerous` | `int32` | number |  | Disagreements where the model credited more than the owner did. |
| `finishedRuns` | `int32` | number |  | Runs that reached a result. |
| `medianMinutes` | `double` | number |  | _(oneof `_median_minutes`)_ Median minutes to a result. |
| `p95Minutes` | `double` | number |  | _(oneof `_p95_minutes`)_ 95th percentile minutes to a result. |
| `medianQueuedMinutes` | `double` | number |  | _(oneof `_median_queued_minutes`)_ Median minutes spent queued, reported apart from work. |
| `landed` | `int32` | number |  | Thirty-day funnel: visitors who reached the landing page. |
| `readWriting` | `int32` | number |  | Visitors who read an article. |
| `clicked` | `int32` | number |  | Visitors who clicked a call to action. |
| `registered` | `int32` | number |  | Visitors who submitted a registration. |
| `verified` | `int32` | number |  | Visitors who verified their email. |
| `signedIn` | `int32` | number |  | Visitors who signed in. |
| `submitted` | `int32` | number |  | Visitors who submitted a posting. |
| `calls` | `int32` | number |  | Model calls in the last 30 days. |
| `callFailures` | `int32` | number |  | Of those, how many failed. |
| `promptTokens` | `int64` | string (decimal) |  | Prompt tokens in the last 30 days. |
| `completionTokens` | `int64` | string (decimal) |  | Completion tokens in the last 30 days. |
| `latestEval` | [`EvalRun`](#evalrun) | object |  | Newest finished evaluation; unset before the first one. |
| `outcomes` | [`OutcomeByFit`](#outcomebyfit)[] | array of object |  | Fit band against recorded outcomes. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.GetOpsStatus

`POST /api/career.v1.AdminService/GetOpsStatus` · **Auth:** Admin · **Rate limit:** default/min

Returns what the box is doing right now: jobs the runner knows
about, the submission pipeline, recent model activity and host
load. Backs /admin/ops.

**Request** — [`GetOpsStatusRequest`](#getopsstatusrequest)

_No fields; send `{}`._

**Response** — [`GetOpsStatusResponse`](#getopsstatusresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobs` | [`JobRow`](#jobrow)[] | array of object |  | Jobs the runner still remembers, newest first. Finished jobs are forgotten after a day. |
| `busy` | `bool` | boolean |  | True when any job is running or queued, which is the one thing to check before deploying. |
| `pipeline` | [`PipelineCount`](#pipelinecount)[] | array of object |  | Submissions by pipeline status. |
| `latestEval` | [`EvalRun`](#evalrun) | object |  | The most recent evaluation, running or not. |
| `callsLastHour` | `int32` | number |  | Model calls in the last hour. |
| `callFailuresLastHour` | `int32` | number |  | Of those, how many failed. |
| `avgCallSeconds` | `double` | number |  | Average seconds per call in the last hour. |
| `load1` | `double` | number |  | Host load average over one minute, as the kernel reports it. |
| `load5` | `double` | number |  | Over five minutes. |
| `memTotalMb` | `int32` | number |  | Total host memory in megabytes. |
| `memAvailableMb` | `int32` | number |  | Host memory available in megabytes. |
| `diskFreeGb` | `int32` | number |  | Free disk in gigabytes on the filesystem the api is running from. |
| `diskTotalGb` | `int32` | number |  | Total disk in gigabytes on that filesystem. |
| `warnings` | `string`[] | array of string |  | Anything this snapshot could not read. A section that failed and a section that is genuinely empty look identical otherwise, and on 2026-09-25 that difference hid a broken query behind the words "no submissions" for as long as nobody read the logs. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.GetJobDetail

`POST /api/career.v1.AdminService/GetJobDetail` · **Auth:** Admin · **Rate limit:** default/min

Returns one job with every progress report it made, for the detail
view on /admin/ops. Separate from GetOpsStatus because that call is
polled: carrying a few hundred events per job in the list payload
would make the page heavier the longer a run goes on, which is
backwards for a page you open because something is running.

**Request** — [`GetJobDetailRequest`](#getjobdetailrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string | `string: min_len: 1 max_len: 64` | The runner id, as GetOpsStatus reports it. |

**Response** — [`GetJobDetailResponse`](#getjobdetailresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `job` | [`JobRow`](#jobrow) | object |  | The job itself, the same row GetOpsStatus lists. |
| `events` | [`JobEvent`](#jobevent)[] | array of object |  | Every progress report, oldest first, capped by the runner. |
| `truncated` | `bool` | boolean |  | True when the runner dropped older events at its cap, so a reader knows the timeline starts mid-run rather than at the beginning. |

<details><summary>Example request body</summary>

```json
{
  "jobId": "string"
}
```

</details>

### AdminService.GetGate

`POST /api/career.v1.AdminService/GetGate` · **Auth:** Admin · **Rate limit:** default/min

Returns the criteria as a gate: one row per criterion with pass,
value, target and as_of, read from the views that define them.
/admin/analytics shows the same numbers arranged for reading; this
answers whether each one is met.

**Request** — [`GetGateRequest`](#getgaterequest)

_No fields; send `{}`._

**Response** — [`GetGateResponse`](#getgateresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `rows` | [`GateRow`](#gaterow)[] | array of object |  | One row per criterion. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.ListDecisionLog

`POST /api/career.v1.AdminService/ListDecisionLog` · **Auth:** Admin · **Rate limit:** default/min

Lists logged reviewer decisions (per-requirement verdicts, gate
outcomes) with the evidence each was made from, for the owner's
human-in-the-loop review. Backs /admin/decisions.

**Request** — [`ListDecisionLogRequest`](#listdecisionlogrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | `string` | string |  | Restrict to one kind; empty for all kinds. |
| `refId` | `string` | string |  | Restrict to one ref id (e.g. a JD submission); empty for all. |
| `unreviewedOnly` | `bool` | boolean |  | Only rows the owner has not reviewed yet. |
| `limit` | `int32` | number |  | Page size, newest first; server caps at 500 (default 200). |
| `failedOnly` | `bool` | boolean |  | Only calls that produced no usable verdict. They are rare by construction and are the rows most worth reading, so finding them should not mean scrolling past two hundred successes. |

**Response** — [`ListDecisionLogResponse`](#listdecisionlogresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `decisions` | [`DecisionLogRow`](#decisionlogrow)[] | array of object |  | Rows, newest first. |
| `totalCount` | `int64` | string (decimal) |  | Rows in the whole table, any kind. |
| `reviewedCount` | `int64` | string (decimal) |  | Rows the owner has reviewed. |

<details><summary>Example request body</summary>

```json
{
  "kind": "string",
  "refId": "string",
  "unreviewedOnly": true,
  "limit": 0,
  "failedOnly": true
}
```

</details>

### AdminService.ReviewDecision

`POST /api/career.v1.AdminService/ReviewDecision` · **Auth:** Admin · **Rate limit:** default/min

Records the owner's own verdict and note on one logged decision.
Saving again overwrites the label; the model's output is never
changed.

**Request** — [`ReviewDecisionRequest`](#reviewdecisionrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Row id to label. |
| `humanVerdict` | `string` | string |  | The owner's verdict; the vocabulary of the row's output (met / partial / unmet for requirement verdicts). |
| `humanNote` | `string` | string |  | Optional note explaining the label. Read by a human; never used as a training target. A rewrite goes in human_answer. |
| `humanAnswer` | `string` | string |  | The wording the owner would have given, for a decision whose output is prose. Optional, and the single most valuable field on this message: a graded answer with no correction can be counted, while a graded answer with one can be learned from. |
| `humanDimensions` | map<`string`, `string`> | object of string |  | Per-rubric grades for kinds that have a rubric (chat answers: grounded, citations, voice, scope, length). Each value is yes, partial, no or n/a. Unknown keys and values are rejected rather than stored, because a grade nobody can interpret still counts in a rate. |

**Response** — [`ReviewDecisionResponse`](#reviewdecisionresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "string",
  "humanVerdict": "string",
  "humanNote": "string",
  "humanAnswer": "string",
  "humanDimensions": {
    "key": "string"
  }
}
```

</details>

### AdminService.ExportDecisionLog

`POST /api/career.v1.AdminService/ExportDecisionLog` · **Auth:** Admin · **Rate limit:** default/min

Exports decisions as JSON Lines for adapter training and
evaluation; reviewed rows carry the human label.

**Request** — [`ExportDecisionLogRequest`](#exportdecisionlogrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `reviewedOnly` | `bool` | boolean |  | Only rows the owner has reviewed (the training set); false exports everything. |

**Response** — [`ExportDecisionLogResponse`](#exportdecisionlogresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jsonl` | `string` | string |  | JSON Lines, one decision per line, oldest first. |
| `rowCount` | `int32` | number |  | Number of lines in jsonl. |

<details><summary>Example request body</summary>

```json
{
  "reviewedOnly": true
}
```

</details>

### AdminService.GetJdFitBands

`POST /api/career.v1.AdminService/GetJdFitBands` · **Auth:** Admin · **Rate limit:** default/min

Reads the JD fit bands (the numbers that classify a review as very
strong / strong / possible / weak / very weak; "strong" is the gate).

**Request** — [`GetJdFitBandsRequest`](#getjdfitbandsrequest)

_No fields; send `{}`._

**Response** — [`GetJdFitBandsResponse`](#getjdfitbandsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `bands` | [`JdFitBands`](#jdfitbands) | object |  | Current bands. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.SetJdFitBands

`POST /api/career.v1.AdminService/SetJdFitBands` · **Auth:** Admin · **Rate limit:** default/min

Sets the JD fit bands (stored in app_settings; the api caches them
for 15 s). Takes effect for the next submission within seconds;
existing scores are re-classified on read.

**Request** — [`SetJdFitBandsRequest`](#setjdfitbandsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `bands` | [`JdFitBands`](#jdfitbands) | object |  | The bands to store. |

**Response** — [`SetJdFitBandsResponse`](#setjdfitbandsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `bands` | [`JdFitBands`](#jdfitbands) | object |  | Stored bands. |

<details><summary>Example request body</summary>

```json
{
  "bands": {
    "veryStrong": 0.5,
    "strong": 0.5,
    "possible": 0.5,
    "weak": 0.5
  }
}
```

</details>

### AdminService.GetDecisionTestSettings

`POST /api/career.v1.AdminService/GetDecisionTestSettings` · **Auth:** Admin · **Rate limit:** default/min

Reads the decision test's timings.

**Request** — [`GetDecisionTestSettingsRequest`](#getdecisiontestsettingsrequest)

_No fields; send `{}`._

**Response** — [`GetDecisionTestSettingsResponse`](#getdecisiontestsettingsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memoriseMs` | `int32` | number |  | How long the number to hold is shown, in milliseconds. |
| `questionMs` | `int32` | number |  | Hard limit per question, covering reading, deciding and rating. |
| `recallMs` | `int32` | number |  | Limit on entering the number at the end of a block. |
| `instrumentVersion` | `string` | string |  | Derived from the three above, and stored on every session taken under them. Shown so the owner can see that changing a timing changes the instrument. |
| `sessionsOnThisVersion` | `int32` | number |  | How many sessions have already been recorded under this version. Changing a timing starts a new one, which is the cost of the change. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.SetDecisionTestSettings

`POST /api/career.v1.AdminService/SetDecisionTestSettings` · **Auth:** Admin · **Rate limit:** default/min

Sets the decision test's timings. The instrument version is derived
from them, so a change here is recorded on every run taken after it
and runs under different timings never pool into one dataset.

**Request** — [`SetDecisionTestSettingsRequest`](#setdecisiontestsettingsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memoriseMs` | `int32` | number | `int32: gte: 0` | How long the number to hold is shown. Clamped to 1 to 30 seconds. |
| `questionMs` | `int32` | number | `int32: gte: 0` | Hard limit per question. Clamped to 5 to 120 seconds, because a question nobody can answer in time is not a harder test, it is an unanswerable one. |
| `recallMs` | `int32` | number | `int32: gte: 0` | Limit on entering the number. Clamped to 5 to 120 seconds. |

**Response** — [`SetDecisionTestSettingsResponse`](#setdecisiontestsettingsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `instrumentVersion` | `string` | string |  | The instrument version now in force. |

<details><summary>Example request body</summary>

```json
{
  "memoriseMs": 0,
  "questionMs": 0,
  "recallMs": 0
}
```

</details>

### AdminService.ListDecisionTestRuns

`POST /api/career.v1.AdminService/ListDecisionTestRuns` · **Auth:** Admin · **Rate limit:** default/min

Lists decision test runs, newest first.

**Request** — [`ListDecisionTestRunsRequest`](#listdecisiontestrunsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includeSynthetic` | `bool` | boolean |  | Include runs driven by an agent rather than taken by a person. Excluded by default, because they are not data. |
| `reviewStatus` | `string` | string | `string: max_len: 24` | Only runs with this review status. Empty returns everything; "unreviewed" returns the queue, which is the common case when the point of opening the page is to work through it. |

**Response** — [`ListDecisionTestRunsResponse`](#listdecisiontestrunsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `counts` | [`DecisionTestReviewCounts`](#decisiontestreviewcounts) | object |  | The queue, counted across every real run regardless of the filter, so the page can say how much is left without a second call. |
| `runs` | [`DecisionTestRun`](#decisiontestrun)[] | array of object |  | The runs. |
| `itemPools` | [`DecisionTestItemPool`](#decisiontestitempool)[] | array of object |  | Each category's room to vary. On the list rather than tucked away, because a pool that cannot vary is the one fault here that looks exactly like success: the draw runs, every block is balanced, and every participant sits the same test. |

<details><summary>Example request body</summary>

```json
{
  "includeSynthetic": true,
  "reviewStatus": "string"
}
```

</details>

### AdminService.GetDecisionTestRun

`POST /api/career.v1.AdminService/GetDecisionTestRun` · **Auth:** Admin · **Rate limit:** default/min

One run in full: every answer, every recall, and the block summary.

**Request** — [`GetDecisionTestRunRequest`](#getdecisiontestrunrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From the list. |

**Response** — [`GetDecisionTestRunResponse`](#getdecisiontestrunresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `run` | [`DecisionTestRun`](#decisiontestrun) | object |  | The run's summary. |
| `blocks` | [`DecisionTestBlock`](#decisiontestblock)[] | array of object |  | One row per block, in running order. |
| `answers` | [`DecisionTestAnswer`](#decisiontestanswer)[] | array of object |  | Every answer, in presentation order. |
| `reviewHistory` | [`DecisionTestReviewEvent`](#decisiontestreviewevent)[] | array of object |  | Every judgement ever recorded for this run, oldest first. |
| `drawnItems` | [`DecisionTestDrawnItem`](#decisiontestdrawnitem)[] | array of object |  | The thirty questions this run was drawn, in order, including any it never reached. |

<details><summary>Example request body</summary>

```json
{
  "sessionKey": "string"
}
```

</details>

### AdminService.ExportDecisionTestData

`POST /api/career.v1.AdminService/ExportDecisionTestData` · **Auth:** Admin · **Rate limit:** default/min

The curated dataset as CSV, one row per question presented.

The dataset is the deliverable (FSD §7b), and a deliverable that
can only be read through this console is not one. This is the
handoff to whatever does the actual analysis.

Served from v_dt_answers, so the export and the console cannot
disagree about what a number means, and so the things that view
deliberately omits stay omitted: no name, no email address, and no
chosen_index, because the raw index across enough runs would let
somebody reconstruct the answer key.

**Request** — [`ExportDecisionTestDataRequest`](#exportdecisiontestdatarequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includeSynthetic` | `bool` | boolean |  | Include runs driven by an agent rather than a person. Off by default: a synthetic run is useful for checking the instrument and ruinous if it is averaged into a claim about people, so including it has to be asked for. |
| `includeExcluded` | `bool` | boolean |  | Include runs and blocks the owner marked do_not_use. Off by default, because curation that does not reach the export is decoration: the export is the thing that leaves for analysis, and shipping a run marked unusable is worse than not curating at all, since the mark creates a belief the data has been cleaned. |

**Response** — [`ExportDecisionTestDataResponse`](#exportdecisiontestdataresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `csv` | `string` | string |  | The whole file, RFC 4180, header row first. Thirty rows per completed run, so this stays small enough to hand over in one response for as long as the volunteer count is realistic. |
| `filename` | `string` | string |  | Suggested filename, carrying the date so two exports do not overwrite each other in a downloads folder. |
| `rows` | `int32` | number |  | Rows in the body, not counting the header. |
| `sessions` | `int32` | number |  | Runs those rows came from. |

<details><summary>Example request body</summary>

```json
{
  "includeSynthetic": true,
  "includeExcluded": true
}
```

</details>

### AdminService.GetDecisionTestAnalysis

`POST /api/career.v1.AdminService/GetDecisionTestAnalysis` · **Auth:** Admin · **Rate limit:** default/min

Reads the decision test's three analysis views.

The load curve, the calibration curve and the per-person
threshold. All three have existed since migration 00054 and were
reachable only through the SQL console, which put the finding this
instrument exists to produce one hand-written query away from
anybody wanting to look at it.

**Request** — [`GetDecisionTestAnalysisRequest`](#getdecisiontestanalysisrequest)

_No fields; send `{}`._

**Response** — [`GetDecisionTestAnalysisResponse`](#getdecisiontestanalysisresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `loadCurve` | [`DecisionTestLoadPoint`](#decisiontestloadpoint)[] | array of object |  | Accuracy and confidence by load level. |
| `calibration` | [`DecisionTestCalibrationPoint`](#decisiontestcalibrationpoint)[] | array of object |  | What each confidence band actually achieved. |
| `thresholds` | [`DecisionTestThreshold`](#decisiontestthreshold)[] | array of object |  | Per participant, the lowest load at which they were confidently wrong. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.ExportDecisionTestRun

`POST /api/career.v1.AdminService/ExportDecisionTestRun` · **Auth:** Admin · **Rate limit:** default/min

Downloads one run's blocks and answers as two CSV files.

Separate from ExportDecisionTestData, which is the whole curated
dataset for analysis and is served from v_dt_answers. This is one
run, for a reviewer who is looking at that run: it is scoped to a
single session, it is built from exactly what the run page was
given so the file and the screen cannot disagree, and it can carry
the per-answer detail the dataset export deliberately omits,
behind an explicit opt-in.

**Request** — [`ExportDecisionTestRunRequest`](#exportdecisiontestrunrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | Which run, from the list. |
| `includeKey` | `bool` | boolean |  | Add the prompt, the chosen option and the correct option to the answers file. Off by default, matching the collapsed section on the run page. An answer key that escapes contaminates a standardized instrument permanently, and a file is easier to forward than a screen. The whole-dataset export never carries these at all (FR-DT-16); this is one run, downloaded deliberately. |

**Response** — [`ExportDecisionTestRunResponse`](#exportdecisiontestrunresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `filenameStem` | `string` | string |  | Base filename, without an extension or a suffix. |
| `blocksCsv` | `string` | string |  | The by-block table, one row per block, with the session context on every row so the file stands alone. |
| `answersCsv` | `string` | string |  | The every-answer table, one row per answer, same context. |
| `blockRows` | `int32` | number |  | Rows in the blocks file, excluding its header. |
| `answerRows` | `int32` | number |  | Rows in the answers file, excluding its header. |

<details><summary>Example request body</summary>

```json
{
  "sessionKey": "string",
  "includeKey": true
}
```

</details>

### AdminService.ReviewDecisionTestRun

`POST /api/career.v1.AdminService/ReviewDecisionTestRun` · **Auth:** Admin · **Rate limit:** default/min

Records the owner's judgement about a run, or about one block of
one, and returns the run as it now reads.

Curation is what turns collection into a dataset. Only
`do_not_use` excludes anything; the other statuses describe a run
without changing what the analysis sees, because whether an
interrupted run is usable is a judgement made per run rather than
a rule. Nothing is ever deleted: an excluded run keeps every row
it produced.

**Request** — [`ReviewDecisionTestRunRequest`](#reviewdecisiontestrunrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | Which run. |
| `blockNo` | `int32` | number | `int32: lte: 5 gte: 0` | 0 judges the whole run; 1 to 5 judges one block, so a single spoiled block does not cost the other four. |
| `status` | `string` | string | `string: max_len: 24` | good, incomplete, hold or do_not_use. Empty clears a run's review back to unreviewed, and for a block removes the judgement altogether, since a block has no stored unreviewed state. |
| `reason` | `string` | string | `string: max_len: 32` | instrument_fault, participant_reported, duplicate or other, and only alongside do_not_use. The first two are the split that earns this field: one is a bug to go and repair, the other is nothing to fix and simply costs a data point. |
| `note` | `string` | string | `string: max_len: 2000` | Free text, for what the participant said or what was observed. |

**Response** — [`ReviewDecisionTestRunResponse`](#reviewdecisiontestrunresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `run` | [`DecisionTestRun`](#decisiontestrun) | object |  | The updated summary, so the caller does not have to refetch to render the new state. |

<details><summary>Example request body</summary>

```json
{
  "sessionKey": "string",
  "blockNo": 0,
  "status": "string",
  "reason": "string",
  "note": "string"
}
```

</details>

### AdminService.GetSchedulerSettings

`POST /api/career.v1.AdminService/GetSchedulerSettings` · **Auth:** Admin · **Rate limit:** default/min

Reads the meeting-scheduler settings: the weekly windows a member
may book into, the lengths on offer, the clearance between
meetings, and the zone all of it is quoted in.

**Request** — [`GetSchedulerSettingsRequest`](#getschedulersettingsrequest)

_No fields; send `{}`._

**Response** — [`GetSchedulerSettingsResponse`](#getschedulersettingsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `settings` | [`SchedulerSettings`](#schedulersettings) | object |  | Current settings. |
| `calendarConnected` | `bool` | boolean |  | False when the calendar provider is not connected or unreachable, so the surface can say booking is off rather than implying these windows are live. |
| `calendarStatus` | `string` | string |  | Why the calendar is not usable, when calendar_connected is false. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.SetSchedulerSettings

`POST /api/career.v1.AdminService/SetSchedulerSettings` · **Auth:** Admin · **Rate limit:** default/min

Replaces the meeting-scheduler settings (stored in app_settings;
the api caches them for 15 s). Validated server-side and refused
whole rather than in part, because a half-applied calendar is
worse than an unchanged one. Existing bookings are never moved:
narrowing the windows stops new bookings, it does not cancel.

**Request** — [`SetSchedulerSettingsRequest`](#setschedulersettingsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `settings` | [`SchedulerSettings`](#schedulersettings) | object |  | The settings to store. |

**Response** — [`SetSchedulerSettingsResponse`](#setschedulersettingsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `settings` | [`SchedulerSettings`](#schedulersettings) | object |  | Stored settings. |

<details><summary>Example request body</summary>

```json
{
  "settings": {
    "zone": "string",
    "durationMinutes": [
      0
    ],
    "gapMinutes": 0,
    "stepMinutes": 0,
    "maxPerDay": 0,
    "leadHours": 0,
    "horizonDays": 0,
    "windows": [
      {
        "weekday": 0,
        "startMinutes": 0,
        "endMinutes": 0
      }
    ]
  }
}
```

</details>

### AdminService.ListQaEntries

`POST /api/career.v1.AdminService/ListQaEntries` · **Auth:** Admin · **Rate limit:** default/min

Lists the Q&A bank: the owner's own answers, served verbatim by
Ask Roger with no model involved.

**Request** — [`ListQaEntriesRequest`](#listqaentriesrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includeDisabled` | `bool` | boolean |  | Include entries that are not enabled. False for anything member-facing. |

**Response** — [`ListQaEntriesResponse`](#listqaentriesresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `entries` | [`QaEntry`](#qaentry)[] | array of object |  | Entries. |
| `awaitingEmbedding` | `int32` | number |  | Phrasings still waiting for an embedding across the whole bank. A non-zero count means part of the bank cannot match yet. |

<details><summary>Example request body</summary>

```json
{
  "includeDisabled": true
}
```

</details>

### AdminService.CreateQaEntry

`POST /api/career.v1.AdminService/CreateQaEntry` · **Auth:** Admin · **Rate limit:** default/min

Writes a new bank entry along with its canonical phrasing. The
entry is unreachable until the embedding job has given its
phrasings vectors, which happens within five minutes.

**Request** — [`CreateQaEntryRequest`](#createqaentryrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `question` | `string` | string | `string: min_len: 1 max_len: 500` | The question. |
| `answer` | `string` | string | `string: min_len: 1 max_len: 8000` | The answer, served verbatim. |
| `sources` | [`QaSource`](#qasource)[] | array of object |  | Sources to show beneath it. |
| `tags` | `string`[] | array of string |  | Tags. |
| `coversRestricted` | `bool` | boolean |  | Whether this deliberately answers an otherwise-refused topic. |
| `enabled` | `bool` | boolean |  | Whether to approve it now. |
| `phrasings` | `string`[] | array of string |  | Extra ways of asking it; the question itself is added automatically. |

**Response** — [`CreateQaEntryResponse`](#createqaentryresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Entry id (numeric, stringified). |

<details><summary>Example request body</summary>

```json
{
  "question": "string",
  "answer": "string",
  "sources": [
    {
      "title": "string",
      "path": "string"
    }
  ],
  "tags": [
    "string"
  ],
  "coversRestricted": true,
  "enabled": true,
  "phrasings": [
    "string"
  ]
}
```

</details>

### AdminService.UpdateQaEntry

`POST /api/career.v1.AdminService/UpdateQaEntry` · **Auth:** Admin · **Rate limit:** default/min

Replaces an entry's editable fields. Changing the question clears
the canonical phrasing's vector, so the entry stops matching the
old wording immediately and matches the new one once re-embedded.

**Request** — [`UpdateQaEntryRequest`](#updateqaentryrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Entry to change. |
| `question` | `string` | string | `string: min_len: 1 max_len: 500` | The question. Changing it clears the canonical phrasing's vector. |
| `answer` | `string` | string | `string: min_len: 1 max_len: 8000` | The answer. |
| `sources` | [`QaSource`](#qasource)[] | array of object |  | Sources. |
| `tags` | `string`[] | array of string |  | Tags. |
| `coversRestricted` | `bool` | boolean |  | Whether this deliberately answers an otherwise-refused topic. |
| `enabled` | `bool` | boolean |  | Whether it is approved. |

**Response** — [`UpdateQaEntryResponse`](#updateqaentryresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "string",
  "question": "string",
  "answer": "string",
  "sources": [
    {
      "title": "string",
      "path": "string"
    }
  ],
  "tags": [
    "string"
  ],
  "coversRestricted": true,
  "enabled": true
}
```

</details>

### AdminService.SetQaEntryEnabled

`POST /api/career.v1.AdminService/SetQaEntryEnabled` · **Auth:** Admin · **Rate limit:** default/min

Approves or withdraws an entry. Enabling is the approval: a
disabled entry is never matched and never served, which is how a
statement is taken back without losing what it said.

**Request** — [`SetQaEntryEnabledRequest`](#setqaentryenabledrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Entry to approve or withdraw. |
| `enabled` | `bool` | boolean |  | True approves it. |

**Response** — [`SetQaEntryEnabledResponse`](#setqaentryenabledresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "string",
  "enabled": true
}
```

</details>

### AdminService.DeleteQaEntry

`POST /api/career.v1.AdminService/DeleteQaEntry` · **Auth:** Admin · **Rate limit:** default/min

Deletes an entry and its phrasings.

**Request** — [`DeleteQaEntryRequest`](#deleteqaentryrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Entry to delete. |

**Response** — [`DeleteQaEntryResponse`](#deleteqaentryresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "string"
}
```

</details>

### AdminService.AddQaPhrasing

`POST /api/career.v1.AdminService/AddQaPhrasing` · **Auth:** Admin · **Rate limit:** default/min

Adds another way of asking an existing entry's question. Matching
runs over every phrasing, so variants are how one answer covers the
several ways people ask for it.

**Request** — [`AddQaPhrasingRequest`](#addqaphrasingrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `entryId` | `string` | string | `string: min_len: 1 max_len: 64` | Entry it belongs to. |
| `text` | `string` | string | `string: min_len: 1 max_len: 500` | The wording. |

**Response** — [`AddQaPhrasingResponse`](#addqaphrasingresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Phrasing id (numeric, stringified). |

<details><summary>Example request body</summary>

```json
{
  "entryId": "string",
  "text": "string"
}
```

</details>

### AdminService.DeleteQaPhrasing

`POST /api/career.v1.AdminService/DeleteQaPhrasing` · **Auth:** Admin · **Rate limit:** default/min

Removes one variant phrasing. The canonical phrasing cannot be
removed: it is the entry's own question.

**Request** — [`DeleteQaPhrasingRequest`](#deleteqaphrasingrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `entryId` | `string` | string | `string: min_len: 1 max_len: 64` | Entry it belongs to. |
| `phrasingId` | `string` | string | `string: min_len: 1 max_len: 64` | Phrasing to remove. The canonical one cannot be removed. |

**Response** — [`DeleteQaPhrasingResponse`](#deleteqaphrasingresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "entryId": "string",
  "phrasingId": "string"
}
```

</details>

### AdminService.GetCalendarConnectURL

`POST /api/career.v1.AdminService/GetCalendarConnectURL` · **Auth:** Admin · **Rate limit:** default/min

Returns the Google consent URL the owner visits to connect his
calendar, carrying a signed, short-lived state so the callback
cannot be driven by anyone else.

**Request** — [`GetCalendarConnectURLRequest`](#getcalendarconnecturlrequest)

_No fields; send `{}`._

**Response** — [`GetCalendarConnectURLResponse`](#getcalendarconnecturlresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `url` | `string` | string |  | The full consent URL, including the signed state. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.ConnectCalendar

`POST /api/career.v1.AdminService/ConnectCalendar` · **Auth:** Admin · **Rate limit:** default/min

Completes the handshake: exchanges the authorisation code for a
refresh token and stores it encrypted at rest. The code is
single-use and the state is verified before anything is stored.

**Request** — [`ConnectCalendarRequest`](#connectcalendarrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `code` | `string` | string | `string: min_len: 1` | The one-time code from the callback's query string. |
| `state` | `string` | string | `string: min_len: 1` | The state from the callback, verified against the one issued. |

**Response** — [`ConnectCalendarResponse`](#connectcalendarresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`CalendarStatus`](#calendarstatus) | object |  | Current status after connecting. |

<details><summary>Example request body</summary>

```json
{
  "code": "string",
  "state": "string"
}
```

</details>

### AdminService.GetCalendarStatus

`POST /api/career.v1.AdminService/GetCalendarStatus` · **Auth:** Admin · **Rate limit:** default/min

Reads the calendar connection: which account, when it was
connected, and whether it is currently working.

**Request** — [`GetCalendarStatusRequest`](#getcalendarstatusrequest)

_No fields; send `{}`._

**Response** — [`GetCalendarStatusResponse`](#getcalendarstatusresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`CalendarStatus`](#calendarstatus) | object |  | Current status. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.DisconnectCalendar

`POST /api/career.v1.AdminService/DisconnectCalendar` · **Auth:** Admin · **Rate limit:** default/min

Forgets the stored credential. Booking stops immediately; existing
meetings are left alone, on the calendar and in this application.

**Request** — [`DisconnectCalendarRequest`](#disconnectcalendarrequest)

_No fields; send `{}`._

**Response** — [`DisconnectCalendarResponse`](#disconnectcalendarresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`CalendarStatus`](#calendarstatus) | object |  | Current status. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.ListMeetings

`POST /api/career.v1.AdminService/ListMeetings` · **Auth:** Admin · **Rate limit:** default/min

Lists booked meetings, soonest first, so the owner can see what has
been taken without opening Google. Cancelling here frees the time
and removes the calendar event.

**Request** — [`ListMeetingsRequest`](#listmeetingsrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includePast` | `bool` | boolean |  | Include meetings that have already happened or been cancelled. |

**Response** — [`ListMeetingsResponse`](#listmeetingsresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meetings` | [`AdminMeeting`](#adminmeeting)[] | array of object |  | Meetings, soonest first. |
| `zone` | `string` | string |  | IANA zone the times should be rendered in. |

<details><summary>Example request body</summary>

```json
{
  "includePast": true
}
```

</details>

### AdminService.CancelMeetingAsAdmin

`POST /api/career.v1.AdminService/CancelMeetingAsAdmin` · **Auth:** Admin · **Rate limit:** default/min

Cancels a meeting on the member's behalf and frees the slot.

**Request** — [`CancelMeetingAsAdminRequest`](#cancelmeetingasadminrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) | `int64: gt: 0` | Which meeting to cancel. |

**Response** — [`CancelMeetingAsAdminResponse`](#cancelmeetingasadminresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meeting` | [`AdminMeeting`](#adminmeeting) | object |  | The cancelled meeting. |

<details><summary>Example request body</summary>

```json
{
  "id": "0"
}
```

</details>

### AdminService.GetJdSubmissionLimit

`POST /api/career.v1.AdminService/GetJdSubmissionLimit` · **Auth:** Admin · **Rate limit:** default/min

Reads how many postings one member may submit per rolling day.

**Request** — [`GetJdSubmissionLimitRequest`](#getjdsubmissionlimitrequest)

_No fields; send `{}`._

**Response** — [`GetJdSubmissionLimitResponse`](#getjdsubmissionlimitresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number |  | Postings one member may submit per rolling window. |
| `windowHours` | `int32` | number |  | How long the window is, in hours. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### AdminService.SetJdSubmissionLimit

`POST /api/career.v1.AdminService/SetJdSubmissionLimit` · **Auth:** Admin · **Rate limit:** default/min

Sets how many postings one member may submit per rolling day
(stored in app_settings; the api caches it for 15 s). Takes effect
for the next submission within seconds. A member already over a new
lower limit is not refunded and not penalised: they simply wait for
their oldest submission to age out.

**Request** — [`SetJdSubmissionLimitRequest`](#setjdsubmissionlimitrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number | `int32: lte: 100 gte: 0` | Postings one member may submit per rolling window. 0 removes the cap, which on a box that reviews one posting an hour means one member with a script can fill the queue indefinitely. |

**Response** — [`SetJdSubmissionLimitResponse`](#setjdsubmissionlimitresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number |  | Stored limit. |
| `windowHours` | `int32` | number |  | How long the window is, in hours. |

<details><summary>Example request body</summary>

```json
{
  "limit": 0
}
```

</details>

## SystemService

Version and governance status.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`GetVersion`](#systemservice-getversion) | `/api/career.v1.SystemService/GetVersion` | Public | default | `GetVersionRequest` → `GetVersionResponse` | Returns the deployed version and build metadata. |
| [`GetGovernanceStatus`](#systemservice-getgovernancestatus) | `/api/career.v1.SystemService/GetGovernanceStatus` | Public | default | `GetGovernanceStatusRequest` → `GetGovernanceStatusResponse` | Returns the governance status shown on the public "How this site was built" page: UxTS frameworks, spec counts, last verification, pass rate, and hash-integrity summary, read from the reports CI publishes. |
| [`GetReviewerStatus`](#systemservice-getreviewerstatus) | `/api/career.v1.SystemService/GetReviewerStatus` | Public | default | `GetReviewerStatusRequest` → `GetReviewerStatusResponse` | Returns how the JD reviewer is currently measuring, for the public "How Ask Roger works" page: agreement with the owner's own grading, and the last evaluation of the fixed posting set. |

### SystemService.GetVersion

`POST /api/career.v1.SystemService/GetVersion` · **Auth:** Public · **Rate limit:** default/min

Returns the deployed version and build metadata.

**Request** — [`GetVersionRequest`](#getversionrequest)

_No fields; send `{}`._

**Response** — [`GetVersionResponse`](#getversionresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `version` | `string` | string |  | Semantic version, e.g. `0.4.0`. |
| `commit` | `string` | string |  | Git commit SHA. |
| `builtAt` | `Timestamp` | string (RFC 3339, UTC) |  | Build time. |
| `goVersion` | `string` | string |  | Go toolchain version. |
| `personaVersion` | `string` | string |  | Active persona version. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### SystemService.GetGovernanceStatus

`POST /api/career.v1.SystemService/GetGovernanceStatus` · **Auth:** Public · **Rate limit:** default/min

Returns the governance status shown on the public "How this site was
built" page: UxTS frameworks, spec counts, last verification, pass rate,
and hash-integrity summary, read from the reports CI publishes.

**Request** — [`GetGovernanceStatusRequest`](#getgovernancestatusrequest)

_No fields; send `{}`._

**Response** — [`GetGovernanceStatusResponse`](#getgovernancestatusresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `frameworks` | [`FrameworkStatus`](#frameworkstatus)[] | array of object |  | Frameworks in matrix order. |
| `commit` | `string` | string |  | Commit the reports were produced from. |
| `publishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the summary was published. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### SystemService.GetReviewerStatus

`POST /api/career.v1.SystemService/GetReviewerStatus` · **Auth:** Public · **Rate limit:** default/min

Returns how the JD reviewer is currently measuring, for the public
"How Ask Roger works" page: agreement with the owner's own grading,
and the last evaluation of the fixed posting set. Read from the
same views the admin gate reads, so the public page cannot quote a
number the owner is not also looking at.

**Request** — [`GetReviewerStatusRequest`](#getreviewerstatusrequest)

_No fields; send `{}`._

**Response** — [`GetReviewerStatusResponse`](#getreviewerstatusresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `graded` | `int32` | number |  | Verdicts the owner has graded and that could be graded (he can also answer "not enough evidence to judge", which is excluded here). |
| `agreed` | `int32` | number |  | Of those, how many he agreed with. |
| `agreementPct` | `double` | number |  | Agreement as a percentage, to one decimal place. |
| `hardDisagreements` | `int32` | number |  | Disagreements where the model said met and he said unmet, or the reverse. Counted apart from the softer kind because they mean the model was wrong rather than unsure. |
| `tooHarsh` | `int32` | number |  | Of the hard disagreements, how many were the model refusing to credit something he can evidence. The opposite direction, crediting what he cannot evidence, is the one that would matter to an employer. |
| `tooGenerous` | `int32` | number |  | Of the hard disagreements, how many were the model crediting something he says is not evidenced. |
| `postings` | `int32` | number |  | Postings in the fixed evaluation set. |
| `postingsRandom` | `int32` | number |  | How many of those were drawn at random from job boards rather than chosen by the owner. |
| `scored` | `int32` | number |  | Postings scored in the last completed evaluation. |
| `gateCorrect` | `int32` | number |  | How many landed on the side the owner said they should. |
| `inversions` | `int32` | number |  | Pairs where a posting he said he could not do outscored one he said he could. Moving the threshold cannot fix one of these. |
| `margin` | `double` | number |  | _(oneof `_margin`)_ The gap between the two groups. Absent when one side is empty. |
| `evaluatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When that evaluation ran. |
| `model` | `string` | string |  | The model that produced it. |
| `bands` | [`JdFitBandsPublic`](#jdfitbandspublic) | object |  | The five fit bands in force, so a page can quote the real thresholds without a session. Without these an anonymous reader is shown numbers derived from an environment default, which are right only until the owner edits the bands. |
| `comparison` | [`ReviewerComparisonRow`](#reviewercomparisonrow)[] | array of object |  | Every posting in the last completed evaluation beside its score in the evaluation before it. Empty until two have completed. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

## DecisionTestService

Runs one sitting of the decision test.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`StartSession`](#decisiontestservice-startsession) | `/api/career.v1.DecisionTestService/StartSession` | Public | default | `StartSessionRequest` → `StartSessionResponse` | Opens a session and returns the practice block. |
| [`GetBlock`](#decisiontestservice-getblock) | `/api/career.v1.DecisionTestService/GetBlock` | Public | default | `GetBlockRequest` → `GetBlockResponse` | Returns the next block: the number to memorise and its six questions. |
| [`SubmitAnswer`](#decisiontestservice-submitanswer) | `/api/career.v1.DecisionTestService/SubmitAnswer` | Public | default | `SubmitAnswerRequest` → `SubmitAnswerResponse` | Records one answer and grades it server-side. |
| [`SubmitRecall`](#decisiontestservice-submitrecall) | `/api/career.v1.DecisionTestService/SubmitRecall` | Public | default | `SubmitRecallRequest` → `SubmitRecallResponse` | Records the digits returned at the end of a block, scored by failure type rather than pass or fail. |
| [`FinishSession`](#decisiontestservice-finishsession) | `/api/career.v1.DecisionTestService/FinishSession` | Public | default | `FinishSessionRequest` → `FinishSessionResponse` | Closes the session. |

### DecisionTestService.StartSession

`POST /api/career.v1.DecisionTestService/StartSession` · **Auth:** Public · **Rate limit:** default/min

Opens a session and returns the practice block. Called when the
participant presses start, which is also the gesture that unlocks
audio in the browser.

**Request** — [`StartSessionRequest`](#startsessionrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `intake` | [`Intake`](#intake) | object |  | Demographics, every field optional. |
| `conditions` | [`Conditions`](#conditions) | object |  | Device and setup, measured before the first question. |
| `synthetic` | `bool` | boolean |  | True when an agent is driving the UI rather than a person, so the rows are excluded from analysis by filter. Separate from the instrument version because the most useful agent run is against the exact version production serves. |

**Response** — [`StartSessionResponse`](#startsessionresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string |  | Opaque handle for the rest of the run. Not a database id. |
| `practice` | [`Block`](#block) | object |  | The practice block, which carries the tick so a participant hears it before anything is scored. |
| `blockCount` | `int32` | number |  | How many scored blocks follow. |
| `questionCount` | `int32` | number |  | How many scored questions in total, for the progress counter. |
| `timings` | [`Timings`](#timings) | object |  | The timings in force for this run. |

<details><summary>Example request body</summary>

```json
{
  "intake": {
    "displayName": "string",
    "ageRange": "string",
    "education": "string",
    "occupation": "string",
    "email": "string",
    "wantsResults": true
  },
  "conditions": {
    "audioMode": "string",
    "deviceClass": "string",
    "tapCheckPassed": true,
    "baselineRtMs": 0,
    "baselineRtSdMs": 0
  },
  "synthetic": true
}
```

</details>

### DecisionTestService.GetBlock

`POST /api/career.v1.DecisionTestService/GetBlock` · **Auth:** Public · **Rate limit:** default/min

Returns the next block: the number to memorise and its six
questions. Blocks are served one at a time so the client never
holds the whole test.

**Request** — [`GetBlockRequest`](#getblockrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From StartSessionResponse. |
| `blockNo` | `int32` | number | `int32: lte: 5 gte: 1` | One-based. Block five returns to block one's difficulty and is the fatigue control. |

**Response** — [`GetBlockResponse`](#getblockresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `block` | [`Block`](#block) | object |  | The block. |

<details><summary>Example request body</summary>

```json
{
  "sessionKey": "string",
  "blockNo": 0
}
```

</details>

### DecisionTestService.SubmitAnswer

`POST /api/career.v1.DecisionTestService/SubmitAnswer` · **Auth:** Public · **Rate limit:** default/min

Records one answer and grades it server-side. The response confirms
receipt and says nothing about correctness.

**Request** — [`SubmitAnswerRequest`](#submitanswerrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From StartSessionResponse. |
| `blockNo` | `int32` | number | `int32: lte: 5 gte: 1` | Which block this answer belongs to. |
| `positionOverall` | `int32` | number | `int32: gte: 1` | Position in the whole test, one-based. |
| `chosenIndex` | `int32` | number | `int32: gte: -1` | Index into the question's options, or -1 when the question expired unanswered. Expiry is its own outcome rather than an error, because a timeout under load is a result. |
| `latencyMs` | `int32` | number | `int32: gte: 0` | Milliseconds from the question appearing to the answer committed. |
| `confidence` | `int32` | number | `int32: lte: 100 gte: 0` | The participant's own rating, 0 to 100, on every question. Error rate alone cannot answer a question about being wrong *and* sure. |

**Response** — [`SubmitAnswerResponse`](#submitanswerresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `stored` | `bool` | boolean |  | True when the answer was stored. Never says whether it was right. |

<details><summary>Example request body</summary>

```json
{
  "sessionKey": "string",
  "blockNo": 0,
  "positionOverall": 0,
  "chosenIndex": 0,
  "latencyMs": 0,
  "confidence": 0
}
```

</details>

### DecisionTestService.SubmitRecall

`POST /api/career.v1.DecisionTestService/SubmitRecall` · **Auth:** Public · **Rate limit:** default/min

Records the digits returned at the end of a block, scored by
failure type rather than pass or fail.

**Request** — [`SubmitRecallRequest`](#submitrecallrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From StartSessionResponse. |
| `blockNo` | `int32` | number | `int32: lte: 5 gte: 1` | Which block is being closed. |
| `digits` | `string` | string | `string: max_len: 16` | What the participant typed, empty when the step expired. |
| `latencyMs` | `int32` | number | `int32: gte: 0` | Milliseconds spent on the recall step. |

**Response** — [`SubmitRecallResponse`](#submitrecallresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `stored` | `bool` | boolean |  | True when the recall was stored. |

<details><summary>Example request body</summary>

```json
{
  "sessionKey": "string",
  "blockNo": 0,
  "digits": "string",
  "latencyMs": 0
}
```

</details>

### DecisionTestService.FinishSession

`POST /api/career.v1.DecisionTestService/FinishSession` · **Auth:** Public · **Rate limit:** default/min

Closes the session. A session that is never finished stays
abandoned, which is kept rather than deleted because where people
stop measures the fifteen-minute burden.

**Request** — [`FinishSessionRequest`](#finishsessionrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From StartSessionResponse. |
| `recallStrategy` | `string` | string | `string: max_len: 16` | Whether the participant converted the number at encoding or carried it and transformed at recall. Asked in the debrief because the two produce different loads during the questions, which turns an uncontrolled variable into a recorded one. |

**Response** — [`FinishSessionResponse`](#finishsessionresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `correct` | `int32` | number |  | How many of the thirty were answered correctly. The only figure a participant is ever shown, and never item by item. |
| `total` | `int32` | number |  | How many were scored, so the figure reads as "N of M". |

<details><summary>Example request body</summary>

```json
{
  "sessionKey": "string",
  "recallStrategy": "string"
}
```

</details>

## SidecarService

Embedding, reranking, classification, and batch jobs.

> **Internal.** Served over gRPC on the Compose network only and never routed by the reverse proxy. Documented here because the API's request path depends on it.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`Embed`](#sidecarservice-embed) | `/career.sidecar.v1.SidecarService/Embed (gRPC)` | — | — | `EmbedRequest` → `EmbedResponse` | Embeds one or more texts with the corpus's embedding model. |
| [`Rerank`](#sidecarservice-rerank) | `/career.sidecar.v1.SidecarService/Rerank (gRPC)` | — | — | `RerankRequest` → `RerankResponse` | Re-scores candidate chunks against a query with a cross-encoder and returns the top-k. |
| [`Classify`](#sidecarservice-classify) | `/career.sidecar.v1.SidecarService/Classify (gRPC)` | — | — | `ClassifyRequest` → `ClassifyResponse` | Classifies a member message as in scope, out of scope, or a suspected prompt injection. |
| [`RunJob`](#sidecarservice-runjob) | `/career.sidecar.v1.SidecarService/RunJob (gRPC)` | — | — | `RunJobRequest` → `RunJobResponse` | Starts a batch job (the same operations as `career-cli`). |
| [`GetJob`](#sidecarservice-getjob) | `/career.sidecar.v1.SidecarService/GetJob (gRPC)` | — | — | `GetJobRequest` → `GetJobResponse` | Returns a job's status. |
| [`Health`](#sidecarservice-health) | `/career.sidecar.v1.SidecarService/Health (gRPC)` | — | — | `HealthRequest` → `HealthResponse` | Reports readiness: models loaded, storage reachable, version. |
| [`Generate`](#sidecarservice-generate) | `/career.sidecar.v1.SidecarService/Generate (gRPC)` | — | — | `GenerateRequest` → `GenerateResponse` | Runs one chat completion with the configured LLM provider (Ollama locally / on-host, a hosted API later, stub in CI). |
| [`RenderResume`](#sidecarservice-renderresume) | `/career.sidecar.v1.SidecarService/RenderResume (gRPC)` | — | — | `RenderResumeRequest` → `RenderResumeResponse` | Renders a verified résumé (the API's structured JSON) to a PDF with Typst and applies an owner password so the file opens freely but cannot be edited. |

### SidecarService.Embed

`/career.sidecar.v1.SidecarService/Embed` (gRPC)

Embeds one or more texts with the corpus's embedding model. Query and
document embeddings go through this single implementation so ingest and
retrieval never diverge. Deadline on the request path: 2 s.

**Request** — [`EmbedRequest`](#embedrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `texts` | `string`[] | array of string | `repeated: min_items: 1 max_items: 64 items: string: min_len: 1 max_len: 8000` | Texts, 1–64 per call; each at most 8,000 characters. |
| `purpose` | [`EmbedPurpose`](#embedpurpose) | string (enum name) | `enum: defined_only: true not_in: 0` | Purpose. |

**Response** — [`EmbedResponse`](#embedresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `embeddings` | [`Embedding`](#embedding)[] | array of object |  | One embedding per input text. |
| `model` | `string` | string |  | Model identifier. |
| `dimensions` | `int32` | number |  | Vector dimensions. |

<details><summary>Example request body</summary>

```json
{
  "texts": [
    "string"
  ],
  "purpose": "EMBED_PURPOSE_QUERY"
}
```

</details>

### SidecarService.Rerank

`/career.sidecar.v1.SidecarService/Rerank` (gRPC)

Re-scores candidate chunks against a query with a cross-encoder and
returns the top-k. Deadline on the request path: 3 s; on timeout the API
keeps the hybrid-retrieval order.

**Request** — [`RerankRequest`](#rerankrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: min_len: 1 max_len: 4000` | The query. |
| `candidates` | [`RerankCandidate`](#rerankcandidate)[] | array of object | `repeated: min_items: 1 max_items: 100` | Candidates, 1–100. |
| `topK` | `int32` | number | `int32: lte: 50 gte: 1` | How many to return, 1–50. |

**Response** — [`RerankResponse`](#rerankresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `scores` | [`RerankScore`](#rerankscore)[] | array of object |  | Top-k in descending score. |
| `model` | `string` | string |  | Model identifier. |

<details><summary>Example request body</summary>

```json
{
  "query": "string",
  "candidates": [
    {
      "id": "string",
      "text": "string"
    }
  ],
  "topK": 0
}
```

</details>

### SidecarService.Classify

`/career.sidecar.v1.SidecarService/Classify` (gRPC)

Classifies a member message as in scope, out of scope, or a suspected
prompt injection. Deadline: 1 s; on timeout the API treats the message as
UNCERTAIN and lets the persona prompt's own scope rules decide.

**Request** — [`ClassifyRequest`](#classifyrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `text` | `string` | string | `string: min_len: 1 max_len: 4000` | The member message. |
| `conversationSummary` | `string` | string | `string: max_len: 2000` | Short summary of the conversation so far; optional. |

**Response** — [`ClassifyResponse`](#classifyresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `scope` | [`Scope`](#scope) | string (enum name) |  | Verdict. |
| `confidence` | `float` | number |  | Confidence in [0, 1]. |
| `reason` | `string` | string |  | Short reason, for the review queue. |

<details><summary>Example request body</summary>

```json
{
  "text": "string",
  "conversationSummary": "string"
}
```

</details>

### SidecarService.RunJob

`/career.sidecar.v1.SidecarService/RunJob` (gRPC)

Starts a batch job (the same operations as `career-cli`). Returns
immediately with a job ID.

**Request** — [`RunJobRequest`](#runjobrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`JobKind`](#jobkind) | string (enum name) | `enum: defined_only: true not_in: 0` | Which job. |
| `args` | map<`string`, `string`> | object of string | `map: max_pairs: 16` | Job arguments, e.g. `{"content_ids": "prj-mdemg,art-ftw"}`. |
| `requestedBy` | `string` | string | `string: max_len: 128` | Who requested it, for the job log (`scheduler`, `admin:<member_id>`, `ci`). |

**Response** — [`RunJobResponse`](#runjobresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string |  | Job ID. |

<details><summary>Example request body</summary>

```json
{
  "kind": "JOB_KIND_CONTENT_VALIDATE",
  "args": {
    "key": "string"
  },
  "requestedBy": "string"
}
```

</details>

### SidecarService.GetJob

`/career.sidecar.v1.SidecarService/GetJob` (gRPC)

Returns a job's status.

**Request** — [`GetJobRequest`](#getjobrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string | `string: min_len: 1 max_len: 64` | Job ID. |

**Response** — [`GetJobResponse`](#getjobresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string |  | Job ID. |
| `kind` | [`JobKind`](#jobkind) | string (enum name) |  | Which job. |
| `status` | [`JobStatus`](#jobstatus) | string (enum name) |  | Status. |
| `progressPct` | `int32` | number |  | Progress in percent, when reported. |
| `startedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Start time. |
| `finishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Finish time. |
| `summary` | `string` | string |  | Summary or error text. |
| `logTail` | `string`[] | array of string |  | Last 50 log lines. |

<details><summary>Example request body</summary>

```json
{
  "jobId": "string"
}
```

</details>

### SidecarService.Health

`/career.sidecar.v1.SidecarService/Health` (gRPC)

Reports readiness: models loaded, storage reachable, version.

**Request** — [`HealthRequest`](#healthrequest)

_No fields; send `{}`._

**Response** — [`HealthResponse`](#healthresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `ready` | `bool` | boolean |  | True when the sidecar can serve request-path RPCs. |
| `embedderReady` | `bool` | boolean |  | Embedding model loaded or reachable. |
| `rerankerReady` | `bool` | boolean |  | Reranker loaded (optional component). |
| `storageReady` | `bool` | boolean |  | Object storage reachable. |
| `version` | `string` | string |  | Sidecar version. |
| `llmReady` | `bool` | boolean |  | LLM provider reachable and serving its configured model, confirmed by asking it rather than by reading configuration. This used to report only that a provider was configured, which meant stopping Ollama did not change the answer and every call failed later, at request time, one posting at a time. |
| `llmProvider` | `string` | string |  | LLM provider name (`stub`, `ollama:<model>`) so the API can decide whether structured pipelines are meaningful. |
| `rendererReady` | `bool` | boolean |  | PDF renderer available (Typst compiler importable). |
| `llmDetail` | `string` | string |  | Why llm_ready is false: the server unreachable, the model absent from its tag list, and so on. Empty when llm_ready is true. Carried so the api can log a cause rather than the bare fact of a refusal. |

<details><summary>Example request body</summary>

```json
{}
```

</details>

### SidecarService.Generate

`/career.sidecar.v1.SidecarService/Generate` (gRPC)

Runs one chat completion with the configured LLM provider (Ollama
locally / on-host, a hosted API later, stub in CI). Non-streaming;
the API uses it for background generation (JD-tailored résumés).
The API owns the prompt text and its version; the sidecar is a
provider gateway and never rewrites prompts. Deadline is set by the
caller and can be minutes on CPU inference.

**Request** — [`GenerateRequest`](#generaterequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `system` | `string` | string | `string: max_len: 32000` | System prompt (persona, rules, output format). Owned by the API's prompt registry; pinned by (prompt_id, prompt_version) there. |
| `user` | `string` | string | `string: min_len: 1 max_len: 200000` | User turn: the task plus any retrieved evidence, with untrusted text already wrapped in delimiters by the caller. |
| `maxTokens` | `int32` | number | `int32: lte: 8192 gte: 0` | Completion cap in tokens; 0 uses the provider default. |
| `temperature` | `float` | number | `float: lte: 2.0 gte: 0.0` | Sampling temperature; 0 is deterministic where the provider allows. |
| `json` | `bool` | boolean |  | Ask the provider for a JSON object response. |
| `traceId` | `string` | string | `string: max_len: 64` | Caller trace id for log correlation (e.g. `jd:42`). |
| `jsonSchema` | `string` | string | `string: max_len: 32000` | JSON Schema (draft 2020-12 subset) the response must satisfy. Providers that support constrained decoding (Ollama `format`) enforce it; others fall back to plain JSON mode and the caller validates. Implies json=true. |

**Response** — [`GenerateResponse`](#generateresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `text` | `string` | string |  | Generated text with any reasoning scaffolding stripped. |
| `model` | `string` | string |  | Provider model identifier (e.g. `ollama:qwen3:14b`). |
| `promptTokens` | `int32` | number |  | Prompt tokens as reported by the provider; 0 if unknown. |
| `completionTokens` | `int32` | number |  | Completion tokens as reported by the provider; 0 if unknown. |
| `latencyMs` | `int64` | string (decimal) |  | Wall-clock time of the provider call in milliseconds. |
| `finishReason` | `string` | string |  | Provider finish reason (`stop`, `length`, ...), empty if unknown. |
| `promptEvalMs` | `int64` | string (decimal) |  | Milliseconds the provider spent evaluating the prompt, as it reported them. This is what a reader waits before the first word, so on a CPU-only host it is the number that decides whether an answer feels slow; latency_ms alone cannot separate it from the time spent writing. Zero when the provider does not report it. |
| `evalMs` | `int64` | string (decimal) |  | Milliseconds the provider spent generating, as it reported them. With prompt_eval_ms this splits the wait into the part that grows with retrieved context and the part that grows with answer length, which are fixed in completely different ways. Zero when the provider does not report it. |

<details><summary>Example request body</summary>

```json
{
  "system": "string",
  "user": "string",
  "maxTokens": 0,
  "temperature": 0.5,
  "json": true,
  "traceId": "string",
  "jsonSchema": "string"
}
```

</details>

### SidecarService.RenderResume

`/career.sidecar.v1.SidecarService/RenderResume` (gRPC)

Renders a verified résumé (the API's structured JSON) to a PDF with
Typst and applies an owner password so the file opens freely but
cannot be edited. Rendering never touches a model.

**Request** — [`RenderResumeRequest`](#renderresumerequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `resumeJson` | `string` | string | `string: min_len: 2 max_len: 200000` | The verified résumé JSON (headline, summary, competencies, experience, education). Source ids are ignored by the renderer. |
| `ownerPassword` | `string` | string | `string: min_len: 1 max_len: 128` | Owner password that locks editing; the user password is always empty so the PDF opens without a prompt. Required. |
| `traceId` | `string` | string | `string: max_len: 64` | Caller trace id for log correlation (e.g. `jd:42`). |

**Response** — [`RenderResumeResponse`](#renderresumeresponse)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `pdf` | `bytes` | string (base64) |  | The encrypted PDF. |
| `pages` | `int32` | number |  | Page count. |
| `engine` | `string` | string |  | Renderer identifier (e.g. `typst:0.15.0`). |

<details><summary>Example request body</summary>

```json
{
  "resumeJson": "string",
  "ownerPassword": "string",
  "traceId": "string"
}
```

</details>

## Types

Every message and enum, in declaration order. Request and response messages already shown above are included so links resolve.

### EmbedRequest

Texts to embed.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `texts` | `string`[] | array of string | `repeated: min_items: 1 max_items: 64 items: string: min_len: 1 max_len: 8000` | Texts, 1–64 per call; each at most 8,000 characters. |
| `purpose` | [`EmbedPurpose`](#embedpurpose) | string (enum name) | `enum: defined_only: true not_in: 0` | Purpose. |

### Embedding

One embedding vector.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `values` | `float`[] | array of number |  | Vector components. |

### EmbedResponse

Embeddings in request order.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `embeddings` | [`Embedding`](#embedding)[] | array of object |  | One embedding per input text. |
| `model` | `string` | string |  | Model identifier. |
| `dimensions` | `int32` | number |  | Vector dimensions. |

### RerankCandidate

A candidate chunk for reranking.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Chunk ID. |
| `text` | `string` | string | `string: min_len: 1 max_len: 8000` | Chunk text. |

### RerankRequest

Rerank request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: min_len: 1 max_len: 4000` | The query. |
| `candidates` | [`RerankCandidate`](#rerankcandidate)[] | array of object | `repeated: min_items: 1 max_items: 100` | Candidates, 1–100. |
| `topK` | `int32` | number | `int32: lte: 50 gte: 1` | How many to return, 1–50. |

### RerankScore

A reranked candidate.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Chunk ID. |
| `score` | `float` | number |  | Relevance score (higher is better). |

### RerankResponse

Reranked candidates.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `scores` | [`RerankScore`](#rerankscore)[] | array of object |  | Top-k in descending score. |
| `model` | `string` | string |  | Model identifier. |

### ClassifyRequest

Classification request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `text` | `string` | string | `string: min_len: 1 max_len: 4000` | The member message. |
| `conversationSummary` | `string` | string | `string: max_len: 2000` | Short summary of the conversation so far; optional. |

### ClassifyResponse

Classification result.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `scope` | [`Scope`](#scope) | string (enum name) |  | Verdict. |
| `confidence` | `float` | number |  | Confidence in [0, 1]. |
| `reason` | `string` | string |  | Short reason, for the review queue. |

### RunJobRequest

Job start.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`JobKind`](#jobkind) | string (enum name) | `enum: defined_only: true not_in: 0` | Which job. |
| `args` | map<`string`, `string`> | object of string | `map: max_pairs: 16` | Job arguments, e.g. `{"content_ids": "prj-mdemg,art-ftw"}`. |
| `requestedBy` | `string` | string | `string: max_len: 128` | Who requested it, for the job log (`scheduler`, `admin:<member_id>`, `ci`). |

### RunJobResponse

Job handle.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string |  | Job ID. |

### GetJobRequest

Job status request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string | `string: min_len: 1 max_len: 64` | Job ID. |

### GetJobResponse

Job status.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string |  | Job ID. |
| `kind` | [`JobKind`](#jobkind) | string (enum name) |  | Which job. |
| `status` | [`JobStatus`](#jobstatus) | string (enum name) |  | Status. |
| `progressPct` | `int32` | number |  | Progress in percent, when reported. |
| `startedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Start time. |
| `finishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Finish time. |
| `summary` | `string` | string |  | Summary or error text. |
| `logTail` | `string`[] | array of string |  | Last 50 log lines. |

### GenerateRequest

One chat completion.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `system` | `string` | string | `string: max_len: 32000` | System prompt (persona, rules, output format). Owned by the API's prompt registry; pinned by (prompt_id, prompt_version) there. |
| `user` | `string` | string | `string: min_len: 1 max_len: 200000` | User turn: the task plus any retrieved evidence, with untrusted text already wrapped in delimiters by the caller. |
| `maxTokens` | `int32` | number | `int32: lte: 8192 gte: 0` | Completion cap in tokens; 0 uses the provider default. |
| `temperature` | `float` | number | `float: lte: 2.0 gte: 0.0` | Sampling temperature; 0 is deterministic where the provider allows. |
| `json` | `bool` | boolean |  | Ask the provider for a JSON object response. |
| `traceId` | `string` | string | `string: max_len: 64` | Caller trace id for log correlation (e.g. `jd:42`). |
| `jsonSchema` | `string` | string | `string: max_len: 32000` | JSON Schema (draft 2020-12 subset) the response must satisfy. Providers that support constrained decoding (Ollama `format`) enforce it; others fall back to plain JSON mode and the caller validates. Implies json=true. |

### GenerateResponse

Completion result plus the accounting the API's usage ledger needs.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `text` | `string` | string |  | Generated text with any reasoning scaffolding stripped. |
| `model` | `string` | string |  | Provider model identifier (e.g. `ollama:qwen3:14b`). |
| `promptTokens` | `int32` | number |  | Prompt tokens as reported by the provider; 0 if unknown. |
| `completionTokens` | `int32` | number |  | Completion tokens as reported by the provider; 0 if unknown. |
| `latencyMs` | `int64` | string (decimal) |  | Wall-clock time of the provider call in milliseconds. |
| `finishReason` | `string` | string |  | Provider finish reason (`stop`, `length`, ...), empty if unknown. |
| `promptEvalMs` | `int64` | string (decimal) |  | Milliseconds the provider spent evaluating the prompt, as it reported them. This is what a reader waits before the first word, so on a CPU-only host it is the number that decides whether an answer feels slow; latency_ms alone cannot separate it from the time spent writing. Zero when the provider does not report it. |
| `evalMs` | `int64` | string (decimal) |  | Milliseconds the provider spent generating, as it reported them. With prompt_eval_ms this splits the wait into the part that grows with retrieved context and the part that grows with answer length, which are fixed in completely different ways. Zero when the provider does not report it. |

### RenderResumeRequest

Résumé render request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `resumeJson` | `string` | string | `string: min_len: 2 max_len: 200000` | The verified résumé JSON (headline, summary, competencies, experience, education). Source ids are ignored by the renderer. |
| `ownerPassword` | `string` | string | `string: min_len: 1 max_len: 128` | Owner password that locks editing; the user password is always empty so the PDF opens without a prompt. Required. |
| `traceId` | `string` | string | `string: max_len: 64` | Caller trace id for log correlation (e.g. `jd:42`). |

### RenderResumeResponse

Résumé render result.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `pdf` | `bytes` | string (base64) |  | The encrypted PDF. |
| `pages` | `int32` | number |  | Page count. |
| `engine` | `string` | string |  | Renderer identifier (e.g. `typst:0.15.0`). |

### HealthRequest

Empty.

_No fields._

### HealthResponse

Readiness.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `ready` | `bool` | boolean |  | True when the sidecar can serve request-path RPCs. |
| `embedderReady` | `bool` | boolean |  | Embedding model loaded or reachable. |
| `rerankerReady` | `bool` | boolean |  | Reranker loaded (optional component). |
| `storageReady` | `bool` | boolean |  | Object storage reachable. |
| `version` | `string` | string |  | Sidecar version. |
| `llmReady` | `bool` | boolean |  | LLM provider reachable and serving its configured model, confirmed by asking it rather than by reading configuration. This used to report only that a provider was configured, which meant stopping Ollama did not change the answer and every call failed later, at request time, one posting at a time. |
| `llmProvider` | `string` | string |  | LLM provider name (`stub`, `ollama:<model>`) so the API can decide whether structured pipelines are meaningful. |
| `rendererReady` | `bool` | boolean |  | PDF renderer available (Typst compiler importable). |
| `llmDetail` | `string` | string |  | Why llm_ready is false: the server unreachable, the model absent from its tag list, and so on. Empty when llm_ready is true. Carried so the api can log a cause rather than the bare fact of a refusal. |

### TrackWeight

Relevance of a content item to one track, as authored in the item's
frontmatter. Weights are in [0, 1].

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `trackId` | `string` | string | `string: min_len: 1 max_len: 64 pattern: "^[a-z0-9-]+$"` | Track identifier from `content/tracks.yaml`, e.g. `automation-control`. |
| `weight` | `float` | number | `float: lte: 1.0 gte: 0.0` | Weight in [0, 1]. |

### Track

A curated area of the owner's experience. Members declare interest in
tracks; content carries track weights; tailoring orders by the product.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Stable identifier, e.g. `ai-agentic`. |
| `name` | `string` | string |  | Display name, e.g. "AI/ML, agentic systems, and RAG". |
| `framing` | `string` | string |  | One-paragraph framing in the owner's voice. |
| `startHere` | `string`[] | array of string |  | Content IDs of the "Start here" reading path, in order. |
| `suggestedQuestions` | `string`[] | array of string |  | Suggested assistant questions for this track. |
| `resumeVariant` | `string` | string |  | Résumé variant that best fits the track (a DownloadItem.variant). |

### ContentSummary

A content item as it appears in lists, feeds, and citations. The full
body and type-specific details are returned by ContentService.GetContent.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Stable content ID from frontmatter, e.g. `prj-mdemg`. |
| `slug` | `string` | string |  | URL segment, e.g. `mdemg`. |
| `type` | [`ContentType`](#contenttype) | string (enum name) |  | Content kind. |
| `title` | `string` | string |  | Title. |
| `summary` | `string` | string |  | One- or two-sentence summary (≤ 240 characters). |
| `published` | `Timestamp` | string (RFC 3339, UTC) |  | First publication date. |
| `updated` | `Timestamp` | string (RFC 3339, UTC) |  | Last material update. |
| `tags` | `string`[] | array of string |  | Free-form tags. |
| `tracks` | [`TrackWeight`](#trackweight)[] | array of object |  | Track weights as authored. |
| `relevance` | `float` | number |  | Relevance score for the calling member under current tailoring (§8.8 of the FSD). Zero when tailoring is off. |
| `reason` | `string` | string |  | Human-readable reason the item is ranked where it is, e.g. "Shown first because you selected Industrial automation". Empty when tailoring is off. |
| `isNew` | `bool` | boolean |  | True when published or materially updated since the member's last visit. |
| `viewed` | `bool` | boolean |  | True when the member has viewed the item. |
| `saved` | `bool` | boolean |  | True when the member has saved the item. |
| `path` | `string` | string |  | Path of the site page for the item, e.g. `/projects/mdemg`. |

### PageRequest

Cursor pagination for list requests.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `pageSize` | `int32` | number | `int32: lte: 100 gte: 0` | Items per page, 1–100. Zero selects the default of 20. |
| `pageToken` | `string` | string | `string: max_len: 512` | Opaque cursor from a previous response; empty for the first page. |

### PageResponse

Cursor pagination for list responses.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `nextPageToken` | `string` | string |  | Cursor for the next page; empty when there are no more items. |
| `totalCount` | `int32` | number |  | Total items matching the request, when cheap to compute; otherwise 0. |

### TrackInterest

A member's declared interest in a track.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `trackId` | `string` | string | `string: min_len: 1 max_len: 64 pattern: "^[a-z0-9-]+$"` | Track identifier. |
| `weight` | `float` | number | `float: lte: 1.0 gte: 0.0` | Interest weight in [0, 1]; the questionnaire assigns 1.0. |
| `source` | [`InterestSource`](#interestsource) | string (enum name) |  | Where the interest came from. |

### Profile

Questionnaire answers and tailoring settings. All fields are optional.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `seniority` | `string` | string | `string: max_len: 64` | Seniority of the role the member is hiring for, free text (e.g. "Director", "VP", "Principal"). |
| `hiringFor` | `string` | string | `string: max_len: 200` | The role being hired for, free text. |
| `priorities` | [`Priority`](#priority)[] | array of string (enum name) | `repeated: max_items: 3 items: enum: defined_only: true` | What matters most, up to three. |
| `heardFrom` | `string` | string | `string: max_len: 200` | How the member heard of the owner, free text. |
| `tailoringEnabled` | `bool` | boolean |  | Whether tailoring is enabled ("Show everything" toggles this off). |
| `questionnaireCompletedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the questionnaire was completed or skipped; unset until then. |

### Me

The calling member. Returned by sign-in, verification, GetMe, and GetHome.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Member ID (UUID). |
| `name` | `string` | string |  | Display name. |
| `email` | `string` | string |  | Email address (verified unless status is UNVERIFIED). |
| `organization` | `string` | string |  | Organization, self-reported. |
| `statedRole` | `string` | string |  | Stated role or title, self-reported. |
| `status` | [`MemberStatus`](#memberstatus) | string (enum name) |  | Account status. |
| `role` | [`MemberRole`](#memberrole) | string (enum name) |  | Account role. |
| `mfaEnrolled` | `bool` | boolean |  | Whether TOTP MFA is enrolled (admins only). |
| `profile` | [`Profile`](#profile) | object |  | Questionnaire answers and tailoring settings. |
| `interests` | [`TrackInterest`](#trackinterest)[] | array of object |  | Declared track interests. |
| `consentVersion` | `string` | string |  | Version of the consent text the member accepted at registration. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | Registration time. |
| `lastSeenAt` | `Timestamp` | string (RFC 3339, UTC) |  | Start of the previous session, for welcome-back copy; unset on first visit. |
| `linkedProviders` | `string`[] | array of string |  | Linked social sign-in providers, e.g. ["linkedin"]. |
| `expiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | When this member's access ends (from users.expires_at). Unset for permanent access and for non-active statuses. |
| `lastNotificationKind` | `string` | string |  | Kind of the last notification email the API sent to this member, e.g. "user_approved" / "user_declined" / "expiry_warn". Empty until the first send. Only populated on admin-side responses. |
| `lastNotificationAt` | `Timestamp` | string (RFC 3339, UTC) |  | Wall-clock time of the last notification send attempt (success OR failure). Admin-side only. |
| `lastNotificationError` | `string` | string |  | Truncated provider error from the last failed send; empty on success. Admin-side only. |

### ActivityEvent

A recorded member action. Sent by the client in batches
(ActivityService.RecordEvents) and returned in history views.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`ActivityEvent.Kind`](#activityeventkind) | string (enum name) | `enum: defined_only: true not_in: 0` | What happened. |
| `contentId` | `string` | string | `string: max_len: 128` | Content item involved, when any. |
| `occurredAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it happened on the client; the server clamps to [now − 24h, now]. |
| `dwellMs` | `int32` | number | `int32: gte: 0` | Time spent on a content page in milliseconds (VIEW only). |
| `query` | `string` | string | `string: max_len: 200` | Search query (SEARCH only). |
| `conversationId` | `string` | string | `string: max_len: 64` | Conversation involved (CHAT and ESCALATE only). |
| `variant` | `string` | string | `string: max_len: 64` | Résumé variant downloaded (DOWNLOAD only). |
| `clientEventId` | `string` | string | `string: max_len: 64` | Client-generated ID used to de-duplicate retried batches. |
| `content` | [`ContentSummary`](#contentsummary) | object |  | Summary of the content item, populated in history responses only. |

### RecordEventsRequest

A batch of events.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `events` | [`ActivityEvent`](#activityevent)[] | array of object | `repeated: min_items: 1 max_items: 50` | Events, 1–50 per batch. |

### RecordEventsResponse

Batch result.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `accepted` | `int32` | number |  | Events accepted (new). |
| `duplicates` | `int32` | number |  | Events ignored as duplicates. |

### AdminQuery

One query offered in the admin dropdown.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Stable id sent back to run it. |
| `label` | `string` | string |  | What the dropdown shows. |
| `detail` | `string` | string |  | What the result means. |

### ListAdminQueriesRequest

Empty: the list is the same for every admin.

_No fields._

### ListAdminQueriesResponse

The queries an admin may run.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `queries` | [`AdminQuery`](#adminquery)[] | array of object |  | Offered queries, in display order. |

### RunAdminQueryRequest

Run one named query.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Id from ListAdminQueries. Anything else is refused. |

### AdminQueryRow

One row of a query result.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `cells` | `string`[] | array of string |  | Cells, in the same order as the response's columns. A NULL is an empty string, because a gap reads better in a column of counts than the word "null", which invites being read as a value. |

### RunAdminQueryResponse

What the query returned.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | [`AdminQuery`](#adminquery) | object |  | The query that ran, echoed so the surface can label the result. |
| `result` | `string` | string |  | A one-line summary of the table below. Deprecated. This was the whole result before it became rows and columns, and it is kept because the breaking-change rules here are FILE, the strictest set, which forbids removing a field even when its number is reserved. That is the right default: a client built against the old contract keeps working rather than silently reading nothing. Still populated rather than emptied, for the same reason. One row becomes its cells joined; several become a count. |
| `ranAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it ran. |
| `columns` | `string`[] | array of string |  | Column headers, taken from the statement itself so a query and its header cannot drift apart. |
| `rows` | [`AdminQueryRow`](#adminqueryrow)[] | array of object |  | Rows, capped server side. Every query here aggregates, so a long result means a GROUP BY went wider than expected. |

### Conversation

A conversation header.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Conversation ID. |
| `title` | `string` | string |  | Title, derived from the first question unless renamed. |
| `startedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Start time. |
| `lastMessageAt` | `Timestamp` | string (RFC 3339, UTC) |  | Time of the last message. |
| `messageCount` | `int32` | number |  | Number of messages. |
| `personaVersion` | `string` | string |  | Persona version in effect. |
| `contextContentId` | `string` | string |  | Content item the conversation was opened from, when any. |

### Citation

A source cited by an assistant message.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `chunkId` | `string` | string |  | Corpus chunk ID. |
| `contentId` | `string` | string |  | Content item the chunk belongs to; empty for private-corpus sources. |
| `title` | `string` | string |  | Title to display. |
| `path` | `string` | string |  | Site path to link to; empty for private-corpus sources. |
| `heading` | `string` | string |  | Heading within the item, when known. |
| `rank` | `int32` | number |  | Rank within the answer's citations (1-based). |

### ProposedAction

An action the assistant proposed, after the server validated it
against the allowlist (FR-CHAT-13 as amended by D-25).

The assistant never performs the action. This renders a control the
member presses, and the press is what acts, so there is no path from
a prompt injection to an action taken. An action outside the
allowlist never reaches this message: the server drops it.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `action` | `string` | string |  | One of: open_scheduler, open_contact_form, open_contributor_request. Clients must ignore anything else rather than guess, so a later server can add one without breaking them. |
| `arg` | `string` | string |  | The single validated argument, or empty. A contact-form category for open_contact_form, a repository name for open_contributor_request, and never set for open_scheduler. |

### MessageFlags

Assistant-side flags on a message.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `outOfScope` | `bool` | boolean |  | The question was outside the assistant's scope and was declined. |
| `noSupport` | `bool` | boolean |  | No supporting material was found; the assistant said so. |
| `degraded` | `bool` | boolean |  | Answered in degraded mode (Q&A bank only, provider unavailable or budget exhausted). |
| `qaMatch` | `bool` | boolean |  | The message was matched directly from the owner's Q&A bank. |

### Message

A message in a conversation.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Message ID. |
| `conversationId` | `string` | string |  | Conversation ID. |
| `role` | [`Message.Role`](#messagerole) | string (enum name) |  | Who wrote it. |
| `text` | `string` | string |  | Text as Markdown (assistant text is sanitized before rendering). |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | Creation time. |
| `citations` | [`Citation`](#citation)[] | array of object |  | Sources (assistant messages only). |
| `rating` | [`Rating`](#rating) | string (enum name) |  | The member's rating, if any (assistant messages only). |
| `flags` | [`MessageFlags`](#messageflags) | object |  | Flags (assistant messages only). |
| `personaVersion` | `string` | string |  | Persona version used (assistant messages only). |
| `proposedAction` | [`ProposedAction`](#proposedaction) | object |  | An action the assistant offered, which the member confirms or ignores; unset when it offered none. |

### CreateConversationRequest

New-conversation request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `contextContentId` | `string` | string | `string: max_len: 128` | Content item the panel was opened from, used for suggestions and retrieval bias; optional. |

### CreateConversationResponse

New conversation with its disclosure message.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversation` | [`Conversation`](#conversation) | object |  | The conversation. |
| `disclosure` | [`Message`](#message) | object |  | The disclosure message (role ASSISTANT). |

### ListConversationsRequest

Conversation list request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### ListConversationsResponse

Conversation list.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversations` | [`Conversation`](#conversation)[] | array of object |  | Conversations, most recent first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

### GetConversationRequest

Conversation detail request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation ID. |

### GetConversationResponse

Conversation with messages.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversation` | [`Conversation`](#conversation) | object |  | The conversation. |
| `messages` | [`Message`](#message)[] | array of object |  | Messages in chronological order. |

### SendMessageRequest

A member message.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation to append to. |
| `text` | `string` | string | `string: min_len: 1 max_len: 4000` | The question or message text. |
| `contextContentId` | `string` | string | `string: max_len: 128` | Content item currently on screen, used to bias retrieval; optional. |

### Usage

Token usage and remaining allowance, sent once per reply.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `inputTokens` | `int32` | number |  | Input tokens for this reply. |
| `outputTokens` | `int32` | number |  | Output tokens for this reply. |
| `remainingToday` | `int32` | number |  | Messages remaining in the member's daily allowance after this one. |
| `remainingInConversation` | `int32` | number |  | Messages remaining in this conversation before a fresh-start prompt. |

### SendMessageResponse

One event in the reply stream.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `start` | [`SendMessageResponse.Start`](#sendmessageresponsestart) | object |  | _(oneof `event`)_ First event: identifiers for the stored member message and the assistant message being generated. |
| `delta` | [`SendMessageResponse.Delta`](#sendmessageresponsedelta) | object |  | _(oneof `event`)_ A fragment of assistant text, in order; concatenate to build the reply. |
| `citations` | [`SendMessageResponse.Citations`](#sendmessageresponsecitations) | object |  | _(oneof `event`)_ Sources for the reply; sent once, after the text. |
| `usage` | [`Usage`](#usage) | object |  | _(oneof `event`)_ Usage and remaining allowance; sent once. |
| `done` | [`SendMessageResponse.Done`](#sendmessageresponsedone) | object |  | _(oneof `event`)_ Final event: the complete assistant message as stored. |

### SendMessageResponse.Start

Stream start.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `userMessageId` | `string` | string |  | ID of the stored member message. |
| `assistantMessageId` | `string` | string |  | ID of the assistant message being generated. |
| `personaVersion` | `string` | string |  | Persona version in effect. |

### SendMessageResponse.Delta

Text fragment.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `text` | `string` | string |  | Text to append. |

### SendMessageResponse.Citations

Citation list.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `citations` | [`Citation`](#citation)[] | array of object |  | Sources in rank order. |

### SendMessageResponse.Done

Stream end.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | [`Message`](#message) | object |  | The complete assistant message. |

### DeleteConversationRequest

Delete request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation ID. |

### DeleteConversationResponse

Empty.

_No fields._

### RateMessageRequest

Rating request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `messageId` | `string` | string | `string: min_len: 1 max_len: 64` | Assistant message ID. |
| `rating` | [`Rating`](#rating) | string (enum name) | `enum: defined_only: true` | The rating; UNSPECIFIED clears a previous rating. |
| `comment` | `string` | string | `string: max_len: 1000` | Optional comment (≤ 1,000 characters). |

### RateMessageResponse

Empty.

_No fields._

### EscalateRequest

Escalation request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation to escalate. |
| `question` | `string` | string | `string: max_len: 4000` | The question for the owner; defaults to the last member message when empty. |
| `messageId` | `string` | string | `string: max_len: 64` | Assistant message that prompted the escalation, when any. |

### Escalation

An escalation record.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Escalation ID. |
| `conversationId` | `string` | string |  | Conversation ID. |
| `status` | [`Escalation.Status`](#escalationstatus) | string (enum name) |  | Current state. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it was raised. |
| `repliedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the owner replied, if they have. |

### EscalateResponse

Escalation result.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `escalation` | [`Escalation`](#escalation) | object |  | The escalation. |

### GetSuggestionsRequest

Suggestions request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `path` | `string` | string | `string: max_len: 256` | Site path of the current page, e.g. `/projects/mdemg`; optional. |
| `contentId` | `string` | string | `string: max_len: 128` | Content item on screen, when any; optional. |

### GetSuggestionsResponse

Suggested questions.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `questions` | `string`[] | array of string |  | Questions, most specific first (at least three). |

### GetQuotaRequest

Empty.

_No fields._

### GetQuotaResponse

Allowance and budget mode.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `dailyLimit` | `int32` | number |  | Daily message limit. |
| `usedToday` | `int32` | number |  | Messages used today. |
| `conversationLimit` | `int32` | number |  | Per-conversation message limit. |
| `budgetMode` | [`GetQuotaResponse.BudgetMode`](#getquotaresponsebudgetmode) | string (enum name) |  | Global budget mode. |

### GetContactOptionsRequest

Empty.

_No fields._

### ContactChannel

A published contact channel.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | `string` | string |  | Channel kind, e.g. `email`, `linkedin`, `scheduling`, `github`. |
| `label` | `string` | string |  | Display label. |
| `url` | `string` | string |  | URL or `mailto:` link. |

### GetContactOptionsResponse

Contact page data.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `availability` | `string` | string |  | Availability statement in the owner's words. |
| `locationPreferences` | `string` | string |  | Location and remote preferences. |
| `channels` | [`ContactChannel`](#contactchannel)[] | array of object |  | Channels in display order. |

### SubmitContactRequest

Contact form.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `subject` | `string` | string | `string: min_len: 1 max_len: 200` | Subject line. |
| `message` | `string` | string | `string: min_len: 1 max_len: 5000` | Message body. |
| `replyChannel` | [`SubmitContactRequest.ReplyChannel`](#submitcontactrequestreplychannel) | string (enum name) | `enum: defined_only: true not_in: 0` | Preferred reply channel. |
| `conversationId` | `string` | string | `string: max_len: 64` | Attach a conversation transcript for context; optional. |
| `category` | [`SupportCategory`](#supportcategory) | string (enum name) | `enum: defined_only: true not_in: 0` | Categorises the message so the owner can filter the support inbox (FR-CNT-22 / FR-ADM-14). Required. |
| `name` | `string` | string | `string: max_len: 200` | Anonymous sender's display name. Required when the request has no session cookie; ignored when it does (the member's name wins). |
| `email` | `string` | string | `string: max_len: 320` | Anonymous sender's email. Required + validated as an email address when the request has no session cookie; ignored when it does. |
| `turnstileToken` | `string` | string | `string: max_len: 4096` | Cloudflare Turnstile response token. Required on anonymous submissions; ignored for members. |
| `hiringRole` | `string` | string | `string: max_len: 200` | The role the sender is considering Roger for. Populated by the form when category = HIRING_INQUIRY; ignored otherwise. Free text; used for admin triage. |
| `hiringJdUrl` | `string` | string | `string: max_len: 2000` | Absolute URL of the job posting. Populated by the form when category = HIRING_INQUIRY; ignored otherwise. Not validated beyond length so recruiters can drop URLs from ATS systems that include tokens / query strings. |
| `hiringTargetStart` | `string` | string | `string: max_len: 100` | Free-text "target start" the sender's hiring cycle is aiming at, e.g. "ASAP", "Q1 2027", "flexible". Populated by the form when category = HIRING_INQUIRY; ignored otherwise. |

### SubmitContactResponse

Submission receipt.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `ticketId` | `string` | string |  | Ticket identifier quoted in the owner's reply. |

### SubmitJdRequest

Submission request. Exactly one of jd_text (paste) OR
jd_pdf_bytes (uploaded PDF) OR jd_text_upload_bytes (uploaded .txt)
is expected — the handler surfaces InvalidArgument otherwise.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jdText` | `string` | string | `string: max_len: 50000` | Full JD text pasted into the textarea. Capped at 50 000 chars by the RPC. Mutually exclusive with the byte-body fields below. |
| `source` | [`JdSource`](#jdsource) | string (enum name) | `enum: defined_only: true not_in: 0` | Where the text came from — the frontend sets this so the backend knows what to record. |
| `roleHint` | `string` | string | `string: max_len: 200` | Optional role / title the member is considering Roger for. Free-form; shown in admin triage and passed as a hint to the requirement and résumé prompts. |
| `employerHint` | `string` | string | `string: max_len: 200` | Optional employer name (e.g. "Anthropic"). Same free-form triage aid and prompt hint as role_hint. |
| `contactEmail` | `string` | string | `string: max_len: 254` | Optional extra address for the finished-review email; the member's account email always receives it. Never surfaced publicly. |
| `applyUrl` | `string` | string | `string: max_len: 2048` | Optional link to apply for the position (http or https). Shown to Roger in the admin triage view; never surfaced publicly. |

### SubmitJdResponse

Submission response. Deliberately minimal — the async scoring +
generation happens in the background; the caller polls
GetJdResult with this id.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string |  | Server-issued id (numeric, stringified over the wire). |
| `status` | [`JdStatus`](#jdstatus) | string (enum name) |  | Current status — usually RECEIVED right at submit time. |
| `message` | `string` | string |  | Fixed human-readable acknowledgement text the /jd-upload page renders back to the caller so the copy stays server-controlled. |
| `resultToken` | `string` | string |  | Secret issued once per submission (hex). Present it on GetJdResult to receive the verdicts and the generated résumé. The submitting member's own session releases the same things without it, so a review can be reopened from /jd-upload/<id> later. |
| `quota` | [`JdQuota`](#jdquota) | object |  | What is left of the member's daily allowance after this submission, so the page can say so rather than let them discover the limit by hitting it. |

### JdQuota

A member's remaining allowance for submitting postings.

Reviewing one posting is close to an hour of inference and the
pipeline runs one at a time, so the allowance is capacity rather than
etiquette. It is reported rather than merely enforced because a limit
someone meets without warning reads as a fault.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number |  | How many postings the member may submit per window. Zero means no limit is in force. |
| `used` | `int32` | number |  | How many the member has already submitted inside the window. |
| `remaining` | `int32` | number |  | How many remain. Zero means the next submission is refused. |
| `windowHours` | `int32` | number |  | How long the window is, in hours; the allowance is rolling rather than aligned to a calendar day. |
| `nextSlotAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the oldest counted submission falls out of the window, which is when the next slot appears. Unset when nothing is counted. |

### GetJdResultRequest

Poll request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | ID from SubmitJdResponse. |
| `resultToken` | `string` | string | `string: max_len: 64` | Token from SubmitJdResponse; optional. Gates the verdicts and the résumé body unless the caller is the submitting member. |

### GetJdResultResponse

Poll response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`JdStatus`](#jdstatus) | string (enum name) |  | Current status. |
| `matchScore` | `double` | number |  | _(oneof `_match_score`)_ Match score in [0, 1] once known; unset before scoring runs and after a failure. |
| `generatedResumeUrl` | `string` | string |  | Download path of the locked résumé PDF, with the result token appended, when status is READY and the caller may see the résumé; empty otherwise. |
| `errorMessage` | `string` | string |  | Human-readable error text when status is FAILED; empty otherwise. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the submission was first accepted. |
| `completedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the terminal state (ready / below_threshold / failed) was reached. Unset while the pipeline is still running. |
| `resumeMarkdown` | `string` | string |  | Generated résumé in markdown, only when status is READY and the caller is the submitting member or carried the result_token. |
| `verdicts` | [`RequirementVerdict`](#requirementverdict)[] | array of object |  | Requirement-by-requirement verdicts behind the score, released to the submitting member (or with the result_token) whatever the outcome, so a below-threshold result shows what was and was not evidenced instead of a bare number (the opposite of an ATS musts-and-misses filter). |
| `metCount` | `int32` | number |  | Number of requirements judged met. |
| `partialCount` | `int32` | number |  | Number judged partially met. |
| `unmetCount` | `int32` | number |  | Number judged not evidenced. |
| `matchThreshold` | `double` | number |  | The résumé gate in force: the "strong" fit band (owner-editable; JD_MATCH_THRESHOLD only seeds it the first time). |
| `progressPct` | `int32` | number |  | Pipeline progress, 0 to 100, while the submission is live; 100 once it has finished. |
| `progressStage` | `string` | string |  | Short human-readable stage ("judging requirement 4 of 12"). |
| `fitCategory` | `string` | string |  | Fit category derived from the score once known: very_strong, strong, possible, weak, very_weak; empty before scoring. |

### JdFitBands

Lower edges of the fit categories; scores below weak are very weak.
Owner-editable from /admin/jd (stored in app_settings, cached briefly
by the api); the strong edge is the résumé gate.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `veryStrong` | `double` | number |  | Very strong at or above this score. |
| `strong` | `double` | number |  | Strong at or above this score; also the résumé gate. |
| `possible` | `double` | number |  | Possible at or above this score. |
| `weak` | `double` | number |  | Weak at or above this score. |

### GetJdReviewConfigRequest

Config request; the member is the session.

_No fields._

### GetJdReviewConfigResponse

The bands in force.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `bands` | [`JdFitBands`](#jdfitbands) | object |  | Current bands. |
| `quota` | [`JdQuota`](#jdquota) | object |  | The signed-in member's allowance as it stands now, so the upload page can state the limit before anyone spends it. |

### ListMySubmissionsRequest

List request; the member is the session.

_No fields._

### ListMySubmissionsResponse

The member's submissions, newest first.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissions` | [`MySubmission`](#mysubmission)[] | array of object |  | Up to 100 rows. |

### MySubmission

One of the member's own submissions, enough to pick it from a list.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Submission id (numeric, stringified); opens /jd-upload/<id>. |
| `status` | [`JdStatus`](#jdstatus) | string (enum name) |  | Current status. |
| `matchScore` | `double` | number |  | _(oneof `_match_score`)_ Match score once known. |
| `roleHint` | `string` | string |  | Role the member typed on the form, if any. |
| `employerHint` | `string` | string |  | Employer the member typed on the form, if any. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it was submitted. |
| `completedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it reached a terminal state; unset while running. |
| `hasResume` | `bool` | boolean |  | True when a tailored résumé exists for it. |
| `fitCategory` | `string` | string |  | Fit category once scored (see GetJdResultResponse.fit_category). |
| `progressPct` | `int32` | number |  | Pipeline progress, 0 to 100. |

### RequirementVerdict

One requirement the reviewer extracted from the posting and the
verdict it reached from the candidate's records.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Requirement id within the submission (r1, r2, ...). |
| `text` | `string` | string |  | The requirement in the posting's own words. |
| `category` | `string` | string |  | "must" or "nice", as the posting stated it. |
| `weight` | `int32` | number |  | Weight 1 to 3 used in the score. Must items always count in the denominator; nice items only when evidenced. |
| `verdict` | `string` | string |  | "met", "partial" or "unmet" (scored 1, 0.5 and 0 in code). |
| `rationale` | `string` | string |  | One-sentence reason, grounded in the evidence the judge saw. |

### MemberCounts

Activity counts for a member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `views` | `int32` | number |  | Content views. |
| `downloads` | `int32` | number |  | Downloads. |
| `chatMessages` | `int32` | number |  | Assistant messages sent. |
| `escalations` | `int32` | number |  | Escalations raised. |
| `saved` | `int32` | number |  | Saved items. |

### MemberRecord

A member as the owner sees it.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member. |
| `counts` | [`MemberCounts`](#membercounts) | object |  | Activity counts. |
| `firstSeenAt` | `Timestamp` | string (RFC 3339, UTC) |  | First sign-in. |
| `noteCount` | `int32` | number |  | Number of admin notes. |

### ListMembersRequest

Member list filters.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: max_len: 200` | Case-insensitive match on name, email, or organization. |
| `status` | [`MemberStatus`](#memberstatus) | string (enum name) | `enum: defined_only: true` | Restrict to one status; unspecified returns all. |
| `trackId` | `string` | string | `string: max_len: 64` | Restrict to members interested in this track. |
| `sort` | [`ListMembersRequest.Sort`](#listmembersrequestsort) | string (enum name) | `enum: defined_only: true` | Sort order. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### ListMembersResponse

Member list.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `members` | [`MemberRecord`](#memberrecord)[] | array of object |  | Members. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

### GetMemberRequest

Member detail request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID. |

### AdminNote

A private admin note.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Note ID. |
| `text` | `string` | string |  | Note text. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it was written. |

### NotificationDelivery

One audited email attempt for a member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Delivery row id. |
| `kind` | `string` | string |  | Slug of the mail: user_approved, user_declined, welcome_whitelist, verify_email, password_reset, expiry_warn, expired, user_auto_declined. |
| `recipient` | `string` | string |  | Address the mail was sent to. |
| `provider` | `string` | string |  | Provider that handled it (resend / smtp). |
| `triggeredBy` | `string` | string |  | "system" for automatic sends; "admin:<id>" for console resends. |
| `durationMs` | `int64` | string (decimal) |  | Provider round-trip in milliseconds. |
| `ok` | `bool` | boolean |  | True when the provider accepted the message. |
| `error` | `string` | string |  | Truncated provider error when ok is false; empty otherwise. |
| `sentAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the attempt happened. |

### GetMemberResponse

Member detail.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | The member. |
| `recentActivity` | [`ActivityEvent`](#activityevent)[] | array of object |  | Most recent 50 activity events, newest first. |
| `conversations` | [`Conversation`](#conversation)[] | array of object |  | Conversations, most recent first. |
| `notes` | [`AdminNote`](#adminnote)[] | array of object |  | Admin notes, newest first. |
| `deliveries` | [`NotificationDelivery`](#notificationdelivery)[] | array of object |  | Most recent 20 email attempts, newest first. |

### ResendNotificationRequest

Resend-notification request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID. |
| `kind` | `string` | string | `string: min_len: 1 max_len: 40` | Which mail to re-fire. Only `user_approved` (member must be active) and `user_declined` (member must be declined) are resendable; token-bearing mails (verify, reset) are not. |

### ResendNotificationResponse

Resend-notification response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `delivery` | [`NotificationDelivery`](#notificationdelivery) | object |  | The audited attempt, including any provider error. |

### AddMemberNoteRequest

Note request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID. |
| `text` | `string` | string | `string: min_len: 1 max_len: 4000` | Note text. |

### AddMemberNoteResponse

The stored note.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `note` | [`AdminNote`](#adminnote) | object |  | The note. |

### SetMemberStatusRequest

Status change.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID. |
| `status` | [`MemberStatus`](#memberstatus) | string (enum name) | `enum: in: 2 in: 4` | New status: ACTIVE (approve or re-enable) or DISABLED. |
| `reason` | `string` | string | `string: max_len: 1000` | Reason, recorded in the audit log. |

### SetMemberStatusResponse

Updated member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | The member after the change. |

### GetReviewQueueRequest

Review queue filters.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`ReviewKind`](#reviewkind) | string (enum name) | `enum: defined_only: true` | Restrict to one kind. |
| `status` | [`ReviewStatus`](#reviewstatus) | string (enum name) | `enum: defined_only: true` | Restrict to one status; unspecified returns OPEN items. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### ReviewItem

A review item.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Review item ID. |
| `kind` | [`ReviewKind`](#reviewkind) | string (enum name) |  | Kind. |
| `status` | [`ReviewStatus`](#reviewstatus) | string (enum name) |  | Status. |
| `member` | [`Me`](#me) | object |  | The member. |
| `conversationId` | `string` | string |  | Conversation ID. |
| `messageId` | `string` | string |  | Message ID that triggered the item. |
| `question` | `string` | string |  | The member's question. |
| `answerExcerpt` | `string` | string |  | The assistant's answer, truncated to 500 characters. |
| `comment` | `string` | string |  | Member comment, for negative feedback. |
| `escalationId` | `string` | string |  | Escalation ID, for escalations. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the item was created. |

### GetReviewQueueResponse

Review queue page.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`ReviewItem`](#reviewitem)[] | array of object |  | Items, oldest open first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |
| `openCount` | `int32` | number |  | Open items in total. |

### ResolveReviewItemRequest

Resolution request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `reviewItemId` | `string` | string | `string: min_len: 1 max_len: 64` | Review item ID. |
| `resolution` | [`ReviewStatus`](#reviewstatus) | string (enum name) | `enum: in: 2 in: 3` | Resolution: RESOLVED or CONVERTED_TO_QA. |
| `note` | `string` | string | `string: max_len: 1000` | Note for the audit log; optional. |

### ResolveReviewItemResponse

Empty.

_No fields._

### ReplyEscalationRequest

Escalation reply.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `escalationId` | `string` | string | `string: min_len: 1 max_len: 64` | Escalation ID. |
| `text` | `string` | string | `string: min_len: 1 max_len: 8000` | Reply text (Markdown). |
| `notifyMember` | `bool` | boolean |  | Also email the member (default true when unset in the UI). |

### ReplyEscalationResponse

The stored reply.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | [`Message`](#message) | object |  | The OWNER message appended to the conversation. |

### GetMemberConversationRequest

Any member's conversation.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `conversationId` | `string` | string | `string: min_len: 1 max_len: 64` | Conversation ID. |

### GetMemberConversationResponse

Conversation with messages.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`Me`](#me) | object |  | The member. |
| `conversation` | [`Conversation`](#conversation) | object |  | The conversation. |
| `messages` | [`Message`](#message)[] | array of object |  | Messages in chronological order, with citations. |

### GetCorpusStatusRequest

Empty.

_No fields._

### GetCorpusStatusResponse

Corpus statistics.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `documents` | `int32` | number |  | Indexed documents. |
| `chunks` | `int32` | number |  | Chunks with embeddings. |
| `qaEntries` | `int32` | number |  | Q&A-bank entries. |
| `documentsByType` | map<`string`, `int32`> | object of number |  | Documents by content type, keyed by the enum name (e.g. `CONTENT_TYPE_ARTICLE`). |
| `privateDocuments` | `int32` | number |  | Private-corpus documents (not linked to site content). |
| `lastIngestAt` | `Timestamp` | string (RFC 3339, UTC) |  | Last successful ingest. |
| `embeddingModel` | `string` | string |  | Embedding model identifier. |
| `embeddingDimensions` | `int32` | number |  | Embedding dimensions. |
| `personaVersion` | `string` | string |  | Active persona version. |

### TestRetrievalRequest

Retrieval test.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: min_len: 1 max_len: 4000` | The question to retrieve for. |
| `topK` | `int32` | number | `int32: lte: 50 gte: 0` | Chunks to return, 1–50; zero selects the assistant's default. |
| `rerank` | `bool` | boolean |  | Apply the sidecar reranker after hybrid retrieval. |

### RetrievalHit

One retrieved chunk.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `chunkId` | `string` | string |  | Chunk ID. |
| `contentId` | `string` | string |  | Content item, when the chunk is site content. |
| `title` | `string` | string |  | Document title. |
| `heading` | `string` | string |  | Heading path within the document. |
| `score` | `float` | number |  | Fused score. |
| `text` | `string` | string |  | Chunk text. |
| `preRerankRank` | `int32` | number |  | Rank before reranking (1-based). |

### TestRetrievalResponse

Retrieval test result.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `hits` | [`RetrievalHit`](#retrievalhit)[] | array of object |  | Chunks in rank order. |
| `qaMatch` | [`ContentSummary`](#contentsummary) | object |  | Matching Q&A-bank entry, when the question matched one above threshold. |
| `qaSimilarity` | `float` | number |  | Similarity of the Q&A match. |
| `latencyMs` | `int32` | number |  | Retrieval latency in milliseconds. |

### RunJobRequest

Job start request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`JobKind`](#jobkind) | string (enum name) | `enum: defined_only: true not_in: 0` | Which job. |
| `sourceKind` | `string` | string | `string: max_len: 40` | For the corpus reindex jobs: restrict the walk to one source_kind; empty walks every kind. |

### RunJobResponse

Job handle.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string |  | Job ID. |

### GetJobRequest

Job status request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string | `string: min_len: 1 max_len: 64` | Job ID. |

### GetJobResponse

Job status.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string |  | Job ID. |
| `kind` | [`JobKind`](#jobkind) | string (enum name) |  | Which job. |
| `status` | [`JobStatus`](#jobstatus) | string (enum name) |  | Status. |
| `progressPct` | `int32` | number |  | Progress in percent, when the job reports it. |
| `startedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Start time. |
| `finishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Finish time. |
| `summary` | `string` | string |  | Human-readable summary or error. |

### GetPersonaRequest

Empty.

_No fields._

### PersonaVersion

A persona version.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `version` | `string` | string |  | Version label, e.g. `v3`. |
| `promptSha256` | `string` | string |  | SHA-256 of the system prompt, as pinned by the ULTS spec. |
| `styleNotes` | `string` | string |  | Style notes summary. |
| `active` | `bool` | boolean |  | True for the live version. |
| `activatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it became active. |
| `evalPassRate` | `float` | number |  | UVTS quick-profile pass rate at activation, in percent. |

### GetPersonaResponse

Persona versions.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `active` | [`PersonaVersion`](#personaversion) | object |  | The live version. |
| `history` | [`PersonaVersion`](#personaversion)[] | array of object |  | Previous versions, newest first. |

### GetAnalyticsRequest

Analytics request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `from` | `Timestamp` | string (RFC 3339, UTC) | `required: true` | Range start (inclusive). |
| `to` | `Timestamp` | string (RFC 3339, UTC) | `required: true` | Range end (exclusive). |

### DailyCount

A daily count.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `day` | `Timestamp` | string (RFC 3339, UTC) |  | Day (UTC midnight). |
| `count` | `int32` | number |  | Count. |

### RankedContent

A ranked content item.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `item` | [`ContentSummary`](#contentsummary) | object |  | The item. |
| `count` | `int32` | number |  | Views or downloads in the range. |

### RankedQuestion

A ranked question.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `question` | `string` | string |  | Representative question text. |
| `count` | `int32` | number |  | Times asked (clustered by similarity). |
| `noSupportPct` | `float` | number |  | Share of answers flagged no-support, in percent. |

### AssistantUsage

Assistant usage in the range.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `messages` | `int32` | number |  | Messages sent. |
| `conversations` | `int32` | number |  | Conversations started. |
| `inputTokens` | `int64` | string (decimal) |  | Input tokens. |
| `outputTokens` | `int64` | string (decimal) |  | Output tokens. |
| `costUsd` | `double` | number |  | Cost in US dollars. |
| `budgetUsedPct` | `float` | number |  | Share of the monthly budget consumed, in percent. |
| `ratingsUp` | `int32` | number |  | Positive ratings. |
| `ratingsDown` | `int32` | number |  | Negative ratings. |

### GetAnalyticsResponse

Analytics for a range.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `registrations` | [`DailyCount`](#dailycount)[] | array of object |  | Registrations per day. |
| `verifications` | [`DailyCount`](#dailycount)[] | array of object |  | Verifications per day. |
| `activeMembers` | [`DailyCount`](#dailycount)[] | array of object |  | Active members per day (any activity). |
| `questionnairesCompleted` | [`DailyCount`](#dailycount)[] | array of object |  | Members who completed the questionnaire, per day. |
| `topContent` | [`RankedContent`](#rankedcontent)[] | array of object |  | Most viewed content. |
| `topDownloads` | [`RankedContent`](#rankedcontent)[] | array of object |  | Most downloaded files. |
| `topQuestions` | [`RankedQuestion`](#rankedquestion)[] | array of object |  | Most asked questions. |
| `assistant` | [`AssistantUsage`](#assistantusage) | object |  | Assistant usage and cost. |
| `membersByTrack` | map<`string`, `int32`> | object of number |  | Members by track interest, keyed by track ID. |

### GetAuditRequest

Audit filters.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `actorId` | `string` | string | `string: max_len: 64` | Restrict to one actor (member ID); empty for all. |
| `actionPrefix` | `string` | string | `string: max_len: 64` | Restrict to an action prefix, e.g. `auth.` or `admin.`. |
| `from` | `Timestamp` | string (RFC 3339, UTC) |  | Range start; optional. |
| `to` | `Timestamp` | string (RFC 3339, UTC) |  | Range end; optional. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### AuditEntry

An audit-log entry.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Entry ID. |
| `actorId` | `string` | string |  | Actor member ID; empty for system actions. |
| `action` | `string` | string |  | Action name, e.g. `auth.login.failed`, `admin.member.note_added`. |
| `targetType` | `string` | string |  | Target type, e.g. `member`, `conversation`. |
| `targetId` | `string` | string |  | Target ID. |
| `payloadJson` | `string` | string |  | Details as JSON. |
| `occurredAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it happened. |

### GetAuditResponse

Audit page.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `entries` | [`AuditEntry`](#auditentry)[] | array of object |  | Entries, newest first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

### SupportMessage

One row of the support_messages table, as the owner sees it.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Numeric ID (bigserial); string over the wire so callers don't hard-code JSON number precision. |
| `ticketId` | `string` | string |  | Ticket ID (public reference shown in the "we got your message" email). |
| `category` | [`SupportCategory`](#supportcategory) | string (enum name) |  | Category the sender picked on the contact form. |
| `status` | [`SupportStatus`](#supportstatus) | string (enum name) |  | Open vs resolved state. |
| `subject` | `string` | string |  | Subject line the sender wrote. |
| `body` | `string` | string |  | Message body, plain text. |
| `senderName` | `string` | string |  | Sender name (as submitted or copied from the user row for members). |
| `senderEmail` | `string` | string |  | Sender email (as submitted or copied from the user row for members). |
| `userId` | `string` | string |  | The signed-in user_id if the sender was a member at submit time, empty for anonymous submissions. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the message was received. |
| `updatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the row last changed (initially = created_at; bumps on status changes). |
| `resolvedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Set only for resolved messages. |
| `hiringRole` | `string` | string |  | Role the sender is considering Roger for; populated only when category is HIRING_INQUIRY. |
| `hiringJdUrl` | `string` | string |  | URL of the job posting; populated only when category is HIRING_INQUIRY. |
| `hiringTargetStart` | `string` | string |  | Free-text target start date; populated only when category is HIRING_INQUIRY. |

### ListContactMessagesRequest

Contact-message list request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: max_len: 200` | Case-insensitive substring match against subject / body / sender. |
| `status` | [`SupportStatus`](#supportstatus) | string (enum name) | `enum: defined_only: true` | Restrict to one status; unspecified returns all. |
| `category` | [`SupportCategory`](#supportcategory) | string (enum name) | `enum: defined_only: true` | Restrict to one category; unspecified returns all. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### ListContactMessagesResponse

Contact-message list response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `messages` | [`SupportMessage`](#supportmessage)[] | array of object |  | Messages matching the filter, newest first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |
| `openCount` | `int32` | number |  | Unfiltered count of messages currently in the "open" state. |
| `resolvedCount` | `int32` | number |  | Unfiltered count of messages currently in the "resolved" state. |

### ResolveContactMessageRequest

Mark-resolved / re-open request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 32` | ID from SupportMessage.id. |
| `status` | [`SupportStatus`](#supportstatus) | string (enum name) | `enum: defined_only: true` | Target status. UNSPECIFIED defaults to RESOLVED. |

### ResolveContactMessageResponse

Updated message.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | [`SupportMessage`](#supportmessage) | object |  | The message row after the status flip. |

### ApproveRegistrationRequest

Approve-registration request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID from MemberRecord.me.id. |
| `reason` | `string` | string | `string: max_len: 1000` | Optional free-text reason recorded in the audit log. |

### ApproveRegistrationResponse

Approve-registration response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | Updated member record. |

### DeclineRegistrationRequest

Decline-registration request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID from MemberRecord.me.id. |
| `reason` | `string` | string | `string: max_len: 1000` | Optional free-text reason recorded in the audit log. |

### DeclineRegistrationResponse

Decline-registration response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | Updated member record. |

### ExtendAccessRequest

Extend-access request. Set exactly one of extend_days /
new_expires_at / permanent to specify the new expiry.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memberId` | `string` | string | `string: min_len: 1 max_len: 64` | Member ID from MemberRecord.me.id. |
| `extendDays` | `int32` | number | `int32: lte: 3650 gte: 0` | Add this many days to the current expires_at (or, if the member has no expires_at set, from now). 1..3650. |
| `newExpiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | Set expires_at to this exact moment. |
| `permanent` | `bool` | boolean |  | Make access permanent (clears expires_at). |
| `reason` | `string` | string | `string: max_len: 1000` | Optional free-text reason recorded in the audit log. |

### ExtendAccessResponse

Extend-access response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `member` | [`MemberRecord`](#memberrecord) | object |  | Updated member record. |

### DbColumn

One column of a database table.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string |  | Column name. |
| `dataType` | `string` | string |  | SQL type as reported by information_schema (e.g. "text", "bigint", "timestamp with time zone"). |
| `nullable` | `bool` | boolean |  | Whether the column is nullable. |

### DbTable

One table in the public schema.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string |  | Table name. |
| `columns` | [`DbColumn`](#dbcolumn)[] | array of object |  | Columns, ordered as declared. |
| `approxRowCount` | `int64` | string (decimal) |  | Approximate row count from pg_class.reltuples (updated by ANALYZE; may be stale — the console labels it as such). |

### ListDbTablesRequest

List-tables request (empty for now; a future revision might
add filters).

_No fields._

### ListDbTablesResponse

List-tables response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `tables` | [`DbTable`](#dbtable)[] | array of object |  | Public-schema tables, ordered by name. |

### RunDbQueryRequest

Run-query request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sql` | `string` | string | `string: min_len: 1 max_len: 4000` | SQL statement to execute. MVP: only SELECT is accepted. |
| `timeoutMs` | `int32` | number |  | Statement timeout in milliseconds; server-clamped to [100, 10000]. |
| `sort` | `string` | string | `string: max_len: 64` | Optional column of the result to sort by (informational only — the client can reorder locally; the server passes it back so a caller can round-trip UI state). |

### DbRow

One row of results, as a list of stringified cell values in the
same order as the columns array on the response. NULLs come across
as an empty string with a companion `null_mask` bit set on the row
(see below).

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `cells` | `string`[] | array of string |  | Cell values, stringified. Callers must not attempt to parse them as JSON; the type is columns[i].data_type on the response. |
| `nullMask` | `uint64` | string (decimal) |  | Bitmask, LSB = column 0. A set bit means the corresponding cell is NULL and its stringified value should be ignored. |

### RunDbQueryResponse

Run-query response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `columns` | `string`[] | array of string |  | Column names (order matches DbRow.cells). |
| `columnTypes` | `string`[] | array of string |  | SQL types (order matches columns). |
| `rows` | [`DbRow`](#dbrow)[] | array of object |  | Rows, in whatever order the query returned them. |
| `truncated` | `bool` | boolean |  | True when the result was cut off at the server-side row cap. |
| `rowCount` | `int32` | number |  | Number of rows returned (before truncation, if applicable — matches len(rows) when truncated is false). |
| `elapsedMs` | `int32` | number |  | Milliseconds the query took on the server, wall-clock. |

### AccessGrant

One row of the access_grants table.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Numeric ID (bigserial); string over the wire so callers don't hard-code JSON number precision. |
| `email` | `string` | string |  | Whitelisted email address (citext in the DB). |
| `defaultTtl` | [`GrantTTL`](#grantttl) | string (enum name) |  | TTL granted on signup. |
| `notes` | `string` | string |  | Free-text admin notes (why this email was whitelisted, etc.). |
| `entryExpiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the whitelist entry itself lapses; unset means never. |
| `createdBy` | `string` | string |  | Admin user_id that created the entry; empty for seeded rows. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the entry was created. |
| `updatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the entry was last updated. |

### ListAccessGrantsRequest

List-grants request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: max_len: 200` | Case-insensitive substring match against email or notes. |

### ListAccessGrantsResponse

List-grants response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `grants` | [`AccessGrant`](#accessgrant)[] | array of object |  | Whitelist entries, newest first. |
| `activeCount` | `int32` | number |  | Count of entries currently active (entry_expires_at unset or in the future). |
| `expiredCount` | `int32` | number |  | Count of entries whose entry_expires_at has passed. |

### UpsertAccessGrantRequest

Upsert-grant request. If a row with this email already exists it
is updated in place; otherwise a new row is created.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `email` | `string` | string | `string: min_len: 3 max_len: 254 email: true` | Email to whitelist. Lower-cased server-side. |
| `defaultTtl` | [`GrantTTL`](#grantttl) | string (enum name) | `enum: defined_only: true not_in: 0` | TTL that new registrations get on approval. |
| `notes` | `string` | string | `string: max_len: 1000` | Free-text admin notes. |
| `entryExpiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | When this whitelist entry itself should lapse; unset = never. |

### UpsertAccessGrantResponse

Upsert-grant response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `grant` | [`AccessGrant`](#accessgrant) | object |  | The stored entry (with fields filled in by the server). |
| `created` | `bool` | boolean |  | True when the request created a new row; false when it updated an existing one. |

### DeleteAccessGrantRequest

Delete-grant request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 32` | ID from AccessGrant.id. |

### DeleteAccessGrantResponse

Delete-grant response.

_No fields._

### SavedQuery

One saved SQL statement on the /admin/db page, scoped to a single
admin. Names are unique per owner but not globally.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Numeric id (bigserial); string over the wire to avoid JSON number-precision drift. |
| `name` | `string` | string |  | The admin's chosen label, shown in the dropdown. |
| `sql` | `string` | string |  | The SELECT text. Same 4000-char cap as RunDbQuery.sql. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the row was first saved. |
| `updatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the row was last overwritten via upsert. |

### ListSavedQueriesRequest

List-saved-queries request. No filters yet — the response is
naturally scoped to the caller and expected to be small.

_No fields._

### ListSavedQueriesResponse

List-saved-queries response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `queries` | [`SavedQuery`](#savedquery)[] | array of object |  | The caller's saved queries, ordered by name (case-insensitive). |

### UpsertSavedQueryRequest

Upsert-saved-query request. A row with (owner, name) is created
if it doesn't exist and overwritten if it does.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string | `string: min_len: 1 max_len: 80` | Label to save under. Trimmed server-side; empty is rejected. |
| `sql` | `string` | string | `string: min_len: 1 max_len: 4000` | The SQL text. Enforcement (SELECT-only, single statement) is applied at execute time by RunDbQuery, not on save — so an admin can save a draft and finish it later. |

### UpsertSavedQueryResponse

Upsert-saved-query response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | [`SavedQuery`](#savedquery) | object |  | The stored row (fields populated by the server). |
| `created` | `bool` | boolean |  | True when the request created a new row; false when it overwrote an existing (owner, name) tuple. |

### DeleteSavedQueryRequest

Delete-saved-query request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 32` | ID from SavedQuery.id. |

### DeleteSavedQueryResponse

Delete-saved-query response.

_No fields._

### MemberActivitySummary

One row of the /admin/activity aggregate view: per-member counts
+ the last recorded event. `last_event_at` is unset for members
with no recorded activity yet.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `userId` | `string` | string |  | Member id (numeric, stringified over the wire). |
| `name` | `string` | string |  | Display name. |
| `email` | `string` | string |  | Email address. |
| `status` | [`MemberStatus`](#memberstatus) | string (enum name) |  | Current lifecycle state. |
| `totalSessions` | `int32` | number |  | Number of sessions ever created for this member. |
| `totalActiveSecs` | `int64` | string (decimal) |  | Sum of session wall-clock time in seconds, approximated as (LEAST(revoked_at, last_active_at, now()) − created_at) per session. |
| `askRogerCount` | `int32` | number |  | Number of `chat` events (Ask Roger interactions). |
| `totalEvents` | `int32` | number |  | Total number of activity_events rows for this member. |
| `lastKind` | `string` | string |  | Kind of the most-recent event (matching ActivityEvent.Kind values as a slug: "view", "login", …). Empty for members with no events yet. |
| `lastEventAt` | `Timestamp` | string (RFC 3339, UTC) |  | Timestamp of the most-recent event; unset when no events. |

### ListMemberActivityRequest

List-member-activity request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sort` | [`ActivitySort`](#activitysort) | string (enum name) | `enum: defined_only: true` | Which column to sort by. |

### ListMemberActivityResponse

List-member-activity response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `members` | [`MemberActivitySummary`](#memberactivitysummary)[] | array of object |  | One row per member (up to 500). |

### IngestCorpusTextRequest

Ingest one document into the Ask Roger corpus.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sourceKind` | `string` | string | `string: min_len: 1 max_len: 40` | Where the text came from: 'article', 'resume', 'career_note', 'adr', 'other'. Kept as free text (not enum) so a new source kind is just a new value at the ingest form, not a proto edit. |
| `sourcePath` | `string` | string | `string: min_len: 1 max_len: 200` | Stable per-source_kind key. Filesystem sources pass the relative path; pasted documents pass a slug like `paste/2026-09-21-notes`. |
| `title` | `string` | string | `string: max_len: 300` | Human-readable title. Empty falls back to the first non-blank line of the body (with a leading `#` stripped) at ingest time. |
| `body` | `string` | string | `string: min_len: 1 max_len: 204800` | The document text, post-front-matter for markdown. Capped at 200 KiB so the form doesn't paste in an unbounded blob. |
| `visibility` | `string` | string | `string: max_len: 16` | `public` (default) or `corpus_only`. |

### IngestCorpusTextResponse

Ingest response — mirrors ingest.IngestResult.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `documentId` | `string` | string |  | ID of the stored corpus_documents row. |
| `chunksInserted` | `int32` | number |  | Number of chunks written this run (0 if skipped). |
| `chunksEmbedded` | `int32` | number |  | Number of chunks successfully embedded this run. |
| `skipped` | `bool` | boolean |  | True when the body's content hash matched the stored row and no chunk / embed work ran. |
| `chunkerName` | `string` | string |  | Stable identifier of the chunker version used (e.g. "paragraph.v1"). |
| `embedderModel` | `string` | string |  | Embedder model reported by the sidecar (e.g. "stub", "ollama"). |

### ListCorpusDocumentsRequest

List-corpus-documents request. No filters yet — corpus is small
enough that the admin surface returns everything.

_No fields._

### CorpusDocumentRow

One row in the corpus with a computed chunk count.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | ID of the corpus_documents row. |
| `sourceKind` | `string` | string |  | Source kind slug. |
| `sourcePath` | `string` | string |  | Stable per-source_kind key. |
| `title` | `string` | string |  | Human-readable title. |
| `chunkCount` | `int32` | number |  | Count of chunk rows referencing this document. |
| `embeddedCount` | `int32` | number |  | Count of chunks that have an embedding populated. |
| `ingestedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the row was first written. |
| `updatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When any field changed (title / hash / meta). |
| `visibility` | `string` | string |  | `public` (citable on the site) or `corpus_only` (informs answers, never quoted or named). |

### ListCorpusDocumentsResponse

List-corpus-documents response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `documents` | [`CorpusDocumentRow`](#corpusdocumentrow)[] | array of object |  | All documents, newest first. |
| `totalDocuments` | `int32` | number |  | Aggregate document count across the whole corpus (rendered as a header chip on /admin/corpus). |
| `totalChunks` | `int32` | number |  | Aggregate chunk count across every stored document. |
| `totalEmbedded` | `int32` | number |  | Aggregate count of chunks that have an embedding populated. |
| `totalPublic` | `int32` | number |  | Documents with visibility=public. |
| `totalCorpusOnly` | `int32` | number |  | Documents with visibility=corpus_only. |
| `embedderCounts` | [`EmbedderCount`](#embeddercount)[] | array of object |  | Chunk counts grouped by the embedder that produced their vector. |

### EmbedderCount

One row of the per-embedder chunk breakdown.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `model` | `string` | string |  | Embedder name as reported by the sidecar (e.g. `stub`, `ollama:nomic-embed-text`); empty for chunks with no vector yet, `untracked` for vectors written before per-chunk tracking existed. |
| `count` | `int32` | number |  | Number of chunks. |

### SweepCorpusEmbeddingsRequest

Sweep-corpus-embeddings request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `maxChunks` | `int32` | number | `int32: lte: 5000 gte: 0` | Upper bound on chunks to (re)embed in this call. 0 → 512. |

### SweepCorpusEmbeddingsResponse

Sweep-corpus-embeddings response — mirrors ingest.SweepResult.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `model` | `string` | string |  | Embedder the sidecar is currently serving; every chunk is converged onto this. |
| `considered` | `int32` | number |  | Chunks selected as stale this call. |
| `embedded` | `int32` | number |  | Chunks successfully re-embedded this call. |
| `failed` | `int32` | number |  | Chunks that failed (left stale for the next sweep). |
| `remaining` | `int32` | number |  | Chunks still stale after this call; zero means converged. |

### GetJdSubmissionRequest

Get-jd-submission request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | Submission id (numeric, stringified). |

### GetJdSubmissionResponse

Get-jd-submission response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `row` | [`JdSubmissionRow`](#jdsubmissionrow) | object |  | The list row for this submission (status, scores, hints). |
| `jdText` | `string` | string |  | Full JD text as submitted. |
| `assessmentJson` | `string` | string |  | Assessment derivation as JSON (requirements, evidence chunk ids, per-requirement verdicts, prompt versions, score formula); empty before scoring or when the assessor was not wired. |
| `resumeMarkdown` | `string` | string |  | Generated résumé in markdown; empty until generation lands. |
| `llmModel` | `string` | string |  | Model that produced the résumé. |
| `promptId` | `string` | string |  | Prompt id that produced the résumé. |
| `promptVersion` | `int32` | number |  | Prompt version that produced the résumé. |
| `downloadUrl` | `string` | string |  | Download path for the locked PDF, including the submission's result token, when a PDF was rendered; empty otherwise. Admin-only by virtue of this RPC's auth level. |
| `runs` | [`JdRun`](#jdrun)[] | array of object |  | Every pipeline run for this submission, newest attempt first. The fields above reflect only the most recent run, because a re-score overwrites them; these rows survive it, so a superseded verdict can still be read and compared with the one that replaced it. |
| `outcome` | [`JdOutcome`](#jdoutcome) | object |  | What happened in the world after this review; unset until recorded. |
| `feedback` | [`JdFeedback`](#jdfeedback)[] | array of object |  | Judgments recorded about this posting's output, newest first. |

### JdOutcome

What happened after a review. One per posting, revised in place.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | `string` | string |  | not_pursued | applied | screening | interview | offer | rejected | no_response | withdrew. |
| `decidedOn` | `string` | string |  | The day that status became true, as YYYY-MM-DD; empty when unknown. |
| `note` | `string` | string |  | Free note. |
| `updatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the record was last revised. |

### JdFeedback

One person's judgment of one run's output.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `runId` | `string` | string |  | The run being judged; empty for a posting that predates run records. |
| `source` | `string` | string |  | owner | submitter. Never averaged together. |
| `target` | `string` | string |  | score | resume. |
| `rating` | `string` | string |  | For score: accurate | too_generous | too_harsh | unusable. For resume: would_send | needs_edits | wrong. |
| `note` | `string` | string |  | Free note. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it was recorded. |

### SetJdOutcomeRequest

Set-jd-outcome request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | Submission id (numeric, stringified). |
| `status` | `string` | string | `string: min_len: 1 max_len: 32` | One of the outcome statuses; validated server-side. |
| `decidedOn` | `string` | string | `string: max_len: 10` | YYYY-MM-DD, or empty when the day is unknown. |
| `note` | `string` | string | `string: max_len: 4000` | Free note, trimmed to 4000 characters. |

### SetJdOutcomeResponse

Set-jd-outcome response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `outcome` | [`JdOutcome`](#jdoutcome) | object |  | The stored outcome. |

### RecordJdFeedbackRequest

Record-jd-feedback request. The owner's judgment of a run's output.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | Submission id (numeric, stringified). |
| `runId` | `string` | string | `string: max_len: 64` | Run being judged; empty attaches the judgment to the newest run. |
| `target` | `string` | string | `string: min_len: 1 max_len: 16` | score | resume. |
| `rating` | `string` | string | `string: min_len: 1 max_len: 32` | Rating from the vocabulary for that target; validated server-side. |
| `note` | `string` | string | `string: max_len: 4000` | Free note, trimmed to 4000 characters. |

### RecordJdFeedbackResponse

Record-jd-feedback response.

_No fields._

### GoldenPosting

One job description with a stated expectation (data layer D4). The
expectation is which side of the gate it belongs on, not a score:
a score is model-dependent and would have to be rewritten on every
change, and that rewriting is how a regression hides.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id. |
| `name` | `string` | string |  | Short unique label used in summaries. |
| `jdText` | `string` | string |  | The posting itself. |
| `roleHint` | `string` | string |  | Role hint passed to the pipeline, as a submitter would give it. |
| `employerHint` | `string` | string |  | Employer hint passed to the pipeline. |
| `expectedGate` | `string` | string |  | above | below: the side of the gate this posting belongs on. |
| `expectedBand` | `string` | string |  | Advisory band; not asserted, because band edges are tuned. |
| `note` | `string` | string |  | Why it is in the set and what it is meant to catch. |
| `active` | `bool` | boolean |  | Retired postings stay for history and are skipped by new runs. |
| `selection` | `string` | string |  | chosen (the owner picked it) or random (drawn without reading it first). Reported apart, because a gate clean on chosen postings and noisy on random ones is the finding that matters. |
| `source` | `string` | string |  | Where the posting came from. |
| `lastScore` | `double` | number |  | _(oneof `_last_score`)_ Score from the most recent evaluation that included it. |
| `lastPassed` | `bool` | boolean |  | Whether that evaluation put it on the expected side. |
| `lastEvalAt` | `Timestamp` | string (RFC 3339, UTC) |  | When that evaluation scored it. |

### ListGoldenPostingsRequest

List-golden-postings request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `activeOnly` | `bool` | boolean |  | True omits retired postings. |

### ListGoldenPostingsResponse

List-golden-postings response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `postings` | [`GoldenPosting`](#goldenposting)[] | array of object |  | The set, expected-above first then by name. |

### UpsertGoldenPostingRequest

Upsert-golden-posting request. Keyed by name within the tenant.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string | `string: min_len: 1 max_len: 80` | Short unique label. |
| `jdText` | `string` | string | `string: min_len: 40 max_len: 50000` | The posting text. |
| `roleHint` | `string` | string | `string: max_len: 200` | Role hint. |
| `employerHint` | `string` | string | `string: max_len: 200` | Employer hint. |
| `expectedGate` | `string` | string | `string: max_len: 8` | above, below, or empty to add it unlabelled for review later. |
| `selection` | `string` | string | `string: max_len: 16` | chosen or random; defaults to chosen. |
| `source` | `string` | string | `string: max_len: 500` | Where the posting came from. |
| `expectedBand` | `string` | string | `string: max_len: 20` | Advisory band. |
| `note` | `string` | string | `string: max_len: 2000` | Why it is in the set. |

### UpsertGoldenPostingResponse

Upsert-golden-posting response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id of the created or replaced posting. |

### SetGoldenActiveRequest

Set-golden-active request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id. |
| `active` | `bool` | boolean |  | False retires it. |

### SetGoldenActiveResponse

Set-golden-active response.

_No fields._

### LabelGoldenPostingRequest

Label-golden-posting request. Labelling is its own call so recording
a judgment does not mean resending the whole posting text.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id. |
| `expectedGate` | `string` | string | `string: min_len: 1 max_len: 8` | above or below. |
| `note` | `string` | string | `string: max_len: 2000` | Why, in a sentence. Appended to the posting's note. |

### LabelGoldenPostingResponse

Label-golden-posting response.

_No fields._

### EvalRun

One scoring of the whole active golden set under one configuration.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id. |
| `evalId` | `string` | string |  | Stable external identifier. |
| `status` | `string` | string |  | running | done | failed. |
| `note` | `string` | string |  | Free note describing what was being tested. |
| `appCommit` | `string` | string |  | Build the api was running. |
| `host` | `string` | string |  | Where the model ran. |
| `model` | `string` | string |  | Judge model. |
| `numCtx` | `int32` | number |  | Context window asked for. |
| `embedderModel` | `string` | string |  | Embedding model behind the corpus. |
| `promptsJson` | `string` | string |  | Prompt id to "version:hash" as JSON. |
| `corpusFingerprint` | `string` | string |  | Corpus fingerprint at the time of the run. |
| `threshold` | `double` | number |  | _(oneof `_threshold`)_ Gate in force. |
| `total` | `int32` | number |  | Postings in the set. |
| `scored` | `int32` | number |  | Postings that produced a score. |
| `gateCorrect` | `int32` | number |  | Scored postings that landed on the expected side of the gate. |
| `orderViolations` | `int32` | number |  | Pairs where a posting expected below outscored one expected above. Worse than a gate miss: a gate can be moved, an inversion cannot. |
| `margin` | `double` | number |  | _(oneof `_margin`)_ Smallest gap between the two groups; a shrinking margin is a regression that gate accuracy hides. |
| `errors` | `int32` | number |  | Postings that failed to score. |
| `startedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it started. |
| `finishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it finished; unset while running. |
| `items` | [`EvalItem`](#evalitem)[] | array of object |  | Per-posting results; populated by GetEvalRun only. |
| `corpusDocuments` | [`EvalCorpusDoc`](#evalcorpusdoc)[] | array of object |  | What the corpus held when the run started; populated by GetEvalRun only. The fingerprint above says whether two runs read the same corpus; this says what that corpus was. Empty for runs that predate the snapshot, which means unknown rather than none. |

### EvalCorpusDoc

One document the corpus held when an evaluation began. Title and kind
are copied at capture rather than joined at read, so re-indexing a
document later does not rewrite the history of a run that read the
version before it.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `documentId` | `int64` | string (decimal) |  | Corpus document id. |
| `title` | `string` | string |  | Document title as it was at capture. |
| `sourceKind` | `string` | string |  | Its source kind, such as career_note or article. |
| `visibility` | `string` | string |  | public or corpus_only. |
| `chunkCount` | `int32` | number |  | Chunks the document held at capture. |
| `contentHash` | `string` | string |  | What the document's text hashed to at capture, hex. Empty for runs captured before this was recorded, which means unknown rather than unchanged. Without it the manifest says which documents a run read and not whether they were the same documents: an edit that preserves the chunk count is otherwise invisible. |

### EvalItem

One posting's result inside one evaluation.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `goldenId` | `int64` | string (decimal) |  | Golden posting id. |
| `goldenName` | `string` | string |  | Its label. |
| `expectedGate` | `string` | string |  | The side of the gate it was expected on. |
| `runId` | `string` | string |  | The pipeline run that produced this result. |
| `submissionId` | `int64` | string (decimal) |  | The submission the run belongs to, for opening the derivation. |
| `matchScore` | `double` | number |  | _(oneof `_match_score`)_ Score it got; unset when it failed. |
| `fit` | `string` | string |  | Fit band it landed in. |
| `gateSide` | `string` | string |  | The side it actually landed on. |
| `passed` | `bool` | boolean |  | Whether that matched the expectation. |
| `error` | `string` | string |  | Why it failed to score. |

### ListEvalRunsRequest

List-eval-runs request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number | `int32: lte: 100 gte: 0` | Maximum rows, default 25. |

### ListEvalRunsResponse

List-eval-runs response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `runs` | [`EvalRun`](#evalrun)[] | array of object |  | Evaluations, newest first. |

### GetEvalRunRequest

Get-eval-run request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Row id. |

### GetEvalRunResponse

Get-eval-run response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `run` | [`EvalRun`](#evalrun) | object |  | The evaluation, with its per-posting results. |

### GetMetricsRequest

Get-metrics request.

_No fields._

### GetGateRequest

Read request.

_No fields._

### GetGateResponse

The criteria, in the order the owner wrote them.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `rows` | [`GateRow`](#gaterow)[] | array of object |  | One row per criterion. |

### GateRow

One criterion, answered.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `criterion` | `string` | string |  | Which criterion, as the owner named it. |
| `pass` | `bool` | boolean |  | _(oneof `_pass`)_ Whether the target is met. Unset means it cannot be answered yet: not enough data, or no target has been set. An unset value is not a failure and must not be rendered as one. |
| `value` | `string` | string |  | The measurement, already phrased for reading. |
| `target` | `string` | string |  | What would count as met. |
| `asOf` | `Timestamp` | string (RFC 3339, UTC) |  | When the measurement is from. |
| `detail` | `string` | string |  | Anything that qualifies the answer, such as how short the sample is or which part of the target failed. |
| `gated` | `bool` | boolean |  | Whether this row is a criterion at all. False means it is measured on purpose and gated on purpose: reach moves with who happened to find the site, not with whether the reviewer improved. Such a row must not be rendered as permanently unanswered, which would read as a standing reproach for something that is not a fault. |

### OutcomeByFit

One fit band against what actually happened afterwards.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `fit` | `string` | string |  | Fit band the run assigned. |
| `outcome` | `string` | string |  | Recorded outcome. |
| `submissions` | `int32` | number |  | How many submissions. |

### GetMetricsResponse

The state of the reviewer. Every field comes from a view in
migration 00028; none is computed here.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `recentRuns` | `int32` | number |  | Real runs in the last twenty. |
| `recentCompleted` | `int32` | number |  | Of those, how many finished without intervention. |
| `recentFailed` | `int32` | number |  | Of those, how many failed. |
| `recentStuck` | `int32` | number |  | Of those, how many are still claiming to run. |
| `reviewed` | `int32` | number |  | Requirement verdicts the owner has reviewed. |
| `agreed` | `int32` | number |  | Of those, how many the owner agreed with. |
| `agreementPct` | `double` | number |  | _(oneof `_agreement_pct`)_ Agreement as a percentage of the gradeable rows; unset when none. |
| `softDisagreements` | `int32` | number |  | Disagreements where one side said partial: the judge being unsure. |
| `hardDisagreements` | `int32` | number |  | Disagreements between met and unmet: the judge being wrong. |
| `gradeable` | `int32` | number |  | Reviewed rows the owner could actually judge. |
| `ungradeable` | `int32` | number |  | Rows the owner could not judge from what they were shown. Excluded from the agreement rate, and the most actionable number here: a pile of them means retrieval is failing, which no prompt fixes. |
| `tooHarsh` | `int32` | number |  | Disagreements where the model said unmet and the owner did not. |
| `tooGenerous` | `int32` | number |  | Disagreements where the model credited more than the owner did. |
| `finishedRuns` | `int32` | number |  | Runs that reached a result. |
| `medianMinutes` | `double` | number |  | _(oneof `_median_minutes`)_ Median minutes to a result. |
| `p95Minutes` | `double` | number |  | _(oneof `_p95_minutes`)_ 95th percentile minutes to a result. |
| `medianQueuedMinutes` | `double` | number |  | _(oneof `_median_queued_minutes`)_ Median minutes spent queued, reported apart from work. |
| `landed` | `int32` | number |  | Thirty-day funnel: visitors who reached the landing page. |
| `readWriting` | `int32` | number |  | Visitors who read an article. |
| `clicked` | `int32` | number |  | Visitors who clicked a call to action. |
| `registered` | `int32` | number |  | Visitors who submitted a registration. |
| `verified` | `int32` | number |  | Visitors who verified their email. |
| `signedIn` | `int32` | number |  | Visitors who signed in. |
| `submitted` | `int32` | number |  | Visitors who submitted a posting. |
| `calls` | `int32` | number |  | Model calls in the last 30 days. |
| `callFailures` | `int32` | number |  | Of those, how many failed. |
| `promptTokens` | `int64` | string (decimal) |  | Prompt tokens in the last 30 days. |
| `completionTokens` | `int64` | string (decimal) |  | Completion tokens in the last 30 days. |
| `latestEval` | [`EvalRun`](#evalrun) | object |  | Newest finished evaluation; unset before the first one. |
| `outcomes` | [`OutcomeByFit`](#outcomebyfit)[] | array of object |  | Fit band against recorded outcomes. |

### JdRun

One pipeline run: what produced a verdict and what it decided
(data layer D2, table `jd_runs`).

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `runId` | `string` | string |  | Stable run identifier; `llm_usage` and `decision_log` rows carry it. |
| `attempt` | `int32` | number |  | 1 for the first run, 2 for the first re-score, and so on. |
| `trigger` | `string` | string |  | `submit` or `rescore`. |
| `triggeredBy` | `int64` | string (decimal) |  | Admin who triggered a re-score; 0 for a submission. |
| `status` | `string` | string |  | running | ready | below_threshold | failed. |
| `error` | `string` | string |  | Why it failed, empty otherwise. |
| `appCommit` | `string` | string |  | Build the api was running. |
| `host` | `string` | string |  | Where the model ran. |
| `model` | `string` | string |  | Judge model as the gateway reported it. |
| `numCtx` | `int32` | number |  | Context window the sidecar was asked for. |
| `embedderModel` | `string` | string |  | Embedding model the corpus chunks were built with. |
| `promptsJson` | `string` | string |  | Prompt id to "version:hash" as JSON, so an edited prompt is visible even when its version did not change. |
| `corpusFingerprint` | `string` | string |  | Hash over the corpus documents this run could retrieve from. |
| `corpusDocuments` | `int32` | number |  | Documents in the corpus at run time. |
| `corpusChunks` | `int32` | number |  | Chunks in the corpus at run time. |
| `scoreFormula` | `string` | string |  | Name of the arithmetic that produced the score. |
| `retrievalScore` | `double` | number |  | _(oneof `_retrieval_score`)_ Mean top-K cosine, the cheap pre-score. |
| `matchScore` | `double` | number |  | _(oneof `_match_score`)_ Requirement-weighted score, the one the gate reads. |
| `threshold` | `double` | number |  | _(oneof `_threshold`)_ Gate in force for this run. |
| `fit` | `string` | string |  | Fit band the score fell in. |
| `requirementCount` | `int32` | number |  | Requirements the posting was broken into. |
| `metCount` | `int32` | number |  | Requirements judged met. |
| `partialCount` | `int32` | number |  | Requirements judged partly met. |
| `unmetCount` | `int32` | number |  | Requirements judged unmet. |
| `resumeGenerated` | `bool` | boolean |  | Whether this run produced a tailored résumé. |
| `queuedMs` | `int64` | string (decimal) |  | Time spent waiting for a pipeline slot. |
| `durationMs` | `int64` | string (decimal) |  | Time spent working, once it held the slot. |
| `startedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the run opened. |
| `finishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the run closed; unset while it is still running. |

### RescoreJdRequest

Rescore-jd request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissionId` | `string` | string | `string: min_len: 1 max_len: 32` | Submission id (numeric, stringified). |

### RescoreJdResponse

Rescore-jd response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`JdStatus`](#jdstatus) | string (enum name) |  | Status the row was set to when the run was queued (`scoring`). |

### ReindexCorpusRequest

Reindex-corpus request. `scope` picks the mount: `public` walks the
committed content under CORPUS_ROOT (documents land as
visibility=public); `private` walks CORPUS_PRIVATE_ROOT, where each
top-level directory is a source_kind and every document lands as
visibility=corpus_only. `source_kind` optionally narrows either
scope to one kind.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sourceKind` | `string` | string | `string: max_len: 40` | Optional kind filter (e.g. `article`, `worksheet`). Empty walks every kind in the scope. Validated against the ingest allow-list. |
| `scope` | `string` | string | `string: max_len: 16` | `public` (default) or `private`. |

### ReindexCorpusResponse

Reindex-corpus response — mirrors ingest.WalkResult.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `root` | `string` | string |  | Absolute path the handler actually walked. |
| `visibility` | `string` | string |  | Visibility stamped on every document this run: public or corpus_only. |
| `kindsWalked` | `string`[] | array of string |  | Source kinds that were walked, sorted. |
| `filesScanned` | `int32` | number |  | Number of `.md` files the walker saw. |
| `docsIngested` | `int32` | number |  | Number of files handed to the ingester without error. Includes skipped-because-unchanged. |
| `docsSkipped` | `int32` | number |  | Subset of docs_ingested whose content_hash matched the stored row and did no chunk / embed work this run. |
| `chunksInserted` | `int32` | number |  | Sum of newly-inserted chunk rows across all files this run. |
| `chunksEmbedded` | `int32` | number |  | Sum of chunks successfully embedded across all files this run. |
| `errors` | `string`[] | array of string |  | Per-file error strings from files that failed mid-walk. Capped at 20 entries so a broken directory can't produce an unbounded response. |

### ListJdSubmissionsRequest

List-jd-submissions request. No filters yet.

_No fields._

### JdSubmissionRow

One row of the /admin/jd triage table.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Submission id (numeric, stringified). |
| `status` | [`JdStatus`](#jdstatus) | string (enum name) |  | Lifecycle status (mirrors JdStatus). |
| `matchScore` | `double` | number |  | _(oneof `_match_score`)_ Aggregated match score in [0, 1]; unset before scoring runs. |
| `textHead` | `string` | string |  | First ~400 chars of the JD text so the admin can preview without loading the full body. |
| `roleHint` | `string` | string |  | Optional role the submitter typed on the form. |
| `employerHint` | `string` | string |  | Optional employer the submitter typed on the form. |
| `contactEmail` | `string` | string |  | Optional email the submitter provided for the tailored-résumé handoff. |
| `source` | [`JdSource`](#jdsource) | string (enum name) |  | Where the JD text came from (paste / pdf / text_upload). |
| `errorMessage` | `string` | string |  | Truncated provider error when status is FAILED; empty otherwise. |
| `generatedResumeUrl` | `string` | string |  | Download path of the locked résumé PDF (without the token) when status is READY; empty until then. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the submission was received. |
| `completedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the terminal state was reached; unset while in-flight. |
| `retrievalScore` | `double` | number |  | _(oneof `_retrieval_score`)_ Retrieval pre-score (mean top-K cosine), diagnostics only, never a gate; unset before scoring. match_score is the requirement-weighted score the gate is applied to. |
| `submitterEmail` | `string` | string |  | Email of the signed-in member who submitted; empty for rows created before JD upload became members-only. |
| `applyUrl` | `string` | string |  | Optional link to apply for the position, as given by the submitter. |

### ListJdSubmissionsResponse

List-jd-submissions response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `submissions` | [`JdSubmissionRow`](#jdsubmissionrow)[] | array of object |  | Rows, newest first (up to 500). |
| `readyCount` | `int32` | number |  | Number of rows currently in a terminal READY state. |
| `belowThresholdCount` | `int32` | number |  | Number that scored below the résumé gate (the strong band). |
| `failedCount` | `int32` | number |  | Number that failed during scoring / generation. |
| `inFlightCount` | `int32` | number |  | Number still in flight (received / scoring / generating). |

### DecisionLogRow

One logged decision: what the model (or the code) decided, from
what, and the owner's review of it. See docs/decision-log.md.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Row id (numeric, stringified). |
| `kind` | `string` | string |  | Decision kind: jd_requirement_verdict, jd_gate, ... |
| `refKind` | `string` | string |  | What the decision belongs to (e.g. jd_submission). |
| `refId` | `string` | string |  | Id of the ref (numeric, stringified). |
| `key` | `string` | string |  | Item within the ref (requirement id); empty when per-ref. |
| `model` | `string` | string |  | Who decided: the gateway model name, or "code". |
| `promptId` | `string` | string |  | Prompt id that produced it; empty for code decisions. |
| `promptVersion` | `int32` | number |  | Prompt version that produced it; 0 for code decisions. |
| `numCtx` | `int32` | number |  | Context window requested for the call; 0 for code decisions. |
| `inputJson` | `string` | string |  | What it was decided from, as JSON (shape depends on kind). |
| `outputJson` | `string` | string |  | What was decided, as JSON (shape depends on kind). |
| `promptText` | `string` | string |  | Exact system + user prompt sent; empty for code decisions. |
| `responseText` | `string` | string |  | Raw model output; empty for code decisions. |
| `promptTokens` | `int32` | number |  | Prompt tokens as reported by the provider. |
| `completionTokens` | `int32` | number |  | Completion tokens as reported by the provider. |
| `latencyMs` | `int64` | string (decimal) |  | Call latency in milliseconds. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the decision was made. |
| `humanVerdict` | `string` | string |  | The owner's verdict in the same vocabulary as the output; empty until reviewed. |
| `humanNote` | `string` | string |  | The owner's free-text note on the review. |
| `reviewedBy` | `string` | string |  | User id of the reviewer (numeric, stringified); empty until reviewed. |
| `reviewedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the review was saved; unset until reviewed. |
| `error` | `string` | string |  | Why this call produced no usable verdict; empty when it produced one. A row carrying it still carries prompt_text and response_text, which is the point: the response that failed to decode is the one worth reading, and it used to be the only one not kept. |
| `humanAnswer` | `string` | string |  | The wording the owner would have given, for a decision whose output is prose rather than a label. This is what an adapter is trained on: with response_text it forms a preference pair, and on its own it is a supervised target. Empty for a classification decision, where human_verdict is already the gold output, and empty for an answer the owner thought was right. |
| `humanDimensions` | map<`string`, `string`> | object of string |  | Per-rubric grades, e.g. grounded / citations / voice / scope / length, each marked yes, partial, no or n/a. One overall verdict cannot express an answer that is factually right and in the wrong voice, and cannot produce the grounding and citation-validity rates the chat acceptance criteria are stated in. |
| `firstTokenMs` | `int64` | string (decimal) |  | Milliseconds to the first token. Separate from latency_ms because on a CPU-only box the first-token wait tracks how much context was retrieved while the total tracks how much was written, and one number cannot tell a retrieval regression from a verbose one. Zero where it does not apply. |

### ListDecisionLogRequest

Filters for the decision-review list.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | `string` | string |  | Restrict to one kind; empty for all kinds. |
| `refId` | `string` | string |  | Restrict to one ref id (e.g. a JD submission); empty for all. |
| `unreviewedOnly` | `bool` | boolean |  | Only rows the owner has not reviewed yet. |
| `limit` | `int32` | number |  | Page size, newest first; server caps at 500 (default 200). |
| `failedOnly` | `bool` | boolean |  | Only calls that produced no usable verdict. They are rare by construction and are the rows most worth reading, so finding them should not mean scrolling past two hundred successes. |

### ListDecisionLogResponse

A page of logged decisions plus table-wide counts.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `decisions` | [`DecisionLogRow`](#decisionlogrow)[] | array of object |  | Rows, newest first. |
| `totalCount` | `int64` | string (decimal) |  | Rows in the whole table, any kind. |
| `reviewedCount` | `int64` | string (decimal) |  | Rows the owner has reviewed. |

### ReviewDecisionRequest

The owner's label for one decision.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Row id to label. |
| `humanVerdict` | `string` | string |  | The owner's verdict; the vocabulary of the row's output (met / partial / unmet for requirement verdicts). |
| `humanNote` | `string` | string |  | Optional note explaining the label. Read by a human; never used as a training target. A rewrite goes in human_answer. |
| `humanAnswer` | `string` | string |  | The wording the owner would have given, for a decision whose output is prose. Optional, and the single most valuable field on this message: a graded answer with no correction can be counted, while a graded answer with one can be learned from. |
| `humanDimensions` | map<`string`, `string`> | object of string |  | Per-rubric grades for kinds that have a rubric (chat answers: grounded, citations, voice, scope, length). Each value is yes, partial, no or n/a. Unknown keys and values are rejected rather than stored, because a grade nobody can interpret still counts in a rate. |

### ReviewDecisionResponse

Empty: success is the absence of an error.

_No fields._

### QaSource

A source shown beneath a Q&A bank answer.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `title` | `string` | string | `string: max_len: 200` | Title to display. |
| `path` | `string` | string | `string: max_len: 256` | Site path to link to. |

### QaPhrasing

One way of asking a bank entry's question.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Phrasing id (numeric, stringified). |
| `text` | `string` | string |  | The wording. |
| `canonical` | `bool` | boolean |  | True for the phrasing generated from the entry's own question. There is exactly one per entry and it cannot be deleted. |
| `embedded` | `bool` | boolean |  | Whether this phrasing has an embedding yet. A phrasing without one never matches, so this is the difference between an entry being in the bank and being reachable. |

### QaEntry

One owner-written answer, served verbatim.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Entry id (numeric, stringified). |
| `question` | `string` | string |  | The question as the owner would put it. Also used as a suggested question, and it is the canonical phrasing. |
| `answer` | `string` | string |  | The answer, in the owner's voice, served word for word. No model ever rewrites this. |
| `sources` | [`QaSource`](#qasource)[] | array of object |  | Sources to show beneath the answer. |
| `tags` | `string`[] | array of string |  | Free tags for filtering and grouping. |
| `coversRestricted` | `bool` | boolean |  | Marks an entry that deliberately answers something the assistant otherwise refuses: compensation, references, an employer matter, something personal. Not enforcement; this is so every such statement can be listed, and so opening one of those doors is a deliberate act. |
| `enabled` | `bool` | boolean |  | Enabling is the approval. A disabled entry is never matched. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it was written. |
| `updatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it last changed. |
| `phrasings` | [`QaPhrasing`](#qaphrasing)[] | array of object |  | Every way of asking it, canonical first. |

### ListQaEntriesRequest

Bank listing request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includeDisabled` | `bool` | boolean |  | Include entries that are not enabled. False for anything member-facing. |

### ListQaEntriesResponse

The bank, newest first.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `entries` | [`QaEntry`](#qaentry)[] | array of object |  | Entries. |
| `awaitingEmbedding` | `int32` | number |  | Phrasings still waiting for an embedding across the whole bank. A non-zero count means part of the bank cannot match yet. |

### CreateQaEntryRequest

New entry.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `question` | `string` | string | `string: min_len: 1 max_len: 500` | The question. |
| `answer` | `string` | string | `string: min_len: 1 max_len: 8000` | The answer, served verbatim. |
| `sources` | [`QaSource`](#qasource)[] | array of object |  | Sources to show beneath it. |
| `tags` | `string`[] | array of string |  | Tags. |
| `coversRestricted` | `bool` | boolean |  | Whether this deliberately answers an otherwise-refused topic. |
| `enabled` | `bool` | boolean |  | Whether to approve it now. |
| `phrasings` | `string`[] | array of string |  | Extra ways of asking it; the question itself is added automatically. |

### CreateQaEntryResponse

The new entry.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Entry id (numeric, stringified). |

### UpdateQaEntryRequest

Entry edit.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Entry to change. |
| `question` | `string` | string | `string: min_len: 1 max_len: 500` | The question. Changing it clears the canonical phrasing's vector. |
| `answer` | `string` | string | `string: min_len: 1 max_len: 8000` | The answer. |
| `sources` | [`QaSource`](#qasource)[] | array of object |  | Sources. |
| `tags` | `string`[] | array of string |  | Tags. |
| `coversRestricted` | `bool` | boolean |  | Whether this deliberately answers an otherwise-refused topic. |
| `enabled` | `bool` | boolean |  | Whether it is approved. |

### UpdateQaEntryResponse

Empty: success is the absence of an error.

_No fields._

### SetQaEntryEnabledRequest

Approval change.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Entry to approve or withdraw. |
| `enabled` | `bool` | boolean |  | True approves it. |

### SetQaEntryEnabledResponse

Empty: success is the absence of an error.

_No fields._

### DeleteQaEntryRequest

Entry removal.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string | `string: min_len: 1 max_len: 64` | Entry to delete. |

### DeleteQaEntryResponse

Empty: success is the absence of an error.

_No fields._

### AddQaPhrasingRequest

New phrasing.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `entryId` | `string` | string | `string: min_len: 1 max_len: 64` | Entry it belongs to. |
| `text` | `string` | string | `string: min_len: 1 max_len: 500` | The wording. |

### AddQaPhrasingResponse

The new phrasing.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Phrasing id (numeric, stringified). |

### DeleteQaPhrasingRequest

Phrasing removal.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `entryId` | `string` | string | `string: min_len: 1 max_len: 64` | Entry it belongs to. |
| `phrasingId` | `string` | string | `string: min_len: 1 max_len: 64` | Phrasing to remove. The canonical one cannot be removed. |

### DeleteQaPhrasingResponse

Empty: success is the absence of an error.

_No fields._

### ExportDecisionLogRequest

Selects which rows the JSONL export includes.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `reviewedOnly` | `bool` | boolean |  | Only rows the owner has reviewed (the training set); false exports everything. |

### ExportDecisionLogResponse

The rendered export.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jsonl` | `string` | string |  | JSON Lines, one decision per line, oldest first. |
| `rowCount` | `int32` | number |  | Number of lines in jsonl. |

### GetJdFitBandsRequest

Read request.

_No fields._

### GetJdFitBandsResponse

The bands in force and who last changed them.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `bands` | [`JdFitBands`](#jdfitbands) | object |  | Current bands. |

### SetJdFitBandsRequest

New bands; must satisfy 0 < weak < possible < strong < very_strong <= 1.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `bands` | [`JdFitBands`](#jdfitbands) | object |  | The bands to store. |

### SetJdFitBandsResponse

The bands as stored.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `bands` | [`JdFitBands`](#jdfitbands) | object |  | Stored bands. |

### SchedulerWindow

One bookable range on one weekday. Held as minutes from local
midnight rather than as a clock time, so a window survives a
daylight-saving change underneath it instead of drifting an hour.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `weekday` | `int32` | number |  | Day of the week, 0 for Sunday through 6 for Saturday, matching Go's time.Weekday so the server needs no translation table. |
| `startMinutes` | `int32` | number |  | Minutes from local midnight at which the window opens. |
| `endMinutes` | `int32` | number |  | Minutes from local midnight at which it closes. Must be after the start and inside the same day. |

### SchedulerSettings

The owner's availability, as the admin surface edits it.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `zone` | `string` | string |  | IANA zone name, for example "America/New_York". Never a fixed offset: an offset is correct for half the year and silently an hour wrong for the other half. |
| `durationMinutes` | `int32`[] | array of number |  | Meeting lengths a member may choose, in minutes. |
| `gapMinutes` | `int32` | number |  | Clearance between meetings, in minutes, applied on both sides of a candidate. Not carved out of the grid in advance: nothing is held until something is booked. |
| `stepMinutes` | `int32` | number |  | How finely start times are offered, in minutes. |
| `maxPerDay` | `int32` | number |  | Most meetings that may be taken in one day. |
| `leadHours` | `int32` | number |  | How far ahead the earliest bookable slot sits, in hours. |
| `horizonDays` | `int32` | number |  | How many days ahead the calendar is offered. |
| `windows` | [`SchedulerWindow`](#schedulerwindow)[] | array of object |  | The weekly windows. Two windows on one day are how a lunch break is expressed; one range with a break bolted on later is the usual way this ends up wrong. |

### GetSchedulerSettingsRequest

Request for GetSchedulerSettings.

_No fields._

### GetSchedulerSettingsResponse

The settings in force.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `settings` | [`SchedulerSettings`](#schedulersettings) | object |  | Current settings. |
| `calendarConnected` | `bool` | boolean |  | False when the calendar provider is not connected or unreachable, so the surface can say booking is off rather than implying these windows are live. |
| `calendarStatus` | `string` | string |  | Why the calendar is not usable, when calendar_connected is false. |

### SetSchedulerSettingsRequest

Replacement settings. Refused whole if any part is invalid.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `settings` | [`SchedulerSettings`](#schedulersettings) | object |  | The settings to store. |

### SetSchedulerSettingsResponse

The settings as stored.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `settings` | [`SchedulerSettings`](#schedulersettings) | object |  | Stored settings. |

### GetCalendarConnectURLRequest

Request for GetCalendarConnectURL.

_No fields._

### GetCalendarConnectURLResponse

Where to send the owner to authorise.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `url` | `string` | string |  | The full consent URL, including the signed state. |

### ConnectCalendarRequest

The authorisation code Google handed back.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `code` | `string` | string | `string: min_len: 1` | The one-time code from the callback's query string. |
| `state` | `string` | string | `string: min_len: 1` | The state from the callback, verified against the one issued. |

### ConnectCalendarResponse

The connection as stored.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`CalendarStatus`](#calendarstatus) | object |  | Current status after connecting. |

### GetCalendarStatusRequest

Request for GetCalendarStatus.

_No fields._

### CalendarStatus

Whether a calendar is connected, and whether it is working.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `configured` | `bool` | boolean |  | True when this deployment has the client credentials and sealing key needed to connect at all. False means set the environment variables, which is a different fix from pressing connect. |
| `connected` | `bool` | boolean |  | True when a credential is stored. |
| `accountEmail` | `string` | string |  | The Google account that authorised, for the owner to recognise. |
| `calendarId` | `string` | string |  | Which calendar is written to; "primary" is the account's own. |
| `scopes` | `string` | string |  | Scopes Google actually granted, so a consent screen that returned less than was asked for is visible rather than surfacing later as a puzzling refusal. |
| `connectedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the owner connected it. |
| `lastOkAt` | `Timestamp` | string (RFC 3339, UTC) |  | When Google last answered successfully. Absent means it has never worked, which is different from having stopped. |
| `lastError` | `string` | string |  | Why the last call failed, if it did. Empty when healthy. |

### GetCalendarStatusResponse

The current connection.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`CalendarStatus`](#calendarstatus) | object |  | Current status. |

### DisconnectCalendarRequest

Request for DisconnectCalendar.

_No fields._

### DisconnectCalendarResponse

The connection after forgetting the credential.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`CalendarStatus`](#calendarstatus) | object |  | Current status. |

### AdminMeeting

One booked meeting, as the owner sees it. Carries who booked it,
which the member-facing message deliberately does not.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Stable id, used to cancel. |
| `start` | `Timestamp` | string (RFC 3339, UTC) |  | When it begins. |
| `end` | `Timestamp` | string (RFC 3339, UTC) |  | When it ends. |
| `durationMinutes` | `int32` | number |  | Length in minutes, as booked. |
| `topic` | `string` | string |  | What the member said they wanted to discuss. |
| `memberName` | `string` | string |  | The member's name, as held on their account. |
| `memberEmail` | `string` | string |  | The member's email. |
| `memberId` | `int64` | string (decimal) |  | The member's account id, 0 when the row has none. |
| `eventId` | `string` | string |  | The calendar event this created. Empty means the claim exists here but no event was made, which is worth seeing. |
| `createdAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the booking was made. |
| `cancelledAt` | `Timestamp` | string (RFC 3339, UTC) |  | Set when cancelled. Cancelled rows are kept rather than deleted, so a meeting that vanished can be explained. |
| `meetingType` | `string` | string |  | video | phone, or empty for a booking made before this was asked. |
| `videoProvider` | `string` | string |  | google_meet | teams | zoom, when the type is video. |
| `phoneNumber` | `string` | string |  | The number the member will call from, when the type is phone. Empty on a phone meeting means they said it is in the comments. |

### ListMeetingsRequest

Request for ListMeetings.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includePast` | `bool` | boolean |  | Include meetings that have already happened or been cancelled. |

### ListMeetingsResponse

Booked meetings.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meetings` | [`AdminMeeting`](#adminmeeting)[] | array of object |  | Meetings, soonest first. |
| `zone` | `string` | string |  | IANA zone the times should be rendered in. |

### CancelMeetingAsAdminRequest

Request for CancelMeetingAsAdmin.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) | `int64: gt: 0` | Which meeting to cancel. |

### CancelMeetingAsAdminResponse

The meeting after cancelling.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meeting` | [`AdminMeeting`](#adminmeeting) | object |  | The cancelled meeting. |

### GetJdSubmissionLimitRequest

Read request.

_No fields._

### GetJdSubmissionLimitResponse

The limit in force.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number |  | Postings one member may submit per rolling window. |
| `windowHours` | `int32` | number |  | How long the window is, in hours. |

### SetJdSubmissionLimitRequest

A new limit.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number | `int32: lte: 100 gte: 0` | Postings one member may submit per rolling window. 0 removes the cap, which on a box that reviews one posting an hour means one member with a script can fill the queue indefinitely. |

### SetJdSubmissionLimitResponse

The limit as stored.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `limit` | `int32` | number |  | Stored limit. |
| `windowHours` | `int32` | number |  | How long the window is, in hours. |

### GetOpsStatusRequest

Empty.

_No fields._

### GetJobDetailRequest

Which job to describe.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobId` | `string` | string | `string: min_len: 1 max_len: 64` | The runner id, as GetOpsStatus reports it. |

### GetJobDetailResponse

One job and everything it reported while it ran.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `job` | [`JobRow`](#jobrow) | object |  | The job itself, the same row GetOpsStatus lists. |
| `events` | [`JobEvent`](#jobevent)[] | array of object |  | Every progress report, oldest first, capped by the runner. |
| `truncated` | `bool` | boolean |  | True when the runner dropped older events at its cap, so a reader knows the timeline starts mid-run rather than at the beginning. |

### JobEvent

One progress report, kept with the time it arrived. The gap between
consecutive events is the measurement; the text is only how the job
described what it was doing.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `at` | `Timestamp` | string (RFC 3339, UTC) |  | When the report arrived. |
| `progress` | `int32` | number |  | Percent complete at that moment, 0 when the job does not report it. |
| `summary` | `string` | string |  | What the job said it was doing. |
| `ref` | `string` | string |  | The record this report is about, empty when there is none. Opaque to the runner: the job that reports the event chooses the form. An evaluation writes "eval:<run id>:<golden id>", with golden id 0 meaning the run itself, so the console can open a posting's result from its line in the timeline instead of parsing the summary. |

### GetOpsStatusResponse

What the box is doing.

The reason this exists: the job runner keeps jobs in memory, so a
deploy recreates the api container and any running job dies with it.
That happened twice on 2026-09-24, the second time losing an
evaluation seven postings in. A deploy is safe or unsafe depending on
this answer, and it was not visible anywhere.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `jobs` | [`JobRow`](#jobrow)[] | array of object |  | Jobs the runner still remembers, newest first. Finished jobs are forgotten after a day. |
| `busy` | `bool` | boolean |  | True when any job is running or queued, which is the one thing to check before deploying. |
| `pipeline` | [`PipelineCount`](#pipelinecount)[] | array of object |  | Submissions by pipeline status. |
| `latestEval` | [`EvalRun`](#evalrun) | object |  | The most recent evaluation, running or not. |
| `callsLastHour` | `int32` | number |  | Model calls in the last hour. |
| `callFailuresLastHour` | `int32` | number |  | Of those, how many failed. |
| `avgCallSeconds` | `double` | number |  | Average seconds per call in the last hour. |
| `load1` | `double` | number |  | Host load average over one minute, as the kernel reports it. |
| `load5` | `double` | number |  | Over five minutes. |
| `memTotalMb` | `int32` | number |  | Total host memory in megabytes. |
| `memAvailableMb` | `int32` | number |  | Host memory available in megabytes. |
| `diskFreeGb` | `int32` | number |  | Free disk in gigabytes on the filesystem the api is running from. |
| `diskTotalGb` | `int32` | number |  | Total disk in gigabytes on that filesystem. |
| `warnings` | `string`[] | array of string |  | Anything this snapshot could not read. A section that failed and a section that is genuinely empty look identical otherwise, and on 2026-09-25 that difference hid a broken query behind the words "no submissions" for as long as nobody read the logs. |

### JobRow

One job the runner remembers.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Runner id. |
| `kind` | `string` | string |  | Which kind of work. |
| `status` | `string` | string |  | queued, running, succeeded, failed or cancelled. |
| `progress` | `int32` | number |  | Percent complete, 0 when the job does not report progress. |
| `summary` | `string` | string |  | What it is doing, or what it finished with. |
| `startedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it started. |
| `finishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it ended; unset while running. |

### PipelineCount

How many submissions sit at one pipeline status.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | `string` | string |  | The status. |
| `count` | `int32` | number |  | How many. |
| `oldest` | `Timestamp` | string (RFC 3339, UTC) |  | The oldest one still at this status, for spotting something stuck. |

### GetDecisionTestSettingsRequest

Asks for the decision test's timings.

_No fields._

### GetDecisionTestSettingsResponse

The decision test's timings, with the instrument version they produce.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memoriseMs` | `int32` | number |  | How long the number to hold is shown, in milliseconds. |
| `questionMs` | `int32` | number |  | Hard limit per question, covering reading, deciding and rating. |
| `recallMs` | `int32` | number |  | Limit on entering the number at the end of a block. |
| `instrumentVersion` | `string` | string |  | Derived from the three above, and stored on every session taken under them. Shown so the owner can see that changing a timing changes the instrument. |
| `sessionsOnThisVersion` | `int32` | number |  | How many sessions have already been recorded under this version. Changing a timing starts a new one, which is the cost of the change. |

### SetDecisionTestSettingsRequest

Changes the decision test's timings.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memoriseMs` | `int32` | number | `int32: gte: 0` | How long the number to hold is shown. Clamped to 1 to 30 seconds. |
| `questionMs` | `int32` | number | `int32: gte: 0` | Hard limit per question. Clamped to 5 to 120 seconds, because a question nobody can answer in time is not a harder test, it is an unanswerable one. |
| `recallMs` | `int32` | number | `int32: gte: 0` | Limit on entering the number. Clamped to 5 to 120 seconds. |

### SetDecisionTestSettingsResponse

The timings after the change.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `instrumentVersion` | `string` | string |  | The instrument version now in force. |

### ListDecisionTestRunsRequest

Asks for the list of runs.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includeSynthetic` | `bool` | boolean |  | Include runs driven by an agent rather than taken by a person. Excluded by default, because they are not data. |
| `reviewStatus` | `string` | string | `string: max_len: 24` | Only runs with this review status. Empty returns everything; "unreviewed" returns the queue, which is the common case when the point of opening the page is to work through it. |

### ListDecisionTestRunsResponse

Runs, newest first.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `counts` | [`DecisionTestReviewCounts`](#decisiontestreviewcounts) | object |  | The queue, counted across every real run regardless of the filter, so the page can say how much is left without a second call. |
| `runs` | [`DecisionTestRun`](#decisiontestrun)[] | array of object |  | The runs. |
| `itemPools` | [`DecisionTestItemPool`](#decisiontestitempool)[] | array of object |  | Each category's room to vary. On the list rather than tucked away, because a pool that cannot vary is the one fault here that looks exactly like success: the draw runs, every block is balanced, and every participant sits the same test. |

### DecisionTestRun

One run, summarised for a list.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string |  | Opaque handle, used to open the run. |
| `status` | `string` | string |  | running, completed or abandoned. Abandoned runs are kept, because where people stop measures the fifteen-minute burden. |
| `displayName` | `string` | string |  | Who, where they gave a name. Empty is the normal case. |
| `ageRange` | `string` | string |  | Banded age, education and occupation, each empty where not given. |
| `education` | `string` | string |  | Highest level of education completed. |
| `occupation` | `string` | string |  | Free-text occupation. |
| `gaveEmail` | `bool` | boolean |  | Whether an address was left, rather than the address itself. |
| `audioMode` | `string` | string |  | sound or visual. |
| `deviceClass` | `string` | string |  | desktop, tablet or phone. |
| `tapCheckPassed` | `bool` | boolean |  | Whether the tap-along check was passed. |
| `baselineRtMs` | `int32` | number |  | Unloaded synchronisation offset from the tap check, in ms. |
| `isRepeat` | `bool` | boolean |  | A second or later run by the same person, flagged not blocked. |
| `isSynthetic` | `bool` | boolean |  | Driven by an agent rather than taken by a person. |
| `instrumentVersion` | `string` | string |  | The timings and item set this run was taken under. Runs on different versions are different instruments and must not be pooled. |
| `itemSetVersion` | `string` | string |  | Which items, in which order. |
| `correct` | `int32` | number |  | Questions answered correctly, of those scored. |
| `answered` | `int32` | number |  | Questions presented. |
| `expired` | `int32` | number |  | Questions that ran out of time, which is its own outcome. |
| `meanConfidence` | `int32` | number |  | Mean confidence across the run, 0 to 100. |
| `durationS` | `int32` | number |  | Seconds from start to finish, zero while still running. |
| `startedAt` | `string` | string |  | When it started, RFC3339. |
| `reviewStatus` | `string` | string |  | Curation (M9). Empty status means nobody has looked at it yet, which is a state rather than a verdict. |
| `reviewReason` | `string` | string |  | Why it was excluded. Only set alongside do_not_use. |
| `reviewNote` | `string` | string |  | The reviewer's note. |
| `reviewedAt` | `string` | string |  | When it was reviewed, RFC3339, empty if it has not been. |
| `blocksExcluded` | `int32` | number |  | How many of its blocks are individually excluded. Non-zero on a run whose own status is not do_not_use is the case this exists for: a spoiled block inside an otherwise good run. |
| `recallStrategy` | `string` | string |  | What the participant said about holding the number: encode or defer. Asked in the debrief, because the two produce different loads during the questions and an uncontrolled variable becomes a recorded one. |
| `baselineRtSdMs` | `int32` | number |  | Variability of the tap-check presses, a cheap read on baseline attention. Sits beside baseline_rt_ms, which the list already had: a slow participant is only interesting against their own unloaded speed, and a jittery one against their own steadiness. |
| `repeatMatchedBy` | `string` | string |  | How a repeat was recognised: account, email or cookie. Descending reliability, and worth knowing which: a cookie match is defeated by a private window, so it is weaker evidence than an account. |
| `attemptNo` | `int32` | number |  | Which sitting this was for this participant, 1 for a first run. Zero means no identity could place the run at all, which is a different statement from "first" and must not be read as one. |
| `attemptNoStrongest` | `int32` | number |  | The same number read from the single most reliable identity that had an answer, rather than from the union of all three. Published beside attempt_no so a reader can see both; they differ when the identities saw different histories. |
| `attemptSource` | `string` | string |  | Which identity produced attempt_no: account, email, cookie, or none. A sequence resting on a cookie is weaker evidence than one resting on an account, and the reviewer decides what that is worth. |
| `attemptSourcesDisagree` | `bool` | boolean |  | The two readings disagree, so the sequence needs a human rather than a default. |
| `priorByAccount` | `int32` | number |  | Deprecated: use prior_sittings, which can express a zero. These three shipped as plain int32 and cannot carry the one distinction they exist for. Under proto3 implicit presence a scalar equal to its default is omitted from the JSON entirely, so a link that existed and saw zero prior sittings arrives indistinguishable from a link that never existed at all. They are kept and still populated, because career.v1 is additive-only and a published field is not edited in place. A reader that uses them gets the lossy answer, which is why the replacement sits beside them rather than quietly reusing a number. |
| `priorByEmail` | `int32` | number |  | Deprecated: use prior_sittings. |
| `priorByCookie` | `int32` | number |  | Deprecated: use prior_sittings. |
| `keyVersion` | `string` | string |  | Which answer key graded this run. Part of the provenance triple with instrument_version and item_set_version. |
| `wantsResults` | `bool` | boolean |  | Whether the participant asked for their results. |
| `finishedAt` | `string` | string |  | When it ended, RFC3339, empty while still running. |
| `reviewedByName` | `string` | string |  | Who signed the current verdict. A curated dataset whose exclusions cannot be attributed has anonymous decisions in it. |
| `priorSittings` | [`DecisionTestPriorSittings`](#decisiontestpriorsittings) | object |  | What each identity could see when the run started, as recorded. A message rather than three scalars, because a message has natural presence: each field can distinguish "this link existed and saw nothing" from "there was no such link", which is the entire reason three observations are stored instead of one derived number. |

### DecisionTestPriorSittings

Prior sittings each identity could see at the moment a run started.

Point-in-time observations, not a conclusion. The sitting number is
derived from them in SQL (dt_attempt_no and friends), so the sequence
can be recomputed under a different rule later without the raw
observations having been thrown away.

Every field is `optional`: absent means there was no such link, and
zero means there was one and it saw no earlier sittings. Collapsing
those two would manufacture a fact about a person.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `byAccount` | `int32` | number |  | _(oneof `_by_account`)_ Prior sittings visible by account at the moment this run started. |
| `byEmail` | `int32` | number |  | _(oneof `_by_email`)_ Prior sittings visible by email address at the moment this started. |
| `byCookie` | `int32` | number |  | _(oneof `_by_cookie`)_ Prior sittings visible by visitor cookie at the moment this started. |

### DecisionTestDrawnItem

One of a run's thirty questions, as drawn.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `position` | `int32` | number |  | Position in the whole test, 1-based. |
| `blockNo` | `int32` | number |  | Which block it falls in. |
| `itemCode` | `string` | string |  | Stable item code. |
| `family` | `string` | string |  | arithmetic, base_rate, conjunction or syllogism. |
| `answered` | `bool` | boolean |  | Whether the participant ever reached it. The reason this list exists separately from the answers: an abandoned run has six answers and says nothing about the twenty-four questions that had already been chosen for it. |
| `fixture` | `bool` | boolean |  | A placeholder from the fixture bank rather than a real item. Always false in production, where fixtures are refused. |

### DecisionTestItemPool

One category's room to vary.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `category` | `string` | string |  | arithmetic, base_rate, conjunction or syllogism. |
| `realItems` | `int32` | number |  | Active scored items in this category, real ones only. |
| `fixtures` | `int32` | number |  | Placeholders, which production never serves. |
| `perTest` | `int32` | number |  | How many a single test consumes. |
| `spare` | `int32` | number |  | real_items minus per_test. Negative means a test cannot be filled. |
| `canVary` | `bool` | boolean |  | Whether there is anything to leave behind. False means every participant is served the same questions however random the selection claims to be, which is the failure that looks exactly like success. |

### GetDecisionTestAnalysisRequest

Asks for the decision test's analysis views.

_No fields._

### GetDecisionTestAnalysisResponse

The three views that answer the research question.

Each is grouped by instrument, because runs taken under different
items or different timings are different instruments and must never
pool into one claim (FR-DT-15).

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `loadCurve` | [`DecisionTestLoadPoint`](#decisiontestloadpoint)[] | array of object |  | Accuracy and confidence by load level. |
| `calibration` | [`DecisionTestCalibrationPoint`](#decisiontestcalibrationpoint)[] | array of object |  | What each confidence band actually achieved. |
| `thresholds` | [`DecisionTestThreshold`](#decisiontestthreshold)[] | array of object |  | Per participant, the lowest load at which they were confidently wrong. |

### DecisionTestLoadPoint

One load level on the curve.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `itemSetVersion` | `string` | string |  | Which items, so two item sets never pool. |
| `instrumentVersion` | `string` | string |  | Which timings, so two instruments never pool. |
| `load` | `string` | string |  | d3, d4, d4_plus1, d4_plus3 or d3_control. |
| `loadRank` | `int32` | number |  | Ordering on the ramp. d3_control ranks 1, the same as d3, because it is d3: block 5 returns to block 1's difficulty. |
| `sessions` | `int32` | number |  | Runs contributing to this point. |
| `sessionsUnreviewed` | `int32` | number |  | Of those, how many nobody has reviewed yet. |
| `sessionsFirstAttempt` | `int32` | number |  | First sittings, and repeats, counted apart. |
| `sessionsRepeat` | `int32` | number |  | Second or later sittings by the same participant. |
| `answers` | `int32` | number |  | Answers behind this point. |
| `accuracyPct` | `int32` | number |  | Correct, as a percentage. |
| `lurePct` | `int32` | number |  | How often the intended wrong answer was taken. |
| `expiryPct` | `int32` | number |  | How often the clock ran out. |
| `meanConfidence` | `int32` | number |  | Mean reported confidence. |
| `gapPct` | `int32` | number |  | Confidence minus accuracy. Positive is more sure than right, which is the finding this instrument exists to measure. |
| `confidentlyWrongPct` | `int32` | number |  | Wrong and reported at or above the confident threshold. |
| `meanBrier` | `double` | number |  | Mean Brier score, 0 is perfect calibration. |
| `families` | `string` | string |  | The item families behind this point, joined with '+'. One family means the point measures the family rather than the load. |

### DecisionTestCalibrationPoint

One confidence band at one load.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `itemSetVersion` | `string` | string |  | Which items. |
| `instrumentVersion` | `string` | string |  | Which timings. |
| `load` | `string` | string |  | The load this band was measured at. |
| `loadRank` | `int32` | number |  | Ordering on the ramp. |
| `confidenceBand` | `int32` | number |  | The band, in tens: 10 is 10 to 19 percent confident. |
| `answers` | `int32` | number |  | Answers in this band. |
| `accuracyPct` | `int32` | number |  | What the band actually achieved. |
| `overclaimPct` | `int32` | number |  | Confidence minus accuracy for this band. Positive is overclaiming. |
| `meanBrier` | `double` | number |  | Mean Brier score for the band. |

### DecisionTestThreshold

One participant's threshold.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string |  | The run, so the console can link to it. |
| `attemptNo` | `int32` | number |  | Which sitting this was. Zero means no identity could place it. |
| `reviewStatus` | `string` | string |  | Curation status, empty where nobody has judged it. |
| `accuracyPct` | `int32` | number |  | Accuracy across the run. |
| `gapPct` | `int32` | number |  | Confidence minus accuracy across the run. |
| `fatigueDeltaPct` | `int32` | number |  | Block 5 against block 1, which are the same difficulty. Negative means accuracy fell by the end. |
| `luredAtRank` | `int32` | number |  | The lowest rank on the ramp at which they were confidently lured. Zero means it never happened. |
| `luredAtLoad` | `string` | string |  | That rank's load, as a name. |
| `luredInControl` | `bool` | boolean |  | Whether it happened in block 5, the fatigue control. That is a different claim from being lured on the hardest block. |
| `confidentlyWrong` | `int32` | number |  | How many answers in the run were wrong and confident. |

### ExportDecisionTestRunRequest

Asks for one run as downloadable files.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | Which run, from the list. |
| `includeKey` | `bool` | boolean |  | Add the prompt, the chosen option and the correct option to the answers file. Off by default, matching the collapsed section on the run page. An answer key that escapes contaminates a standardized instrument permanently, and a file is easier to forward than a screen. The whole-dataset export never carries these at all (FR-DT-16); this is one run, downloaded deliberately. |

### ExportDecisionTestRunResponse

One run as two CSV files.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `filenameStem` | `string` | string |  | Base filename, without an extension or a suffix. |
| `blocksCsv` | `string` | string |  | The by-block table, one row per block, with the session context on every row so the file stands alone. |
| `answersCsv` | `string` | string |  | The every-answer table, one row per answer, same context. |
| `blockRows` | `int32` | number |  | Rows in the blocks file, excluding its header. |
| `answerRows` | `int32` | number |  | Rows in the answers file, excluding its header. |

### GetDecisionTestRunRequest

Asks for one run in full.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From the list. |

### GetDecisionTestRunResponse

One run in full.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `run` | [`DecisionTestRun`](#decisiontestrun) | object |  | The run's summary. |
| `blocks` | [`DecisionTestBlock`](#decisiontestblock)[] | array of object |  | One row per block, in running order. |
| `answers` | [`DecisionTestAnswer`](#decisiontestanswer)[] | array of object |  | Every answer, in presentation order. |
| `reviewHistory` | [`DecisionTestReviewEvent`](#decisiontestreviewevent)[] | array of object |  | Every judgement ever recorded for this run, oldest first. |
| `drawnItems` | [`DecisionTestDrawnItem`](#decisiontestdrawnitem)[] | array of object |  | The thirty questions this run was drawn, in order, including any it never reached. |

### ExportDecisionTestDataRequest

Asks for the dataset as CSV.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includeSynthetic` | `bool` | boolean |  | Include runs driven by an agent rather than a person. Off by default: a synthetic run is useful for checking the instrument and ruinous if it is averaged into a claim about people, so including it has to be asked for. |
| `includeExcluded` | `bool` | boolean |  | Include runs and blocks the owner marked do_not_use. Off by default, because curation that does not reach the export is decoration: the export is the thing that leaves for analysis, and shipping a run marked unusable is worse than not curating at all, since the mark creates a belief the data has been cleaned. |

### ReviewDecisionTestRunRequest

Records a curation judgement.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | Which run. |
| `blockNo` | `int32` | number | `int32: lte: 5 gte: 0` | 0 judges the whole run; 1 to 5 judges one block, so a single spoiled block does not cost the other four. |
| `status` | `string` | string | `string: max_len: 24` | good, incomplete, hold or do_not_use. Empty clears a run's review back to unreviewed, and for a block removes the judgement altogether, since a block has no stored unreviewed state. |
| `reason` | `string` | string | `string: max_len: 32` | instrument_fault, participant_reported, duplicate or other, and only alongside do_not_use. The first two are the split that earns this field: one is a bug to go and repair, the other is nothing to fix and simply costs a data point. |
| `note` | `string` | string | `string: max_len: 2000` | Free text, for what the participant said or what was observed. |

### ReviewDecisionTestRunResponse

The run as it now reads.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `run` | [`DecisionTestRun`](#decisiontestrun) | object |  | The updated summary, so the caller does not have to refetch to render the new state. |

### ExportDecisionTestDataResponse

The dataset.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `csv` | `string` | string |  | The whole file, RFC 4180, header row first. Thirty rows per completed run, so this stays small enough to hand over in one response for as long as the volunteer count is realistic. |
| `filename` | `string` | string |  | Suggested filename, carrying the date so two exports do not overwrite each other in a downloads folder. |
| `rows` | `int32` | number |  | Rows in the body, not counting the header. |
| `sessions` | `int32` | number |  | Runs those rows came from. |

### DecisionTestReviewCounts

How many real runs sit in each curation state.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `total` | `int32` | number |  | Every real run. Synthetic runs are excluded throughout: they await nobody's judgement and counting them would invent work. |
| `unreviewed` | `int32` | number |  | Nobody has looked at it yet. |
| `good` | `int32` | number |  | Judged usable. |
| `incomplete` | `int32` | number |  | Did not finish, and not thereby excluded: whether an interrupted run is usable is judged per run. |
| `hold` | `int32` | number |  | Something is odd and the reviewer has not decided. |
| `doNotUse` | `int32` | number |  | Excluded from every view that makes a claim about people, and from the export by default. Never deleted. |

### DecisionTestBlock

One block of a run.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `blockNo` | `int32` | number |  | 1 to 5. |
| `load` | `string` | string |  | d3, d4, d4_plus1, d4_plus3 or d3_control. |
| `correct` | `int32` | number |  | Correct answers in this block. |
| `total` | `int32` | number |  | Questions in this block. |
| `lure` | `int32` | number |  | Intuitive wrong answers chosen. |
| `expired` | `int32` | number |  | Questions that ran out of time. |
| `meanConfidence` | `int32` | number |  | Mean confidence in this block, 0 to 100. |
| `meanLatencyMs` | `int32` | number |  | Mean time to answer, in milliseconds. |
| `presentedDigits` | `string` | string |  | The number that was shown. |
| `expectedDigits` | `string` | string |  | What a correct response was, after any transformation. |
| `responseDigits` | `string` | string |  | What came back. |
| `recallOutcome` | `string` | string |  | exact, untransformed, wrong_digits, partial or expired. untransformed means the number survived and the operation did not. |
| `memoryFailurePct` | `int32` | number |  | How much of the number was lost, 0 to 100. Zero is a number held intact, 100 one lost entirely. Scored against whichever of the presented or expected number the response is closer to, so an untransformed answer reads as perfect retention rather than as total loss. Negative where the recall expired and nothing was attempted, which is not a memory failure of any size. |
| `reviewStatus` | `string` | string |  | Curation for this block alone, so one spoiled block costs six answers rather than thirty. Empty means nobody has judged it. |
| `reviewNote` | `string` | string |  | The note attached to this block's judgement. |
| `digitsCorrect` | `int32` | number |  | Digit positions matching the expected number, after any transformation the block required. |
| `digitsHeld` | `int32` | number |  | Digit positions that survived, scored against whichever of the presented or the expected number the response is closer to. This is what memory_failure_pct is derived from, and why an untransformed answer reads as full retention: the number was held and only the operation failed. Not the length of the number. |
| `recallLatencyMs` | `int32` | number |  | How long the participant took to give the number back, in ms. |
| `meanLatencyVsBaseline` | `double` | number |  | Mean latency for this block in units of the tap-check offset, from v_dt_blocks. Where the ratio earns its place: the load effect is the research question, and comparing block 1 with block 4 across participants needs each person's answers on their own scale. |

### DecisionTestAnswer

One answer.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `position` | `int32` | number |  | Position in the whole test, 1-based. |
| `blockNo` | `int32` | number |  | Which block it belonged to. |
| `itemCode` | `string` | string |  | Stable item code, e.g. A1 or D4. |
| `itemFamily` | `string` | string |  | arithmetic, base_rate, conjunction or syllogism. |
| `outcome` | `string` | string |  | correct, lure, other or expired. |
| `confidence` | `int32` | number |  | The participant's own rating, 0 to 100. |
| `latencyMs` | `int32` | number |  | Time to answer, in milliseconds. |
| `prompt` | `string` | string |  | The question as it was put. |
| `chosenText` | `string` | string |  | The option the participant chose, as text. Empty on an expiry, where nothing was chosen at all. |
| `correctText` | `string` | string |  | The option that was correct, as text. Admin-only and deliberately so. This is never served to a participant and never leaves in the CSV export, whose view carries no chosen_index and no prompt precisely so the answer key cannot be reconstructed from it (FR-DT-16). The owner built the instrument and cannot judge whether a run is usable without seeing what was actually picked. |
| `chosenIndex` | `int32` | number |  | Zero-based index of the chosen option, -1 where it expired. |
| `positionInBlock` | `int32` | number |  | Position within its own block, 1-based. |
| `itemVersion` | `int32` | number |  | Which revision of the item was served. |
| `isLure` | `bool` | boolean |  | The intuitive wrong answer was taken. |
| `confidentlyWrong` | `bool` | boolean |  | Wrong, and reported at or above the confident threshold. This is the research question in one field. |
| `confidentlyLured` | `bool` | boolean |  | Took the lure and was confident about it. |
| `latencyVsBaseline` | `double` | number |  | Time to answer against this participant's own unloaded baseline, as a ratio. 1.0 is baseline speed. |
| `brier` | `double` | number |  | Brier score for this answer, 0 is perfect calibration and 1 is maximally wrong. Negative where it could not be scored. |

### DecisionTestReviewEvent

One entry in a run's curation history.

Every judgement is appended, including a clearing, so an exclusion
can always be explained. Written since migration 00056 and readable
only through the SQL console until now.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `blockNo` | `int32` | number |  | 0 for a verdict on the whole run, 1 to 5 for one block. |
| `status` | `string` | string |  | The status recorded, or "cleared" where a block judgement was removed. |
| `reason` | `string` | string |  | Why, where one was given. |
| `note` | `string` | string |  | The reviewer's note at the time. |
| `reviewedBy` | `string` | string |  | Who recorded it. |
| `createdAt` | `string` | string |  | When, RFC3339. |

### RegisterRequest

Registration form.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string | `string: min_len: 1 max_len: 120` | Display name. |
| `email` | `string` | string | `string: max_len: 254 email: true` | Email address; becomes the sign-in identifier. |
| `password` | `string` | string | `string: min_len: 12 max_len: 256` | Password: at least 12 characters, no composition rules; rejected if it appears in a breached-password list. |
| `organization` | `string` | string | `string: max_len: 200` | Organization, optional. |
| `statedRole` | `string` | string | `string: max_len: 120` | Stated role or title, optional. |
| `consentVersion` | `string` | string | `string: min_len: 1 max_len: 32` | Version identifier of the consent text shown; must match the current version served on the registration page. |
| `consentAccepted` | `bool` | boolean | `bool: const: true` | Explicit acknowledgement that activity and assistant conversations are stored and visible to the owner. Must be true. |
| `turnstileToken` | `string` | string | `string: min_len: 1 max_len: 2048` | Cloudflare Turnstile response token from the registration page. |

### RegisterResponse

Always the same body; see Register.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | `string` | string |  | Fixed text for display: "Check your email for a verification link." |

### VerifyRequest

Verification by link token or by code.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `token` | `string` | string | `string: min_len: 16 max_len: 256` | _(oneof `credential`)_ Single-use token from the emailed link. |
| `code` | `string` | string | `string: pattern: "^[0-9]{6$"` | _(oneof `credential`)_ Six-digit code from the email; requires `email`. |
| `email` | `string` | string | `string: max_len: 254` | Email address the code was sent to; required with `code`, ignored with `token`. |

### VerifyResponse

Verification result; the session cookie is set on this response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The now-active member. |
| `questionnairePending` | `bool` | boolean |  | True when the first-visit questionnaire has not been completed; the client should route to `/me/interests`. |

### ResendVerificationRequest

Resend request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `email` | `string` | string | `string: max_len: 254 email: true` | Email address of the unverified account. |

### ResendVerificationResponse

Always the same body.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | `string` | string |  | Fixed text for display. |

### LoginRequest

Sign-in form.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `email` | `string` | string | `string: max_len: 254 email: true` | Email address. |
| `password` | `string` | string | `string: min_len: 1 max_len: 256` | Password. |
| `turnstileToken` | `string` | string | `string: max_len: 2048` | Turnstile token; required after repeated failures for the account or IP (the server responds `failed_precondition` with reason `turnstile_required` when it is missing but needed). |

### LoginResponse

Sign-in result; the session cookie is set on this response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The signed-in member. `status` is UNVERIFIED for an account that has not confirmed its email; such a session can only call verification methods. |
| `mfaRequired` | `bool` | boolean |  | True for an admin session that must complete MfaVerify (or MfaEnroll then MfaVerify) before admin methods are available. |
| `questionnairePending` | `bool` | boolean |  | True when the first-visit questionnaire has not been completed. |

### LogoutRequest

Empty.

_No fields._

### LogoutResponse

Empty.

_No fields._

### LogoutAllRequest

Empty.

_No fields._

### LogoutAllResponse

Number of sessions revoked.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `revoked` | `int32` | number |  | Sessions revoked, including the current one. |

### ForgotPasswordRequest

Password-reset request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `email` | `string` | string | `string: max_len: 254 email: true` | Email address of the account. |

### ForgotPasswordResponse

Always the same body.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | `string` | string |  | Fixed text for display. |

### ResetPasswordRequest

Password-reset completion.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `token` | `string` | string | `string: min_len: 16 max_len: 256` | Single-use token from the reset email. |
| `newPassword` | `string` | string | `string: min_len: 12 max_len: 256` | New password, same rules as registration. |

### ResetPasswordResponse

Reset result; the session cookie is set on this response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The signed-in member. |

### ChangePasswordRequest

Password change for a signed-in member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `currentPassword` | `string` | string | `string: min_len: 1 max_len: 256` | Current password. |
| `newPassword` | `string` | string | `string: min_len: 12 max_len: 256` | New password, same rules as registration. |

### ChangePasswordResponse

Number of other sessions revoked.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `revoked` | `int32` | number |  | Other sessions revoked. |

### ChangeEmailRequest

Email change for a signed-in member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `newEmail` | `string` | string | `string: max_len: 254 email: true` | New email address; a verification email is sent there. |
| `currentPassword` | `string` | string | `string: max_len: 256` | Current password; required unless the account is social-only. |

### ChangeEmailResponse

Always the same body.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `message` | `string` | string |  | Fixed text for display. |

### MfaEnrollRequest

Empty.

_No fields._

### MfaEnrollResponse

TOTP enrolment material. Shown once; the secret is not retrievable later.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `otpauthUri` | `string` | string |  | otpauth:// URI for authenticator apps. |
| `secretBase32` | `string` | string |  | The same secret in Base32 for manual entry. |
| `recoveryCodes` | `string`[] | array of string |  | One-time recovery codes. |

### MfaVerifyRequest

TOTP verification.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `code` | `string` | string | `string: min_len: 6 max_len: 32` | Six-digit TOTP code or a recovery code. |

### MfaVerifyResponse

Verification result.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The admin member with `mfa_enrolled` true. |
| `recoveryCodesRemaining` | `int32` | number |  | Recovery codes remaining, when a recovery code was used. |

### ListContentRequest

List filters.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `type` | [`ContentType`](#contenttype) | string (enum name) | `enum: defined_only: true` | Restrict to one content type; unspecified returns all types. |
| `trackIds` | `string`[] | array of string | `repeated: max_items: 8 items: string: max_len: 64 pattern: "^[a-z0-9-]+$"` | Restrict to items with a non-zero weight on any of these tracks. |
| `tags` | `string`[] | array of string | `repeated: max_items: 8 items: string: max_len: 64` | Restrict to items carrying all of these tags. |
| `order` | [`ListContentRequest.Order`](#listcontentrequestorder) | string (enum name) | `enum: defined_only: true` | Ordering; DEFAULT follows the member's tailoring setting. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### ListContentResponse

List page.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`ContentSummary`](#contentsummary)[] | array of object |  | Items in the requested order. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |
| `tailored` | `bool` | boolean |  | True when the order was tailored to the member's tracks. |

### GetContentRequest

Detail request by slug or ID.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `slug` | `string` | string | `string: min_len: 1 max_len: 128` | _(oneof `key`)_ URL slug, e.g. `mdemg`. |
| `id` | `string` | string | `string: min_len: 1 max_len: 128` | _(oneof `key`)_ Content ID, e.g. `prj-mdemg`. |

### Accomplishment

A quantified accomplishment within a role.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `text` | `string` | string |  | Headline statement. |
| `metric` | `string` | string |  | The number or result, e.g. "45% fewer equipment failures". |
| `detail` | `string` | string |  | Optional situation / action / result detail (Markdown). |
| `tracks` | [`TrackWeight`](#trackweight)[] | array of object |  | Track weights for this accomplishment. |

### RoleDetail

Role-specific detail.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `organization` | `string` | string |  | Employer. |
| `location` | `string` | string |  | Location. |
| `start` | `Timestamp` | string (RFC 3339, UTC) |  | Start date. |
| `end` | `Timestamp` | string (RFC 3339, UTC) |  | End date; unset means present. |
| `accomplishments` | [`Accomplishment`](#accomplishment)[] | array of object |  | Accomplishments in display order. |
| `technologies` | `string`[] | array of string |  | Technologies and systems. |

### GithubMeta

Live repository metadata cached from GitHub.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `stars` | `int32` | number |  | Star count. |
| `language` | `string` | string |  | Primary language. |
| `pushedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Last push. |
| `fetchedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the cache was refreshed. |

### ProjectDetail

Project-specific detail.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `role` | `string` | string |  | The owner's role on the project. |
| `status` | [`ProjectDetail.Status`](#projectdetailstatus) | string (enum name) |  | Lifecycle status. |
| `repoUrl` | `string` | string |  | Repository URL, when public. |
| `stack` | `string`[] | array of string |  | Technology stack. |
| `problem` | `string` | string |  | Problem statement (Markdown). |
| `approach` | `string` | string |  | Approach (Markdown). |
| `outcomes` | `string`[] | array of string |  | Outcomes. |
| `diagramMermaid` | `string` | string |  | Architecture diagram as Mermaid source, when available. |
| `github` | [`GithubMeta`](#githubmeta) | object |  | GitHub metadata, when the repository is public and the cache is warm. |

### ArticleDetail

Article-specific detail.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `venue` | `string` | string |  | Where it was published, e.g. "LinkedIn". |
| `canonicalUrl` | `string` | string |  | Canonical URL at the venue, when any. |
| `series` | `string` | string |  | Series name, when part of one. |
| `seriesOrder` | `int32` | number |  | Position within the series (1-based). |
| `readingTimeMinutes` | `int32` | number |  | Estimated reading time in minutes. |
| `hosted` | `bool` | boolean |  | True when the full text is hosted on this site (`body` is populated). |

### PresentationDetail

Presentation-specific detail.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `event` | `string` | string |  | Event or venue. |
| `abstract` | `string` | string |  | Abstract (Markdown). |
| `slidesCount` | `int32` | number |  | Number of slides or pages. |
| `fileUrl` | `string` | string |  | Short-lived signed URL of the PDF for the in-page viewer. |
| `fileUrlExpiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | Expiry of `file_url`. |

### CredentialDetail

Credential-specific detail.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`CredentialDetail.Kind`](#credentialdetailkind) | string (enum name) |  | Kind of credential. |
| `issuer` | `string` | string |  | Issuing organization. |
| `date` | `Timestamp` | string (RFC 3339, UTC) |  | Date awarded or completed. |

### ImageVariant

A responsive image rendition.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `url` | `string` | string |  | URL of the rendition. |
| `width` | `int32` | number |  | Width in pixels. |
| `height` | `int32` | number |  | Height in pixels. |
| `mimeType` | `string` | string |  | MIME type, e.g. `image/avif`. |

### PhotoDetail

Photo-specific detail. EXIF is stripped at build time; originals are never served.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `variants` | [`ImageVariant`](#imagevariant)[] | array of object |  | Renditions from small to large. |
| `caption` | `string` | string |  | Caption. |
| `context` | `string` | string |  | Context: where and what is happening. |
| `taken` | `Timestamp` | string (RFC 3339, UTC) |  | When the photo was taken (month precision is typical). |
| `alt` | `string` | string |  | Alt text for accessibility. |

### ResumeDetail

Résumé-specific detail. Download goes through DownloadService.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `variant` | `string` | string |  | Variant identifier (DownloadItem.variant). |
| `version` | `string` | string |  | Version label. |

### QaDetail

Q&A-specific detail.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `question` | `string` | string |  | The question. |
| `answer` | `string` | string |  | The owner's answer (Markdown). |

### Asset

A file attached to an item (diagram, image, PDF).

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | `string` | string |  | Kind of asset, e.g. `diagram`, `image`, `pdf`. |
| `url` | `string` | string |  | URL (signed when the asset lives in object storage). |
| `alt` | `string` | string |  | Alt text or caption. |

### ContentItem

A content item in full.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `summary` | [`ContentSummary`](#contentsummary) | object |  | Summary fields. |
| `body` | `string` | string |  | Body as Markdown/MDX (empty for items without a body, such as photos). |
| `related` | [`ContentSummary`](#contentsummary)[] | array of object |  | Related items. |
| `assets` | [`Asset`](#asset)[] | array of object |  | Attached assets. |
| `role` | [`RoleDetail`](#roledetail) | object |  | _(oneof `detail`)_ Present when `summary.type` is ROLE. |
| `project` | [`ProjectDetail`](#projectdetail) | object |  | _(oneof `detail`)_ Present when `summary.type` is PROJECT. |
| `article` | [`ArticleDetail`](#articledetail) | object |  | _(oneof `detail`)_ Present when `summary.type` is ARTICLE. |
| `presentation` | [`PresentationDetail`](#presentationdetail) | object |  | _(oneof `detail`)_ Present when `summary.type` is PRESENTATION. |
| `credential` | [`CredentialDetail`](#credentialdetail) | object |  | _(oneof `detail`)_ Present when `summary.type` is CREDENTIAL. |
| `photo` | [`PhotoDetail`](#photodetail) | object |  | _(oneof `detail`)_ Present when `summary.type` is PHOTO. |
| `resume` | [`ResumeDetail`](#resumedetail) | object |  | _(oneof `detail`)_ Present when `summary.type` is RESUME. |
| `qa` | [`QaDetail`](#qadetail) | object |  | _(oneof `detail`)_ Present when `summary.type` is QA. |

### GetContentResponse

Detail response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `item` | [`ContentItem`](#contentitem) | object |  | The item. |

### WhatsNewRequest

What's-new request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `since` | `Timestamp` | string (RFC 3339, UTC) |  | Return items changed after this time; unset uses the member's previous visit, or 90 days ago on a first visit. |
| `limit` | `int32` | number | `int32: lte: 50 gte: 0` | Maximum items, 1–50; zero selects 20. |

### WhatsNewResponse

What's-new response.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`ContentSummary`](#contentsummary)[] | array of object |  | Items, newest change first. |
| `since` | `Timestamp` | string (RFC 3339, UTC) |  | The effective `since` used. |

### ListTracksRequest

Empty.

_No fields._

### ListTracksResponse

The track taxonomy.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `tracks` | [`Track`](#track)[] | array of object |  | Tracks in display order. |

### GetSkillsRequest

Empty.

_No fields._

### Skill

One skill with evidence.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Skill ID. |
| `name` | `string` | string |  | Name. |
| `level` | `int32` | number |  | Depth on a 1–5 scale. |
| `years` | `int32` | number |  | Years of experience. |
| `evidence` | [`ContentSummary`](#contentsummary)[] | array of object |  | Evidence: content items that demonstrate the skill (at least one). |
| `tracks` | [`TrackWeight`](#trackweight)[] | array of object |  | Track weights. |

### SkillCategory

Skills grouped by category.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string |  | Category name, e.g. "Industrial automation, process control & OT". |
| `skills` | [`Skill`](#skill)[] | array of object |  | Skills in display order. |

### GetSkillsResponse

The skills matrix.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `categories` | [`SkillCategory`](#skillcategory)[] | array of object |  | Categories in display order, tailored to the member's tracks when tailoring is on. |

### SearchRequest

Search request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `query` | `string` | string | `string: min_len: 1 max_len: 200` | Query text. |
| `type` | [`ContentType`](#contenttype) | string (enum name) | `enum: defined_only: true` | Restrict to one content type; unspecified searches all types. |
| `trackIds` | `string`[] | array of string | `repeated: max_items: 8 items: string: max_len: 64 pattern: "^[a-z0-9-]+$"` | Restrict to items weighted on any of these tracks. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### SearchHit

One search hit.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `item` | [`ContentSummary`](#contentsummary) | object |  | The item. |
| `snippet` | `string` | string |  | Snippet with the match highlighted using `<mark>` tags; safe to render as HTML. |
| `score` | `float` | number |  | Relevance score (higher is better; not comparable across queries). |

### SearchResponse

Search results.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `hits` | [`SearchHit`](#searchhit)[] | array of object |  | Hits in descending score. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

### Intake

Optional demographics, collected after the effort warning and posted
with the session rather than on keystroke, so somebody who reads the
warning and leaves has given nothing.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `displayName` | `string` | string | `string: max_len: 120` | Free text, optional like every field here. |
| `ageRange` | `string` | string | `string: max_len: 40` | Banded rather than a date of birth; the analysis never needs more. |
| `education` | `string` | string | `string: max_len: 60` | One of the owner's nine options, stored as given. |
| `occupation` | `string` | string | `string: max_len: 120` | Free text, because an occupation list is always wrong for somebody. |
| `email` | `string` | string | `string: max_len: 200` | An identifier as well as a contact: it is how a repeat participant is recognised, which the privacy policy states in those terms. |
| `wantsResults` | `bool` | boolean |  | Results are sent only to somebody who asked for them. |

### Conditions

What the participant's device and setup were, which splits the sample
and therefore cannot be inferred later.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `audioMode` | `string` | string | `string: max_len: 16` | sound or visual. Visual replaces the tick with a heartbeat icon for a participant who cannot use audio. |
| `deviceClass` | `string` | string | `string: max_len: 16` | desktop, tablet or phone. Phones are allowed and recorded rather than refused. |
| `tapCheckPassed` | `bool` | boolean |  | Whether the tap-along check was passed. Observed rather than self-reported. |
| `baselineRtMs` | `int32` | number | `int32: gte: 0` | Unloaded reaction time from the tap check, which turns latency from an absolute into a relative measure. |
| `baselineRtSdMs` | `int32` | number | `int32: gte: 0` | Variability of those taps, a cheap read on baseline attention. |

### StartSessionRequest

Opens a session.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `intake` | [`Intake`](#intake) | object |  | Demographics, every field optional. |
| `conditions` | [`Conditions`](#conditions) | object |  | Device and setup, measured before the first question. |
| `synthetic` | `bool` | boolean |  | True when an agent is driving the UI rather than a person, so the rows are excluded from analysis by filter. Separate from the instrument version because the most useful agent run is against the exact version production serves. |

### Timings

The timings this run is bound by, served to the client rather than
hard-coded in it.

They live in app_settings and are editable from the admin console,
because the first two live runs each moved them and each move cost a
deploy. They are returned per session so a run cannot drift from the
settings it started under.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `memoriseMs` | `int32` | number |  | How long the number to hold is shown. |
| `questionMs` | `int32` | number |  | Hard limit per question, covering reading, deciding and rating. |
| `recallMs` | `int32` | number |  | Limit on entering the number at the end of a block. |

### StartSessionResponse

The opened session.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string |  | Opaque handle for the rest of the run. Not a database id. |
| `practice` | [`Block`](#block) | object |  | The practice block, which carries the tick so a participant hears it before anything is scored. |
| `blockCount` | `int32` | number |  | How many scored blocks follow. |
| `questionCount` | `int32` | number |  | How many scored questions in total, for the progress counter. |
| `timings` | [`Timings`](#timings) | object |  | The timings in force for this run. |

### GetBlockRequest

Asks for a block by number.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From StartSessionResponse. |
| `blockNo` | `int32` | number | `int32: lte: 5 gte: 1` | One-based. Block five returns to block one's difficulty and is the fatigue control. |

### GetBlockResponse

One block: a number to hold, and the questions to answer while
holding it.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `block` | [`Block`](#block) | object |  | The block. |

### Block

A number to memorise and the questions answered under it.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `blockNo` | `int32` | number |  | One-based, or zero for the practice block. |
| `load` | `string` | string |  | d3, d4, d4_plus1, d4_plus3 or d3_control. |
| `digits` | `string` | string |  | The digits shown for two seconds. The expected response is derived server-side and never sent. |
| `transform` | `string` | string |  | How the digits must be transformed before they are returned: none, plus1 or plus3. Shown to the participant as an instruction; the answer is still computed in the api. |
| `questions` | [`Question`](#question)[] | array of object |  | The questions, in order. |

### Question

One question as the participant sees it. No correct answer, no lure
marker, nothing that identifies which option is right.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `code` | `string` | string |  | Stable code, e.g. A1 or D4. Used to tie an answer to an item without exposing the bank. |
| `version` | `int32` | number |  | Version of the item text, so a reworded item does not pool with its predecessor. |
| `prompt` | `string` | string |  | The question text. |
| `reminder` | `string` | string |  | A standing one-line framing, empty for most items. The syllogisms carry "Assume both statements are true" because this test degrades the working memory that instructions live in. |
| `options` | `string`[] | array of string |  | Options in presentation order. Position is fixed for everyone because the test is standardized. |
| `positionOverall` | `int32` | number |  | Position in the whole test, one-based, for the progress counter. |

### SubmitAnswerRequest

Records one answer.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From StartSessionResponse. |
| `blockNo` | `int32` | number | `int32: lte: 5 gte: 1` | Which block this answer belongs to. |
| `positionOverall` | `int32` | number | `int32: gte: 1` | Position in the whole test, one-based. |
| `chosenIndex` | `int32` | number | `int32: gte: -1` | Index into the question's options, or -1 when the question expired unanswered. Expiry is its own outcome rather than an error, because a timeout under load is a result. |
| `latencyMs` | `int32` | number | `int32: gte: 0` | Milliseconds from the question appearing to the answer committed. |
| `confidence` | `int32` | number | `int32: lte: 100 gte: 0` | The participant's own rating, 0 to 100, on every question. Error rate alone cannot answer a question about being wrong *and* sure. |

### SubmitAnswerResponse

Acknowledgement. Deliberately carries no verdict.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `stored` | `bool` | boolean |  | True when the answer was stored. Never says whether it was right. |

### SubmitRecallRequest

Records the digits returned at the end of a block.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From StartSessionResponse. |
| `blockNo` | `int32` | number | `int32: lte: 5 gte: 1` | Which block is being closed. |
| `digits` | `string` | string | `string: max_len: 16` | What the participant typed, empty when the step expired. |
| `latencyMs` | `int32` | number | `int32: gte: 0` | Milliseconds spent on the recall step. |

### SubmitRecallResponse

Acknowledgement. A missed recall is a data point, not a failure, and
the block's answers are kept either way.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `stored` | `bool` | boolean |  | True when the recall was stored. |

### FinishSessionRequest

Closes the session.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `sessionKey` | `string` | string | `string: max_len: 64` | From StartSessionResponse. |
| `recallStrategy` | `string` | string | `string: max_len: 16` | Whether the participant converted the number at encoding or carried it and transformed at recall. Asked in the debrief because the two produce different loads during the questions, which turns an uncontrolled variable into a recorded one. |

### FinishSessionResponse

The closed session.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `correct` | `int32` | number |  | How many of the thirty were answered correctly. The only figure a participant is ever shown, and never item by item. |
| `total` | `int32` | number |  | How many were scored, so the figure reads as "N of M". |

### ListDownloadsRequest

Empty.

_No fields._

### DownloadItem

A downloadable file.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `variant` | `string` | string |  | Variant identifier used in the download path, e.g. `automation-control`. |
| `title` | `string` | string |  | Display title, e.g. "Résumé — Industrial automation & process control". |
| `description` | `string` | string |  | One-line description. |
| `version` | `string` | string |  | Version label, e.g. `2026.09`. |
| `trackIds` | `string`[] | array of string |  | Tracks the variant targets. |
| `sizeBytes` | `int64` | string (decimal) |  | File size in bytes. |
| `mimeType` | `string` | string |  | MIME type, e.g. `application/pdf`. |
| `updatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | Last update. |
| `recommended` | `bool` | boolean |  | True for the variant recommended for the member. |
| `downloadPath` | `string` | string |  | Download path: `GET /api/downloads/{variant}`. |

### ListDownloadsResponse

Download list.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`DownloadItem`](#downloaditem)[] | array of object |  | Items, recommended first. |

### BrowserEvent

One browser event.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `eventId` | `string` | string | `string: min_len: 8 max_len: 64` | Client-minted UUID; a retried batch de-duplicates on it. |
| `name` | `string` | string | `string: min_len: 1 max_len: 64` | Registry name (page.view, page.leave, landing.cta_click, ...). |
| `clientTsMs` | `int64` | string (decimal) |  | Client clock in milliseconds since the epoch; kept for skew analysis, never used for ordering. |
| `path` | `string` | string | `string: max_len: 512` | Page path (no query string). |
| `referrer` | `string` | string | `string: max_len: 1024` | document.referrer, reduced to its host by the api. |
| `uiMode` | `string` | string | `string: max_len: 8` | UI mode the page rendered in: it | ot. |
| `propsJson` | `string` | string | `string: max_len: 2048` | Event properties as a JSON object; keys are checked against the registry and the whole thing is capped at 2 KB. |

### RecordRequest

A batch of browser events.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `events` | [`BrowserEvent`](#browserevent)[] | array of object | `repeated: min_items: 1 max_items: 50` | Up to 50 events. |

### RecordResponse

How the batch was handled.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `accepted` | `int32` | number |  | Events stored (duplicates and rejected events are not counted). |

### GetHomeRequest

Empty.

_No fields._

### WelcomeBack

Welcome-back panel data; absent on the first visit.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `lastVisitAt` | `Timestamp` | string (RFC 3339, UTC) |  | Start of the previous visit. |
| `previousVisits` | `int32` | number |  | Number of previous visits. |
| `recentlyViewed` | [`ContentSummary`](#contentsummary)[] | array of object |  | Up to three items viewed last time. |
| `recentTopics` | `string`[] | array of string |  | Topics asked about last time, as short phrases. |
| `newSinceLastVisit` | [`ContentSummary`](#contentsummary)[] | array of object |  | Items added or updated since the previous visit. |
| `resumeConversationId` | `string` | string |  | Conversation to offer resuming, when the last one was recent. |

### PathItem

One step of a "Start here" reading path.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `position` | `int32` | number |  | Position in the path (1-based). |
| `item` | [`ContentSummary`](#contentsummary) | object |  | The item. |
| `completed` | `bool` | boolean |  | True when the member has viewed it. |

### HighlightSection

A block of highlighted items with the reason it is shown.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `title` | `string` | string |  | Section title, e.g. "Roles that match process-control leadership". |
| `reason` | `string` | string |  | One-line reason, e.g. "Shown first because you selected Industrial automation". |
| `trackId` | `string` | string |  | Track that drove the section, when any. |
| `items` | [`ContentSummary`](#contentsummary)[] | array of object |  | Items in order. |

### RecommendedResume

The résumé variant recommended for the member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `variant` | `string` | string |  | Variant identifier (DownloadItem.variant). |
| `title` | `string` | string |  | Display title. |
| `version` | `string` | string |  | Version label. |
| `downloadPath` | `string` | string |  | Download path: `GET /api/downloads/{variant}`. |
| `reason` | `string` | string |  | Why this variant, e.g. "Matches your interest in AI and agentic systems". |

### Hero

Hero image selected for the member's primary track.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `variants` | [`ImageVariant`](#imagevariant)[] | array of object |  | Renditions from small to large. |
| `alt` | `string` | string |  | Alt text. |
| `caption` | `string` | string |  | Caption. |

### GetHomeResponse

Home page payload.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member. |
| `welcomeBack` | [`WelcomeBack`](#welcomeback) | object |  | Welcome-back panel; unset on the first visit. |
| `startHere` | [`PathItem`](#pathitem)[] | array of object |  | "Start here" path for the member's primary track (5–8 items); empty when the member has no interests. |
| `startHereTrackId` | `string` | string |  | Track the path belongs to. |
| `highlights` | [`HighlightSection`](#highlightsection)[] | array of object |  | Highlight sections in display order. |
| `resume` | [`RecommendedResume`](#recommendedresume) | object |  | Recommended résumé variant. |
| `hero` | [`Hero`](#hero) | object |  | Hero image. |
| `suggestedQuestions` | `string`[] | array of string |  | Suggested assistant questions for the member's tracks. |
| `questionnairePending` | `bool` | boolean |  | True when the first-visit questionnaire is still pending. |
| `tailored` | `bool` | boolean |  | True when the page was tailored (tailoring on and interests present). |

### GetMeetingOptionsRequest

Request for GetMeetingOptions. Empty: the options belong to the
owner, not to the caller, and a member cannot vary them.

_No fields._

### GetMeetingOptionsResponse

What a member may choose, and the clock it is all quoted in.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `durationMinutes` | `int32`[] | array of number |  | Meeting lengths on offer, in minutes, shortest first. The owner offers 15, 30 and 45; the length is the member's decision because only they know whether they need a question answered or a real conversation. |
| `zone` | `string` | string |  | IANA zone name every time in this API is rendered in, for example "America/New_York". Never an offset: an offset is correct for half the year. Show it beside the times rather than converting. |
| `zoneLabel` | `string` | string |  | Human label for the zone to print next to a time, for example "Eastern time". Sent by the server so the wording is the owner's and not the frontend's guess from the IANA name. |
| `horizonDays` | `int32` | number |  | How many days ahead the calendar is offered. |
| `leadHours` | `int32` | number |  | How far ahead the earliest bookable slot sits, in hours, so the page can say why today is not on offer. |
| `hoursSummary` | `string` | string |  | Plain-language summary of the owner's stated hours, for example "Tuesday to Thursday, mornings and early afternoons". Rendered rather than derived, so a member reads intent instead of a grid. |
| `available` | `bool` | boolean |  | False when the calendar is not connected or is unreachable, in which case no slots can be offered and the page should say so rather than show an empty calendar that reads as "never free". |
| `unavailableReason` | `string` | string |  | Why booking is unavailable, for the member, when available is false. Empty otherwise. |

### GetAvailabilityRequest

Request for GetAvailability.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `durationMinutes` | `int32` | number | `int32: gt: 0` | Meeting length in minutes. Must be one of the lengths returned by GetMeetingOptions; a length nobody offered is a bad request rather than something to round to the nearest. |
| `from` | `Timestamp` | string (RFC 3339, UTC) |  | Start of the range to search, inclusive. Defaults to now when unset. Clamped to the lead time regardless of what is sent. |
| `to` | `Timestamp` | string (RFC 3339, UTC) |  | End of the range, exclusive. Defaults to the horizon when unset, and is clamped to it, so a caller cannot ask about next year. |

### MeetingSlot

One offered start time.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `start` | `Timestamp` | string (RFC 3339, UTC) |  | When the meeting would begin. |
| `end` | `Timestamp` | string (RFC 3339, UTC) |  | When it would end. Sent rather than derived so the page cannot disagree with the server about the length it is showing. |

### GetAvailabilityResponse

Available start times for the requested length.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `slots` | [`MeetingSlot`](#meetingslot)[] | array of object |  | Offered start times, in order. Empty means nothing is free in the range, which is a normal answer and not an error. |
| `zone` | `string` | string |  | IANA zone the slots should be rendered in, repeated here so a response is self-describing without the options call. |
| `available` | `bool` | boolean |  | False when the calendar could not be read. Distinguishes "nothing is free" from "we do not know", which must not look the same. |
| `unavailableReason` | `string` | string |  | Why availability could not be computed, when available is false. |

### BookMeetingRequest

Request for BookMeeting.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `start` | `Timestamp` | string (RFC 3339, UTC) | `required: true` | The chosen start, exactly as returned by GetAvailability. The server re-checks it: this is a claim, not an instruction. |
| `durationMinutes` | `int32` | number | `int32: gt: 0` | Meeting length in minutes; must be one of the offered lengths. |
| `topic` | `string` | string | `string: max_len: 500` | What the member wants to discuss, in their words. Shown to the owner on the calendar entry so he arrives knowing the subject. |
| `contactPreference` | `string` | string | `string: max_len: 200` | How the member would like to meet, in their words, for example a phone number or "your call, send a link". Optional. |
| `meetingType` | [`MeetingType`](#meetingtype) | string (enum name) |  | Video or phone. Required on new bookings. |
| `videoProvider` | [`VideoProvider`](#videoprovider) | string (enum name) |  | Which service, when meeting_type is video. Rejected otherwise. |
| `phoneNumber` | `string` | string | `string: max_len: 32` | The number the member will call FROM, when meeting_type is phone. Country code, a space, then the ten-digit number, for example "+1 5135551234". May be left empty to say the number is in the comments instead, which is the escape hatch for anyone whose number does not fit that shape. |

### Meeting

A booked meeting, as the member sees it.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) |  | Stable id for this booking, used to cancel it. |
| `start` | `Timestamp` | string (RFC 3339, UTC) |  | When it begins. |
| `end` | `Timestamp` | string (RFC 3339, UTC) |  | When it ends. |
| `durationMinutes` | `int32` | number |  | Length in minutes, as booked. |
| `topic` | `string` | string |  | What the member said they wanted to discuss. |
| `zone` | `string` | string |  | IANA zone the times should be rendered in. |
| `icsUrl` | `string` | string |  | Relative URL of the calendar file for this meeting, so the member can add it to their own calendar or forward an invitation of their own. Always present, including for a meeting booked in the app. |
| `cancelledAt` | `Timestamp` | string (RFC 3339, UTC) |  | Set when the meeting has been cancelled. Cancelled meetings stay in the list rather than vanishing, because a meeting that silently disappears reads as a bug. |
| `meetingType` | [`MeetingType`](#meetingtype) | string (enum name) |  | Video or phone, as booked. |
| `videoProvider` | [`VideoProvider`](#videoprovider) | string (enum name) |  | Which service, when the type is video. |
| `phoneNumber` | `string` | string |  | The number the member said they would call from. Empty with a phone meeting means they said it is in the comments. |

### BookMeetingResponse

Result of booking.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meeting` | [`Meeting`](#meeting) | object |  | The meeting as booked. |

### ListMyMeetingsRequest

Request for ListMyMeetings.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `includePast` | `bool` | boolean |  | Include meetings that have already happened or been cancelled. False returns only what is still ahead. |

### ListMyMeetingsResponse

The caller's meetings.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meetings` | [`Meeting`](#meeting)[] | array of object |  | Meetings, soonest first. |

### CancelMeetingRequest

Request for CancelMeeting.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `int64` | string (decimal) | `int64: gt: 0` | Which meeting to cancel. Must belong to the calling member. |

### CancelMeetingResponse

Result of cancelling.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `meeting` | [`Meeting`](#meeting) | object |  | The meeting, with cancelled_at set. |

### GetMeRequest

Empty.

_No fields._

### GetMeResponse

The calling member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member. |

### UpdateMeRequest

Partial profile update.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `name` | `string` | string | `string: max_len: 120` | Display name. |
| `organization` | `string` | string | `string: max_len: 200` | Organization. |
| `statedRole` | `string` | string | `string: max_len: 120` | Stated role or title. |
| `profile` | [`Profile`](#profile) | object |  | Questionnaire answers and tailoring setting. |
| `updateMask` | `FieldMask` | string (comma-separated paths) | `required: true` | Which fields to update, using paths such as `name`, `organization`, `stated_role`, `profile.seniority`, `profile.hiring_for`, `profile.priorities`, `profile.heard_from`, `profile.tailoring_enabled`. |

### UpdateMeResponse

Updated member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member after the update. |

### SetInterestsRequest

Full replacement of track interests.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `interests` | [`TrackInterest`](#trackinterest)[] | array of object | `repeated: max_items: 8` | Interests, at most eight; the questionnaire offers up to three. |
| `tailoringEnabled` | `bool` | boolean |  | Whether tailoring is enabled. |
| `questionnaireCompleted` | `bool` | boolean |  | Marks the first-visit questionnaire as completed (or skipped). |

### SetInterestsResponse

Updated member.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `me` | [`Me`](#me) | object |  | The member after the update. |

### GetHistoryRequest

History page request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | [`ActivityEvent.Kind`](#activityeventkind) | string (enum name) | `enum: defined_only: true` | Restrict to one kind; unspecified returns all kinds. |
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### GetHistoryResponse

History page.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `events` | [`ActivityEvent`](#activityevent)[] | array of object |  | Events, newest first, with `content` populated where applicable. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

### ListSavedRequest

Saved-items page request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `page` | [`PageRequest`](#pagerequest) | object |  | Pagination. |

### SavedItem

A saved item.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `content` | [`ContentSummary`](#contentsummary) | object |  | The content item. |
| `note` | `string` | string |  | Private note, possibly empty. |
| `savedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When it was saved. |

### ListSavedResponse

Saved-items page.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `items` | [`SavedItem`](#saveditem)[] | array of object |  | Items, newest first. |
| `page` | [`PageResponse`](#pageresponse) | object |  | Pagination. |

### SaveItemRequest

Save request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `contentId` | `string` | string | `string: min_len: 1 max_len: 128` | Content ID to save. |
| `note` | `string` | string | `string: max_len: 2000` | Optional private note (≤ 2,000 characters). |

### SaveItemResponse

The saved item.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `item` | [`SavedItem`](#saveditem) | object |  | The saved item as stored. |

### UnsaveItemRequest

Unsave request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `contentId` | `string` | string | `string: min_len: 1 max_len: 128` | Content ID to remove from the saved list. |

### UnsaveItemResponse

Empty.

_No fields._

### RequestExportRequest

Empty.

_No fields._

### RequestExportResponse

Export handle.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `exportId` | `string` | string |  | Export identifier for GetExport. |
| `estimatedReadyAt` | `Timestamp` | string (RFC 3339, UTC) |  | Expected readiness; exports usually complete within a minute. |

### GetExportRequest

Export status request.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `exportId` | `string` | string | `string: min_len: 1 max_len: 64` | Export identifier from RequestExport. |

### GetExportResponse

Export status.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `status` | [`GetExportResponse.Status`](#getexportresponsestatus) | string (enum name) |  | Current state. |
| `downloadUrl` | `string` | string |  | Signed download URL, valid for `download_url_expires_at`; empty until READY. |
| `downloadUrlExpiresAt` | `Timestamp` | string (RFC 3339, UTC) |  | Expiry of the download URL. |
| `sizeBytes` | `int64` | string (decimal) |  | Size of the export in bytes when READY. |

### DeleteAccountRequest

Account deletion; requires an explicit confirmation phrase and, for
password accounts, the current password.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `confirmation` | `string` | string | `string: const: "DELETE"` | Must be exactly "DELETE". |
| `currentPassword` | `string` | string | `string: max_len: 256` | Current password; required unless the account is social-only. |

### DeleteAccountResponse

Deletion schedule.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `purgeBy` | `Timestamp` | string (RFC 3339, UTC) |  | Latest time by which personal data is purged. |

### GetVersionRequest

Empty.

_No fields._

### GetVersionResponse

Build metadata.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `version` | `string` | string |  | Semantic version, e.g. `0.4.0`. |
| `commit` | `string` | string |  | Git commit SHA. |
| `builtAt` | `Timestamp` | string (RFC 3339, UTC) |  | Build time. |
| `goVersion` | `string` | string |  | Go toolchain version. |
| `personaVersion` | `string` | string |  | Active persona version. |

### GetGovernanceStatusRequest

Empty.

_No fields._

### FrameworkStatus

One UxTS framework's status.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `acronym` | `string` | string |  | Acronym, e.g. `UATS`. |
| `name` | `string` | string |  | Full name. |
| `status` | `string` | string |  | Maturity: `spec-only`, `pilot`, `active`, `deprecated`. |
| `gateMode` | `string` | string |  | CI gate mode: `observe`, `soft`, `block`. |
| `specCount` | `int32` | number |  | Canonical specs on disk. |
| `passed` | `int32` | number |  | Last run: specs passed. |
| `failed` | `int32` | number |  | Last run: specs failed. |
| `hashesVerified` | `int32` | number |  | Last run: hashes verified. |
| `hashesMismatched` | `int32` | number |  | Last run: hash mismatches. |
| `lastRunAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the last run finished. |

### GetGovernanceStatusResponse

Governance summary.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `frameworks` | [`FrameworkStatus`](#frameworkstatus)[] | array of object |  | Frameworks in matrix order. |
| `commit` | `string` | string |  | Commit the reports were produced from. |
| `publishedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When the summary was published. |

### GetReviewerStatusRequest

Empty.

_No fields._

### GetReviewerStatusResponse

How the reviewer is measuring, in the two ways that can be stated
without describing anyone's private material.

Everything here is already visible to the owner on /admin/gate. The
point of publishing it is that a claim about a reviewer being honest
is worth less than the numbers it is failing on.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `graded` | `int32` | number |  | Verdicts the owner has graded and that could be graded (he can also answer "not enough evidence to judge", which is excluded here). |
| `agreed` | `int32` | number |  | Of those, how many he agreed with. |
| `agreementPct` | `double` | number |  | Agreement as a percentage, to one decimal place. |
| `hardDisagreements` | `int32` | number |  | Disagreements where the model said met and he said unmet, or the reverse. Counted apart from the softer kind because they mean the model was wrong rather than unsure. |
| `tooHarsh` | `int32` | number |  | Of the hard disagreements, how many were the model refusing to credit something he can evidence. The opposite direction, crediting what he cannot evidence, is the one that would matter to an employer. |
| `tooGenerous` | `int32` | number |  | Of the hard disagreements, how many were the model crediting something he says is not evidenced. |
| `postings` | `int32` | number |  | Postings in the fixed evaluation set. |
| `postingsRandom` | `int32` | number |  | How many of those were drawn at random from job boards rather than chosen by the owner. |
| `scored` | `int32` | number |  | Postings scored in the last completed evaluation. |
| `gateCorrect` | `int32` | number |  | How many landed on the side the owner said they should. |
| `inversions` | `int32` | number |  | Pairs where a posting he said he could not do outscored one he said he could. Moving the threshold cannot fix one of these. |
| `margin` | `double` | number |  | _(oneof `_margin`)_ The gap between the two groups. Absent when one side is empty. |
| `evaluatedAt` | `Timestamp` | string (RFC 3339, UTC) |  | When that evaluation ran. |
| `model` | `string` | string |  | The model that produced it. |
| `bands` | [`JdFitBandsPublic`](#jdfitbandspublic) | object |  | The five fit bands in force, so a page can quote the real thresholds without a session. Without these an anonymous reader is shown numbers derived from an environment default, which are right only until the owner edits the bands. |
| `comparison` | [`ReviewerComparisonRow`](#reviewercomparisonrow)[] | array of object |  | Every posting in the last completed evaluation beside its score in the evaluation before it. Empty until two have completed. |

### ReviewerComparisonRow

One posting scored twice, which is the evidence that the same input
gives the same answer.

Postings the owner applied for are labelled generically. Naming them
would tell those employers that he applied and what the reviewer
scored him at, which is his information to disclose and not a detail
the evidence needs. Randomly drawn postings carry their role, since
they are public advertisements and say nothing about him.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `label` | `string` | string |  | A description safe to publish, not the posting's stored name. |
| `score` | `double` | number |  | Score in the most recent completed evaluation. |
| `previous` | `double` | number |  | _(oneof `_previous`)_ Score in the evaluation before it. Unset when the posting was not in that one, which is what a newly added posting looks like. |
| `unchanged` | `bool` | boolean |  | Whether the two agree to three decimal places. |

### JdFitBandsPublic

The five fit bands: very strong, strong, possible, weak, very weak.

Four numbers describe them, because each threshold opens a band and
the fifth needs no number: very weak is whatever falls below the weak
line. Storing a zero for it would be a field that can never hold
anything else.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `veryStrong` | `double` | number |  | At or above this is very strong. |
| `strong` | `double` | number |  | At or above this, and below very strong, is strong. This is also the résumé gate. |
| `possible` | `double` | number |  | At or above this, and below strong, is possible. |
| `weak` | `double` | number |  | At or above this, and below possible, is weak. Anything below this is very weak, the fifth band. |

### EmbedPurpose

Why a text is being embedded; some models use different instructions for
queries and documents.

| Value | Number | Description |
|---|---|---|
| `EMBED_PURPOSE_UNSPECIFIED` | 0 | Not set. |
| `EMBED_PURPOSE_QUERY` | 1 | A member question at retrieval time. |
| `EMBED_PURPOSE_DOCUMENT` | 2 | A corpus chunk at ingest time. |

### Scope

Scope verdict.

| Value | Number | Description |
|---|---|---|
| `SCOPE_UNSPECIFIED` | 0 | Not set. |
| `SCOPE_IN_SCOPE` | 1 | About the owner's professional background, work, views, or this site. |
| `SCOPE_OUT_OF_SCOPE` | 2 | Outside the assistant's remit. |
| `SCOPE_INJECTION_SUSPECTED` | 3 | Looks like an attempt to override instructions or extract the prompt. |
| `SCOPE_UNCERTAIN` | 4 | The classifier could not decide. |

### JobKind

Batch jobs.

| Value | Number | Description |
|---|---|---|
| `JOB_KIND_UNSPECIFIED` | 0 | Not set. |
| `JOB_KIND_CONTENT_VALIDATE` | 1 | `career-cli content validate`. |
| `JOB_KIND_CONTENT_INDEX` | 2 | `career-cli content index`. |
| `JOB_KIND_INGEST_CHANGED` | 3 | `career-cli ingest --changed`. |
| `JOB_KIND_INGEST_FULL` | 4 | `career-cli ingest --full`. |
| `JOB_KIND_ASSETS_PUSH` | 5 | `career-cli assets push`. |
| `JOB_KIND_RESUME_BUILD` | 6 | `career-cli resume build`. |
| `JOB_KIND_EVAL_QUICK` | 7 | `career-cli eval run --profile quick`. |
| `JOB_KIND_EVAL_FULL` | 8 | `career-cli eval run --profile full`. |

### JobStatus

Job lifecycle.

| Value | Number | Description |
|---|---|---|
| `JOB_STATUS_UNSPECIFIED` | 0 | Not set. |
| `JOB_STATUS_QUEUED` | 1 | Queued. |
| `JOB_STATUS_RUNNING` | 2 | Running. |
| `JOB_STATUS_SUCCEEDED` | 3 | Finished successfully. |
| `JOB_STATUS_FAILED` | 4 | Failed. |

### ActivityEvent.Kind

Kinds of activity.

| Value | Number | Description |
|---|---|---|
| `KIND_UNSPECIFIED` | 0 | Not set. |
| `KIND_VIEW` | 1 | Viewed a content page. |
| `KIND_DOWNLOAD` | 2 | Downloaded a file. |
| `KIND_SEARCH` | 3 | Ran a search. |
| `KIND_SAVE` | 4 | Saved an item. |
| `KIND_CHAT` | 5 | Sent a message to the assistant (recorded server-side; clients need not send it). |
| `KIND_ESCALATE` | 6 | Escalated a question to the owner (recorded server-side). |
| `KIND_LOGIN` | 7 | Signed in via the /login form (recorded server-side). |
| `KIND_LOGOUT` | 8 | Signed out via /logout (recorded server-side). |

### ContentType

The kind of a content item. Mirrors the `type` field of files under
`content/` in the repository.

| Value | Number | Description |
|---|---|---|
| `CONTENT_TYPE_UNSPECIFIED` | 0 | Not set; used only as a "no filter" value in list requests. |
| `CONTENT_TYPE_ROLE` | 1 | A role on the career timeline. |
| `CONTENT_TYPE_PROJECT` | 2 | A project or repository. |
| `CONTENT_TYPE_ARTICLE` | 3 | An article or publication. |
| `CONTENT_TYPE_PRESENTATION` | 4 | A deck or paper with a PDF. |
| `CONTENT_TYPE_SKILL` | 5 | A skill entry from the skills matrix. |
| `CONTENT_TYPE_CREDENTIAL` | 6 | An award, degree, service record, or eligibility statement. |
| `CONTENT_TYPE_PHOTO` | 7 | A gallery photo. |
| `CONTENT_TYPE_RESUME` | 8 | A résumé variant available for download. |
| `CONTENT_TYPE_QA` | 9 | An owner-written question/answer pair used by the assistant. |

### InterestSource

Origin of a track interest.

| Value | Number | Description |
|---|---|---|
| `INTEREST_SOURCE_UNSPECIFIED` | 0 | Not set. |
| `INTEREST_SOURCE_QUESTIONNAIRE` | 1 | Chosen in the first-visit questionnaire. |
| `INTEREST_SOURCE_EDITED` | 2 | Edited later under /me/interests. |
| `INTEREST_SOURCE_INVITE` | 3 | Pre-selected by an invite link (post-launch feature). |

### MemberStatus

Account status.

| Value | Number | Description |
|---|---|---|
| `MEMBER_STATUS_UNSPECIFIED` | 0 | Not set. |
| `MEMBER_STATUS_UNVERIFIED` | 1 | Registered but the email address is not yet verified. |
| `MEMBER_STATUS_ACTIVE` | 2 | Verified; full member access. |
| `MEMBER_STATUS_PENDING_APPROVAL` | 3 | Verified but waiting for owner approval (approval mode only). |
| `MEMBER_STATUS_DISABLED` | 4 | Disabled by the owner. |
| `MEMBER_STATUS_DECLINED` | 5 | Owner declined the access request (D-02, ADR-0002). |
| `MEMBER_STATUS_EXPIRED` | 6 | Access period has ended (users.expires_at in the past; FR-AUTH-18, ADR-0020). |

### MemberRole

Account role.

| Value | Number | Description |
|---|---|---|
| `MEMBER_ROLE_UNSPECIFIED` | 0 | Not set. |
| `MEMBER_ROLE_MEMBER` | 1 | Ordinary member. |
| `MEMBER_ROLE_ADMIN` | 2 | The owner's administrative account. |

### Priority

What the member said matters most to them.

| Value | Number | Description |
|---|---|---|
| `PRIORITY_UNSPECIFIED` | 0 | Not set. |
| `PRIORITY_LEADERSHIP` | 1 | Leadership and strategy evidence. |
| `PRIORITY_HANDS_ON_DEPTH` | 2 | Hands-on technical depth. |
| `PRIORITY_RESEARCH` | 3 | Research and original thinking. |
| `PRIORITY_WRITING` | 4 | Writing and communication. |

### AuthLevel

Who may call a method.

Enforced by hand inside each handler, not before it. This comment used
to say "Enforced before the handler runs", which described the
interceptor in docs/sprint-auth-interceptor.md rather than the code.
The level here is the contract those handlers are checked against:
internal/handlers/authpolicy_test.go compares every declaration with
what its handler actually calls and fails on any disagreement.

A caller below the required level receives the Connect code
`unauthenticated` (no session) or `permission_denied` (session without
the role). That part is accurate, because it is what the handlers
return, and it is what the interceptor will return when S4 moves the
decision out of them.

| Value | Number | Description |
|---|---|---|
| `AUTH_LEVEL_UNSPECIFIED` | 0 | Not set, and the server will not start that way: internal/authpolicy builds the policy from these declarations at boot and exits naming any method that left the level unspecified. This comment previously claimed "Lint fails any method that leaves the level unspecified"; no such lint rule existed, which is why it is a boot failure now. |
| `AUTH_LEVEL_PUBLIC` | 1 | No session required. |
| `AUTH_LEVEL_MEMBER` | 2 | A verified member session is required (status ACTIVE). Methods that an unverified member may call additionally set `allow_unverified`. |
| `AUTH_LEVEL_ADMIN` | 3 | An admin session with a recent TOTP verification is required. |

### Message.Role

Message author.

| Value | Number | Description |
|---|---|---|
| `ROLE_UNSPECIFIED` | 0 | Not set. |
| `ROLE_USER` | 1 | The member. |
| `ROLE_ASSISTANT` | 2 | The assistant. |
| `ROLE_OWNER` | 3 | The owner, replying to an escalation. |

### Escalation.Status

Escalation lifecycle.

| Value | Number | Description |
|---|---|---|
| `STATUS_UNSPECIFIED` | 0 | Not set. |
| `STATUS_OPEN` | 1 | Waiting for the owner. |
| `STATUS_ANSWERED` | 2 | The owner replied in the conversation. |
| `STATUS_CLOSED` | 3 | Closed without a reply. |

### GetQuotaResponse.BudgetMode

Global assistant budget state.

| Value | Number | Description |
|---|---|---|
| `BUDGET_MODE_UNSPECIFIED` | 0 | Not set. |
| `BUDGET_MODE_NORMAL` | 1 | Full generation available. |
| `BUDGET_MODE_QA_ONLY` | 2 | Monthly budget reached: answers come from the Q&A bank only. |
| `BUDGET_MODE_DEGRADED` | 3 | Provider unavailable: Q&A bank only until it recovers. |

### Rating

Thumbs up or down.

| Value | Number | Description |
|---|---|---|
| `RATING_UNSPECIFIED` | 0 | Not rated. |
| `RATING_UP` | 1 | Helpful. |
| `RATING_DOWN` | 2 | Not helpful. |

### SubmitContactRequest.ReplyChannel

How the member wants a reply.

| Value | Number | Description |
|---|---|---|
| `REPLY_CHANNEL_UNSPECIFIED` | 0 | Not set. |
| `REPLY_CHANNEL_EMAIL` | 1 | Email to the member's verified address, or (for anonymous submitters) to the email supplied on the form. |
| `REPLY_CHANNEL_LINKEDIN` | 2 | LinkedIn message (requires a linked LinkedIn profile). |

### SupportCategory

Category buckets for the support inbox (FR-CNT-22).

| Value | Number | Description |
|---|---|---|
| `SUPPORT_CATEGORY_UNSPECIFIED` | 0 | Not set. |
| `SUPPORT_CATEGORY_GENERAL_QUESTION` | 1 | Open-ended question about the site or the owner. |
| `SUPPORT_CATEGORY_BUG_REPORT` | 2 | Bug report — something on the site isn't working. |
| `SUPPORT_CATEGORY_FEATURE_REQUEST` | 3 | Feature request or suggestion. |
| `SUPPORT_CATEGORY_CONTRIBUTOR_ACCESS` | 4 | Request to be added as a collaborator on one of the owner's public GitHub repositories. Body should name the repo. |
| `SUPPORT_CATEGORY_PRESS_INQUIRY` | 5 | Press, interview, or podcast inquiry. |
| `SUPPORT_CATEGORY_OTHER` | 6 | Anything not covered above. |
| `SUPPORT_CATEGORY_HIRING_INQUIRY` | 7 | Hiring manager reaching out about a specific role. The web form reveals dedicated fields (role, JD URL, target start) when this is selected; those land on SubmitContactRequest.hiring_*. |

### JdSource

Where the JD text came from — mirrors jd_submissions.source_kind.
New sources are added here as the frontend gains upload paths;
nothing in the schema breaks when this list grows.

| Value | Number | Description |
|---|---|---|
| `JD_SOURCE_UNSPECIFIED` | 0 | Not set (proto3 requires a zero value). Rejected in requests. |
| `JD_SOURCE_PASTE` | 1 | Pasted directly into the textarea on /jd-upload. |
| `JD_SOURCE_PDF` | 2 | Uploaded PDF; text extracted server-side. Reserved: the form only sends PASTE today. |
| `JD_SOURCE_TEXT_UPLOAD` | 3 | Uploaded plain-text file (.txt, .md). Reserved: the form only sends PASTE today. |

### JdStatus

Lifecycle of a submission — mirrors jd_submissions.status.

| Value | Number | Description |
|---|---|---|
| `JD_STATUS_UNSPECIFIED` | 0 | Not set. Servers never return this; callers never send it. |
| `JD_STATUS_RECEIVED` | 1 | Stored, waiting to be scored. |
| `JD_STATUS_SCORING` | 2 | Retrieval + scoring is running. |
| `JD_STATUS_BELOW_THRESHOLD` | 3 | Score below the résumé gate (the "strong" fit band in force, see JdFitBands); the caller still gets the verdict breakdown. |
| `JD_STATUS_GENERATING` | 4 | Score at or above the gate; résumé generation is running. |
| `JD_STATUS_READY` | 5 | Résumé is ready; generated_resume_url is populated. |
| `JD_STATUS_FAILED` | 6 | Anything above raised an error; see error_message for detail. |
| `JD_STATUS_NOT_A_POSTING` | 7 | The submitted text is not a job posting (a list of search terms, a résumé, a fragment), so nothing was scored. error_message says what it looked like and what to do. This is a refusal, not a failure: a score computed from the wrong kind of input looks exactly like a real one and means nothing. |

### ListMembersRequest.Sort

Sort orders.

| Value | Number | Description |
|---|---|---|
| `SORT_UNSPECIFIED` | 0 | Most recently seen first. |
| `SORT_NEWEST` | 1 | Most recently registered first. |
| `SORT_MOST_ACTIVE` | 2 | Most active first. |
| `SORT_NAME` | 3 | Alphabetical by name. |

### ReviewKind

Kinds of review items.

| Value | Number | Description |
|---|---|---|
| `REVIEW_KIND_UNSPECIFIED` | 0 | Not set (no filter). |
| `REVIEW_KIND_NEGATIVE_FEEDBACK` | 1 | A member rated an answer down. |
| `REVIEW_KIND_NO_SUPPORT` | 2 | The assistant found no supporting material. |
| `REVIEW_KIND_ESCALATION` | 3 | A member escalated to the owner. |
| `REVIEW_KIND_OUT_OF_SCOPE` | 4 | The assistant declined an out-of-scope question. |

### ReviewStatus

Review item lifecycle.

| Value | Number | Description |
|---|---|---|
| `REVIEW_STATUS_UNSPECIFIED` | 0 | Not set (no filter). |
| `REVIEW_STATUS_OPEN` | 1 | Awaiting the owner. |
| `REVIEW_STATUS_RESOLVED` | 2 | Handled. |
| `REVIEW_STATUS_CONVERTED_TO_QA` | 3 | Handled by adding a Q&A-bank entry. |
| `REVIEW_STATUS_REPLIED` | 4 | Handled by replying to the member. |

### JobKind

Jobs the owner can start. Kinds 1 to 7 are reserved for sidecar jobs
and are not runnable yet; kinds 8 to 10 run inside the api.

| Value | Number | Description |
|---|---|---|
| `JOB_KIND_UNSPECIFIED` | 0 | Not set. |
| `JOB_KIND_CONTENT_VALIDATE` | 1 | Validate content files against the schema. |
| `JOB_KIND_CONTENT_INDEX` | 2 | Rebuild the content catalog in the database. |
| `JOB_KIND_INGEST_CHANGED` | 3 | Ingest changed content into the corpus. |
| `JOB_KIND_INGEST_FULL` | 4 | Re-ingest the entire corpus. |
| `JOB_KIND_RESUME_BUILD` | 5 | Rebuild résumé PDFs from structured content. |
| `JOB_KIND_EVAL_QUICK` | 6 | Run the UVTS retrieval evaluation (quick profile). |
| `JOB_KIND_GITHUB_SYNC` | 7 | Refresh GitHub repository metadata. |
| `JOB_KIND_CORPUS_REINDEX_PUBLIC` | 8 | Walk the public content mount into the corpus (api-side job). |
| `JOB_KIND_CORPUS_REINDEX_PRIVATE` | 9 | Walk the private corpus mount into the corpus (api-side job). |
| `JOB_KIND_EMBED_SWEEP` | 10 | Re-embed every stale chunk until none remain (api-side job). |

### JobStatus

Job lifecycle.

| Value | Number | Description |
|---|---|---|
| `JOB_STATUS_UNSPECIFIED` | 0 | Not set. |
| `JOB_STATUS_QUEUED` | 1 | Queued. |
| `JOB_STATUS_RUNNING` | 2 | Running. |
| `JOB_STATUS_SUCCEEDED` | 3 | Finished successfully. |
| `JOB_STATUS_FAILED` | 4 | Failed. |

### SupportStatus

Support message resolution status.

| Value | Number | Description |
|---|---|---|
| `SUPPORT_STATUS_UNSPECIFIED` | 0 | Default (proto3 requires a zero value). Treated as "any" in list requests and as "resolved" in ResolveContactMessage. |
| `SUPPORT_STATUS_OPEN` | 1 | Message is waiting for a reply / triage. |
| `SUPPORT_STATUS_RESOLVED` | 2 | Message has been handled. |

### GrantTTL

The whitelisted set of TTL choices from FSD FR-AUTH-17. Mirrors
the `access_ttl` enum type in the DB (`1d` / `3d` / `7d` / `30d`
/ `permanent`).

| Value | Number | Description |
|---|---|---|
| `GRANT_TTL_UNSPECIFIED` | 0 | Not set (proto3 requires a zero value). Rejected in the upsert request; validation forces the caller to pick one. |
| `GRANT_TTL_1D` | 1 | 24 hours. |
| `GRANT_TTL_3D` | 2 | 72 hours. |
| `GRANT_TTL_7D` | 3 | 7 days. |
| `GRANT_TTL_30D` | 4 | 30 days. |
| `GRANT_TTL_PERMANENT` | 5 | No expiry — access does not auto-lapse. |

### ActivitySort

Sort order for ListMemberActivity.

| Value | Number | Description |
|---|---|---|
| `ACTIVITY_SORT_UNSPECIFIED` | 0 | Default: name (case-insensitive ascending). |
| `ACTIVITY_SORT_LAST_EVENT_DESC` | 1 | Newest activity first. |
| `ACTIVITY_SORT_SESSIONS_DESC` | 2 | Highest total session count first. |
| `ACTIVITY_SORT_ACTIVE_TIME_DESC` | 3 | Highest total active-time first. |
| `ACTIVITY_SORT_ASK_ROGER_DESC` | 4 | Most Ask Roger interactions first. |

### ListContentRequest.Order

Orderings.

| Value | Number | Description |
|---|---|---|
| `ORDER_UNSPECIFIED` | 0 | Tailored when the member has tailoring on, otherwise newest first. |
| `ORDER_RELEVANCE` | 1 | Relevance to the member's tracks (falls back to newest first when the member has no interests). |
| `ORDER_NEWEST` | 2 | Newest first by `published`. |
| `ORDER_CHRONOLOGICAL` | 3 | Chronological by role start date or `published` (timeline order). |

### ProjectDetail.Status

Project lifecycle.

| Value | Number | Description |
|---|---|---|
| `STATUS_UNSPECIFIED` | 0 | Not set. |
| `STATUS_ACTIVE` | 1 | Under active development. |
| `STATUS_MAINTAINED` | 2 | Stable; maintained. |
| `STATUS_ARCHIVED` | 3 | No longer developed. |
| `STATUS_EXPLORATION` | 4 | Early exploration. |

### CredentialDetail.Kind

Credential kinds.

| Value | Number | Description |
|---|---|---|
| `KIND_UNSPECIFIED` | 0 | Not set. |
| `KIND_AWARD` | 1 | Award or recognition. |
| `KIND_EDUCATION` | 2 | Degree or certificate. |
| `KIND_SERVICE` | 3 | Military or public service. |
| `KIND_ELIGIBILITY` | 4 | Eligibility statement (citizenship, clearance eligibility). |

### MeetingType

How the meeting happens. The owner's decision is that the member
brings their own video room rather than this application creating
one: no Meet, Teams or Zoom credentials are held here.

| Value | Number | Description |
|---|---|---|
| `MEETING_TYPE_UNSPECIFIED` | 0 | Not set. Bookings made before this was asked carry this. |
| `MEETING_TYPE_VIDEO` | 1 | A video call the member sets up in their own calendar. |
| `MEETING_TYPE_PHONE` | 2 | A phone call. The member says which number they will call from. |

### VideoProvider

Which video service the member will host in. Named only so the
invitation says the right thing and the member is sent to the right
place; no room is created here.

| Value | Number | Description |
|---|---|---|
| `VIDEO_PROVIDER_UNSPECIFIED` | 0 | Not set, and required to be unset unless the type is video. |
| `VIDEO_PROVIDER_GOOGLE_MEET` | 1 | Google Meet. |
| `VIDEO_PROVIDER_TEAMS` | 2 | Microsoft Teams. |
| `VIDEO_PROVIDER_ZOOM` | 3 | Zoom. |

### GetExportResponse.Status

Export lifecycle.

| Value | Number | Description |
|---|---|---|
| `STATUS_UNSPECIFIED` | 0 | Not set. |
| `STATUS_PENDING` | 1 | Queued or running. |
| `STATUS_READY` | 2 | Ready for download. |
| `STATUS_FAILED` | 3 | Failed; the member may request again. |
| `STATUS_EXPIRED` | 4 | Download window elapsed; request again. |
