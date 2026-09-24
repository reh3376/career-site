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
| [`ActivityService`](#activityservice) | Batched, fire-and-forget activity reporting. | 1 |
| [`ChatService`](#chatservice) | Conversations with the assistant. | 9 |
| [`DownloadService`](#downloadservice) | Lists what can be downloaded. | 1 |
| [`ContactService`](#contactservice) | Reaching the owner outside the assistant. | 2 |
| [`AdminService`](#adminservice) | Owner console. | 54 |
| [`SystemService`](#systemservice) | Version and governance status. | 2 |
| [`JdService`](#jdservice) | JD-upload flow, members only: a signed-in session is required to submit or poll, and the submitting member is recorded on the row. | 4 |
| [`EventService`](#eventservice) | Accepts browser-minted events. | 1 |
| [`SidecarService`](#sidecarservice) | Embedding, reranking, classification, and batch jobs. _(internal)_ | 8 |

## AuthService

Registration, verification, sign-in, and credential management.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`Register`](#authservice-register) | `/api/career.v1.AuthService/Register` | Public | 5 | `RegisterRequest` → `RegisterResponse` | Creates an unverified account and sends a verification email. |
| [`Verify`](#authservice-verify) | `/api/career.v1.AuthService/Verify` | Public | 10 | `VerifyRequest` → `VerifyResponse` | Confirms control of the email address with the emailed link token or the six-digit code, activates the account, and starts a session (cookie set in the response). |
| [`ResendVerification`](#authservice-resendverification) | `/api/career.v1.AuthService/ResendVerification` | Public | 3 | `ResendVerificationRequest` → `ResendVerificationResponse` | Re-sends the verification email for an unverified account. |
| [`Login`](#authservice-login) | `/api/career.v1.AuthService/Login` | Public | 10 | `LoginRequest` → `LoginResponse` | Signs in with email and password and sets the session cookie. |
| [`Logout`](#authservice-logout) | `/api/career.v1.AuthService/Logout` | Member (unverified OK) | default | `LogoutRequest` → `LogoutResponse` | Ends the current session and clears the cookie. |
| [`LogoutAll`](#authservice-logoutall) | `/api/career.v1.AuthService/LogoutAll` | Member | default | `LogoutAllRequest` → `LogoutAllResponse` | Ends every session of the member ("sign out of all devices"). |
| [`ForgotPassword`](#authservice-forgotpassword) | `/api/career.v1.AuthService/ForgotPassword` | Public | 3 | `ForgotPasswordRequest` → `ForgotPasswordResponse` | Emails a single-use password-reset link (valid one hour). |
| [`ResetPassword`](#authservice-resetpassword) | `/api/career.v1.AuthService/ResetPassword` | Public | 5 | `ResetPasswordRequest` → `ResetPasswordResponse` | Sets a new password using a reset token and signs the member in on success. |
| [`ChangePassword`](#authservice-changepassword) | `/api/career.v1.AuthService/ChangePassword` | Member | 5 | `ChangePasswordRequest` → `ChangePasswordResponse` | Changes the password of the signed-in member; requires the current password. |
| [`ChangeEmail`](#authservice-changeemail) | `/api/career.v1.AuthService/ChangeEmail` | Member | 3 | `ChangeEmailRequest` → `ChangeEmailResponse` | Starts an email change: a verification email goes to the new address and the change applies when it is confirmed via Verify. |
| [`MfaEnroll`](#authservice-mfaenroll) | `/api/career.v1.AuthService/MfaEnroll` | Member | default | `MfaEnrollRequest` → `MfaEnrollResponse` | Enrols TOTP MFA for an admin account that has none. |
| [`MfaVerify`](#authservice-mfaverify) | `/api/career.v1.AuthService/MfaVerify` | Member | 5 | `MfaVerifyRequest` → `MfaVerifyResponse` | Verifies a TOTP code (or a recovery code) for the current admin session. |

### AuthService.Register

`POST /api/career.v1.AuthService/Register` · **Auth:** Public · **Rate limit:** 5/min

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

`POST /api/career.v1.AuthService/Verify` · **Auth:** Public · **Rate limit:** 10/min

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

`POST /api/career.v1.AuthService/ResendVerification` · **Auth:** Public · **Rate limit:** 3/min

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

`POST /api/career.v1.AuthService/Login` · **Auth:** Public · **Rate limit:** 10/min

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

`POST /api/career.v1.AuthService/Logout` · **Auth:** Member (unverified OK) · **Rate limit:** default/min

Ends the current session and clears the cookie.

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

`POST /api/career.v1.AuthService/ForgotPassword` · **Auth:** Public · **Rate limit:** 3/min

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

`POST /api/career.v1.AuthService/ResetPassword` · **Auth:** Public · **Rate limit:** 5/min

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

`POST /api/career.v1.AuthService/ChangePassword` · **Auth:** Member · **Rate limit:** 5/min

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

`POST /api/career.v1.AuthService/ChangeEmail` · **Auth:** Member · **Rate limit:** 3/min

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

`POST /api/career.v1.AuthService/MfaVerify` · **Auth:** Member · **Rate limit:** 5/min

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
| [`RequestExport`](#memberservice-requestexport) | `/api/career.v1.MemberService/RequestExport` | Member | 2 | `RequestExportRequest` → `RequestExportResponse` | Requests an export of all data held about the member (profile, interests, activity, saved items, conversations) as JSON. |
| [`GetExport`](#memberservice-getexport) | `/api/career.v1.MemberService/GetExport` | Member | default | `GetExportRequest` → `GetExportResponse` | Returns the status of an export and, when ready, a short-lived download URL. |
| [`DeleteAccount`](#memberservice-deleteaccount) | `/api/career.v1.MemberService/DeleteAccount` | Member (unverified OK) | 2 | `DeleteAccountRequest` → `DeleteAccountResponse` | Schedules deletion of the account and all personal data. |

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

`POST /api/career.v1.MemberService/RequestExport` · **Auth:** Member · **Rate limit:** 2/min

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

`POST /api/career.v1.MemberService/DeleteAccount` · **Auth:** Member (unverified OK) · **Rate limit:** 2/min

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
| [`Search`](#contentservice-search) | `/api/career.v1.ContentService/Search` | Member | 60 | `SearchRequest` → `SearchResponse` | Full-text search across titles, summaries, bodies, and tags, with a relevance boost for the member's tracks. |

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

`POST /api/career.v1.ContentService/Search` · **Auth:** Member · **Rate limit:** 60/min

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

## ActivityService

Batched, fire-and-forget activity reporting.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`RecordEvents`](#activityservice-recordevents) | `/api/career.v1.ActivityService/RecordEvents` | Member | 120 | `RecordEventsRequest` → `RecordEventsResponse` | Records a batch of events. |

### ActivityService.RecordEvents

`POST /api/career.v1.ActivityService/RecordEvents` · **Auth:** Member · **Rate limit:** 120/min

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

## ChatService

Conversations with the assistant.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`CreateConversation`](#chatservice-createconversation) | `/api/career.v1.ChatService/CreateConversation` | Member | 20 | `CreateConversationRequest` → `CreateConversationResponse` | Starts a conversation. |
| [`ListConversations`](#chatservice-listconversations) | `/api/career.v1.ChatService/ListConversations` | Member | default | `ListConversationsRequest` → `ListConversationsResponse` | Lists the member's conversations, most recent first. |
| [`GetConversation`](#chatservice-getconversation) | `/api/career.v1.ChatService/GetConversation` | Member | default | `GetConversationRequest` → `GetConversationResponse` | Returns a conversation with all of its messages. |
| [`SendMessage`](#chatservice-sendmessage) | `/api/career.v1.ChatService/SendMessage` | Member | 20 | `SendMessageRequest` → `SendMessageResponse` (server-streaming) | Sends a member message and streams the assistant's reply: a `start` event, then `delta` events with text as it is generated, then `citations`, `usage`, and a final `done` carrying the complete message. |
| [`DeleteConversation`](#chatservice-deleteconversation) | `/api/career.v1.ChatService/DeleteConversation` | Member | default | `DeleteConversationRequest` → `DeleteConversationResponse` | Deletes a conversation. |
| [`RateMessage`](#chatservice-ratemessage) | `/api/career.v1.ChatService/RateMessage` | Member | default | `RateMessageRequest` → `RateMessageResponse` | Rates an assistant message up or down with an optional comment. |
| [`Escalate`](#chatservice-escalate) | `/api/career.v1.ChatService/Escalate` | Member | 5 | `EscalateRequest` → `EscalateResponse` | Sends a question, with the conversation context, to the owner. |
| [`GetSuggestions`](#chatservice-getsuggestions) | `/api/career.v1.ChatService/GetSuggestions` | Member | default | `GetSuggestionsRequest` → `GetSuggestionsResponse` | Returns suggested questions for the current page and the member's tracks (at least three). |
| [`GetQuota`](#chatservice-getquota) | `/api/career.v1.ChatService/GetQuota` | Member | default | `GetQuotaRequest` → `GetQuotaResponse` | Returns the member's remaining allowance and the global budget mode so the panel can show limits before they are hit. |

### ChatService.CreateConversation

`POST /api/career.v1.ChatService/CreateConversation` · **Auth:** Member · **Rate limit:** 20/min

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

`POST /api/career.v1.ChatService/SendMessage` · **Auth:** Member · **Rate limit:** 20/min · **Server-streaming**

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

`POST /api/career.v1.ChatService/Escalate` · **Auth:** Member · **Rate limit:** 5/min

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
| [`SubmitContact`](#contactservice-submitcontact) | `/api/career.v1.ContactService/SubmitContact` | Public | 3 | `SubmitContactRequest` → `SubmitContactResponse` | Sends a message to the owner. |

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

`POST /api/career.v1.ContactService/SubmitContact` · **Auth:** Public · **Rate limit:** 3/min

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

## AdminService

Owner console.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`ListMembers`](#adminservice-listmembers) | `/api/career.v1.AdminService/ListMembers` | Admin (fresh MFA) | default | `ListMembersRequest` → `ListMembersResponse` | Lists members with search, filters, and pagination. |
| [`ResendNotification`](#adminservice-resendnotification) | `/api/career.v1.AdminService/ResendNotification` | Admin (fresh MFA) | default | `ResendNotificationRequest` → `ResendNotificationResponse` | Re-sends the approval or decline email to a member and returns the audited attempt so the console can show the provider's verdict inline. |
| [`GetMember`](#adminservice-getmember) | `/api/career.v1.AdminService/GetMember` | Admin (fresh MFA) | default | `GetMemberRequest` → `GetMemberResponse` | Returns one member with recent activity, conversations, admin notes, and email delivery history. |
| [`AddMemberNote`](#adminservice-addmembernote) | `/api/career.v1.AdminService/AddMemberNote` | Admin (fresh MFA) | default | `AddMemberNoteRequest` → `AddMemberNoteResponse` | Adds a private admin note to a member. |
| [`SetMemberStatus`](#adminservice-setmemberstatus) | `/api/career.v1.AdminService/SetMemberStatus` | Admin (fresh MFA) | default | `SetMemberStatusRequest` → `SetMemberStatusResponse` | Approves or rejects a registration waiting in PENDING_APPROVAL (approval mode only), or disables/re-enables an account. |
| [`GetReviewQueue`](#adminservice-getreviewqueue) | `/api/career.v1.AdminService/GetReviewQueue` | Admin (fresh MFA) | default | `GetReviewQueueRequest` → `GetReviewQueueResponse` | Lists items needing the owner's attention: negative feedback, "I don't know" answers, escalations, and out-of-scope refusals. |
| [`ResolveReviewItem`](#adminservice-resolvereviewitem) | `/api/career.v1.AdminService/ResolveReviewItem` | Admin (fresh MFA) | default | `ResolveReviewItemRequest` → `ResolveReviewItemResponse` | Resolves a review item, optionally recording that it was converted into a Q&A-bank entry (the entry itself is authored in `content/qa/`). |
| [`ReplyEscalation`](#adminservice-replyescalation) | `/api/career.v1.AdminService/ReplyEscalation` | Admin (fresh MFA) | default | `ReplyEscalationRequest` → `ReplyEscalationResponse` | Replies to an escalation. |
| [`GetMemberConversation`](#adminservice-getmemberconversation) | `/api/career.v1.AdminService/GetMemberConversation` | Admin (fresh MFA) | default | `GetMemberConversationRequest` → `GetMemberConversationResponse` | Returns any member's conversation with messages, sources, and persona versions (read-only). |
| [`GetCorpusStatus`](#adminservice-getcorpusstatus) | `/api/career.v1.AdminService/GetCorpusStatus` | Admin (fresh MFA) | default | `GetCorpusStatusRequest` → `GetCorpusStatusResponse` | Returns corpus statistics: documents, chunks, Q&A entries, last ingest, embedding model, and persona version. |
| [`TestRetrieval`](#adminservice-testretrieval) | `/api/career.v1.AdminService/TestRetrieval` | Admin (fresh MFA) | default | `TestRetrievalRequest` → `TestRetrievalResponse` | Runs retrieval for a question and returns the chunks the assistant would see, with scores ("what would the assistant retrieve?"). |
| [`RunJob`](#adminservice-runjob) | `/api/career.v1.AdminService/RunJob` | Admin (fresh MFA) | default | `RunJobRequest` → `RunJobResponse` | Starts a background job inside the api (corpus reindex of the public or private mount, embed sweep) and returns its id at once; the console polls GetJob for progress. |
| [`GetJob`](#adminservice-getjob) | `/api/career.v1.AdminService/GetJob` | Admin (fresh MFA) | default | `GetJobRequest` → `GetJobResponse` | Returns the status and progress of a job started by RunJob. |
| [`GetPersona`](#adminservice-getpersona) | `/api/career.v1.AdminService/GetPersona` | Admin (fresh MFA) | default | `GetPersonaRequest` → `GetPersonaResponse` | Returns the active persona version and its history. |
| [`GetAnalytics`](#adminservice-getanalytics) | `/api/career.v1.AdminService/GetAnalytics` | Admin (fresh MFA) | default | `GetAnalyticsRequest` → `GetAnalyticsResponse` | Returns aggregate analytics for a date range. |
| [`GetAudit`](#adminservice-getaudit) | `/api/career.v1.AdminService/GetAudit` | Admin (fresh MFA) | default | `GetAuditRequest` → `GetAuditResponse` | Lists audit-log entries. |
| [`ListContactMessages`](#adminservice-listcontactmessages) | `/api/career.v1.AdminService/ListContactMessages` | Admin (fresh MFA) | default | `ListContactMessagesRequest` → `ListContactMessagesResponse` | Lists messages sent via the public contact form (FR-ADM-14). |
| [`ResolveContactMessage`](#adminservice-resolvecontactmessage) | `/api/career.v1.AdminService/ResolveContactMessage` | Admin (fresh MFA) | default | `ResolveContactMessageRequest` → `ResolveContactMessageResponse` | Marks a contact message as resolved (or re-opens it). |
| [`ApproveRegistration`](#adminservice-approveregistration) | `/api/career.v1.AdminService/ApproveRegistration` | Admin (fresh MFA) | default | `ApproveRegistrationRequest` → `ApproveRegistrationResponse` | Approves a pending registration from the admin console. |
| [`DeclineRegistration`](#adminservice-declineregistration) | `/api/career.v1.AdminService/DeclineRegistration` | Admin (fresh MFA) | default | `DeclineRegistrationRequest` → `DeclineRegistrationResponse` | Declines a pending registration from the admin console. |
| [`ExtendAccess`](#adminservice-extendaccess) | `/api/career.v1.AdminService/ExtendAccess` | Admin (fresh MFA) | default | `ExtendAccessRequest` → `ExtendAccessResponse` | Extends an active member's access period by a fixed duration (`extend_days`), or sets a specific new `expires_at`. |
| [`ListDbTables`](#adminservice-listdbtables) | `/api/career.v1.AdminService/ListDbTables` | Admin (fresh MFA) | default | `ListDbTablesRequest` → `ListDbTablesResponse` | Returns the public tables + columns of the API database, from information_schema. |
| [`RunDbQuery`](#adminservice-rundbquery) | `/api/career.v1.AdminService/RunDbQuery` | Admin (fresh MFA) | default | `RunDbQueryRequest` → `RunDbQueryResponse` | Runs a SQL query against the API database from the /admin/db console. |
| [`ListAccessGrants`](#adminservice-listaccessgrants) | `/api/career.v1.AdminService/ListAccessGrants` | Admin (fresh MFA) | default | `ListAccessGrantsRequest` → `ListAccessGrantsResponse` | Lists the access whitelist entries. |
| [`UpsertAccessGrant`](#adminservice-upsertaccessgrant) | `/api/career.v1.AdminService/UpsertAccessGrant` | Admin (fresh MFA) | default | `UpsertAccessGrantRequest` → `UpsertAccessGrantResponse` | Creates a new access whitelist entry or updates an existing one by email (email is the natural key). |
| [`DeleteAccessGrant`](#adminservice-deleteaccessgrant) | `/api/career.v1.AdminService/DeleteAccessGrant` | Admin (fresh MFA) | default | `DeleteAccessGrantRequest` → `DeleteAccessGrantResponse` | Removes a whitelist entry. |
| [`ListSavedQueries`](#adminservice-listsavedqueries) | `/api/career.v1.AdminService/ListSavedQueries` | Admin (fresh MFA) | default | `ListSavedQueriesRequest` → `ListSavedQueriesResponse` | Returns the calling admin's saved SQL statements from /admin/db. |
| [`UpsertSavedQuery`](#adminservice-upsertsavedquery) | `/api/career.v1.AdminService/UpsertSavedQuery` | Admin (fresh MFA) | default | `UpsertSavedQueryRequest` → `UpsertSavedQueryResponse` | Creates or updates a saved query for the caller. |
| [`DeleteSavedQuery`](#adminservice-deletesavedquery) | `/api/career.v1.AdminService/DeleteSavedQuery` | Admin (fresh MFA) | default | `DeleteSavedQueryRequest` → `DeleteSavedQueryResponse` | Removes one of the caller's saved queries by id. |
| [`ListMemberActivity`](#adminservice-listmemberactivity) | `/api/career.v1.AdminService/ListMemberActivity` | Admin (fresh MFA) | default | `ListMemberActivityRequest` → `ListMemberActivityResponse` | Returns one row per member with engagement aggregates (session count, total active time, ask-roger count, last event). |
| [`IngestCorpusText`](#adminservice-ingestcorpustext) | `/api/career.v1.AdminService/IngestCorpusText` | Admin (fresh MFA) | default | `IngestCorpusTextRequest` → `IngestCorpusTextResponse` | Ingests one text document into the Ask Roger corpus. |
| [`ListCorpusDocuments`](#adminservice-listcorpusdocuments) | `/api/career.v1.AdminService/ListCorpusDocuments` | Admin (fresh MFA) | default | `ListCorpusDocumentsRequest` → `ListCorpusDocumentsResponse` | Returns every document currently in the corpus with a per-row chunk count. |
| [`ReindexCorpus`](#adminservice-reindexcorpus) | `/api/career.v1.AdminService/ReindexCorpus` | Admin (fresh MFA) | default | `ReindexCorpusRequest` → `ReindexCorpusResponse` | Walks a corpus mount for markdown files and runs each through IngestCorpusText, so the corpus can be seeded from committed content instead of paste-by-paste. |
| [`SweepCorpusEmbeddings`](#adminservice-sweepcorpusembeddings) | `/api/career.v1.AdminService/SweepCorpusEmbeddings` | Admin (fresh MFA) | default | `SweepCorpusEmbeddingsRequest` → `SweepCorpusEmbeddingsResponse` | Re-embeds every chunk whose vector is missing or was produced by a different embedder than the sidecar's current one. |
| [`ListJdSubmissions`](#adminservice-listjdsubmissions) | `/api/career.v1.AdminService/ListJdSubmissions` | Admin (fresh MFA) | default | `ListJdSubmissionsRequest` → `ListJdSubmissionsResponse` | Returns every JD submission with score + status. |
| [`GetJdSubmission`](#adminservice-getjdsubmission) | `/api/career.v1.AdminService/GetJdSubmission` | Admin (fresh MFA) | default | `GetJdSubmissionRequest` → `GetJdSubmissionResponse` | Returns one JD submission in full: the JD text, both scores, the assessment derivation (requirements, evidence, verdicts) and the generated résumé when present. |
| [`RescoreJd`](#adminservice-rescorejd) | `/api/career.v1.AdminService/RescoreJd` | Admin (fresh MFA) | default | `RescoreJdRequest` → `RescoreJdResponse` | Re-runs the scoring pipeline (retrieval pre-score for diagnostics, per-requirement assessment, score in code, résumé and locked PDF when the fit is strong or better) for one submission in the background, e.g. |
| [`SetJdOutcome`](#adminservice-setjdoutcome) | `/api/career.v1.AdminService/SetJdOutcome` | Admin (fresh MFA) | default | `SetJdOutcomeRequest` → `SetJdOutcomeResponse` | Records what happened in the world after a review: applied, interview, offer, no response. |
| [`RecordJdFeedback`](#adminservice-recordjdfeedback) | `/api/career.v1.AdminService/RecordJdFeedback` | Admin (fresh MFA) | default | `RecordJdFeedbackRequest` → `RecordJdFeedbackResponse` | Records the owner's judgment of one run's output: whether the score was accurate, too generous or too harsh, and whether the résumé is sendable. |
| [`ListGoldenPostings`](#adminservice-listgoldenpostings) | `/api/career.v1.AdminService/ListGoldenPostings` | Admin (fresh MFA) | default | `ListGoldenPostingsRequest` → `ListGoldenPostingsResponse` | Lists the golden set: fixed postings with a stated expectation, re-scored to measure whether a prompt or model change helped. |
| [`UpsertGoldenPosting`](#adminservice-upsertgoldenposting) | `/api/career.v1.AdminService/UpsertGoldenPosting` | Admin (fresh MFA) | default | `UpsertGoldenPostingRequest` → `UpsertGoldenPostingResponse` | Adds or replaces a golden posting, keyed by name. |
| [`SetGoldenActive`](#adminservice-setgoldenactive) | `/api/career.v1.AdminService/SetGoldenActive` | Admin (fresh MFA) | default | `SetGoldenActiveRequest` → `SetGoldenActiveResponse` | Retires or restores a golden posting. |
| [`LabelGoldenPosting`](#adminservice-labelgoldenposting) | `/api/career.v1.AdminService/LabelGoldenPosting` | Admin (fresh MFA) | default | `LabelGoldenPostingRequest` → `LabelGoldenPostingResponse` | Records which side of the gate a posting belongs on. |
| [`ListEvalRuns`](#adminservice-listevalruns) | `/api/career.v1.AdminService/ListEvalRuns` | Admin (fresh MFA) | default | `ListEvalRunsRequest` → `ListEvalRunsResponse` | Lists evaluations, newest first, without their per-posting results. |
| [`GetEvalRun`](#adminservice-getevalrun) | `/api/career.v1.AdminService/GetEvalRun` | Admin (fresh MFA) | default | `GetEvalRunRequest` → `GetEvalRunResponse` | Returns one evaluation with every posting's result. |
| [`GetMetrics`](#adminservice-getmetrics) | `/api/career.v1.AdminService/GetMetrics` | Admin (fresh MFA) | default | `GetMetricsRequest` → `GetMetricsResponse` | Returns the state of the reviewer, read from the SQL views that define each metric once. |
| [`GetGate`](#adminservice-getgate) | `/api/career.v1.AdminService/GetGate` | Admin (fresh MFA) | default | `GetGateRequest` → `GetGateResponse` | Returns the criteria as a gate: one row per criterion with pass, value, target and as_of, read from the views that define them. |
| [`ListDecisionLog`](#adminservice-listdecisionlog) | `/api/career.v1.AdminService/ListDecisionLog` | Admin (fresh MFA) | default | `ListDecisionLogRequest` → `ListDecisionLogResponse` | Lists logged reviewer decisions (per-requirement verdicts, gate outcomes) with the evidence each was made from, for the owner's human-in-the-loop review. |
| [`ReviewDecision`](#adminservice-reviewdecision) | `/api/career.v1.AdminService/ReviewDecision` | Admin (fresh MFA) | default | `ReviewDecisionRequest` → `ReviewDecisionResponse` | Records the owner's own verdict and note on one logged decision. |
| [`ExportDecisionLog`](#adminservice-exportdecisionlog) | `/api/career.v1.AdminService/ExportDecisionLog` | Admin (fresh MFA) | default | `ExportDecisionLogRequest` → `ExportDecisionLogResponse` | Exports decisions as JSON Lines for adapter training and evaluation; reviewed rows carry the human label. |
| [`GetJdFitBands`](#adminservice-getjdfitbands) | `/api/career.v1.AdminService/GetJdFitBands` | Admin (fresh MFA) | default | `GetJdFitBandsRequest` → `GetJdFitBandsResponse` | Reads the JD fit bands (the numbers that classify a review as very strong / strong / possible / weak / very weak; "strong" is the gate). |
| [`SetJdFitBands`](#adminservice-setjdfitbands) | `/api/career.v1.AdminService/SetJdFitBands` | Admin (fresh MFA) | default | `SetJdFitBandsRequest` → `SetJdFitBandsResponse` | Sets the JD fit bands (stored in app_settings; the api caches them for 15 s). |
| [`GetJdSubmissionLimit`](#adminservice-getjdsubmissionlimit) | `/api/career.v1.AdminService/GetJdSubmissionLimit` | Admin (fresh MFA) | default | `GetJdSubmissionLimitRequest` → `GetJdSubmissionLimitResponse` | Reads how many postings one member may submit per rolling day. |
| [`SetJdSubmissionLimit`](#adminservice-setjdsubmissionlimit) | `/api/career.v1.AdminService/SetJdSubmissionLimit` | Admin (fresh MFA) | default | `SetJdSubmissionLimitRequest` → `SetJdSubmissionLimitResponse` | Sets how many postings one member may submit per rolling day (stored in app_settings; the api caches it for 15 s). |

### AdminService.ListMembers

`POST /api/career.v1.AdminService/ListMembers` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ResendNotification` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetMember` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/AddMemberNote` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/SetMemberStatus` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetReviewQueue` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ResolveReviewItem` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ReplyEscalation` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetMemberConversation` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetCorpusStatus` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/TestRetrieval` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/RunJob` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetJob` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetPersona` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetAnalytics` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetAudit` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListContactMessages` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ResolveContactMessage` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ApproveRegistration` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/DeclineRegistration` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ExtendAccess` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListDbTables` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/RunDbQuery` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListAccessGrants` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/UpsertAccessGrant` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/DeleteAccessGrant` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListSavedQueries` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/UpsertSavedQuery` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/DeleteSavedQuery` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListMemberActivity` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/IngestCorpusText` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListCorpusDocuments` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ReindexCorpus` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/SweepCorpusEmbeddings` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListJdSubmissions` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetJdSubmission` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/RescoreJd` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/SetJdOutcome` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/RecordJdFeedback` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListGoldenPostings` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/UpsertGoldenPosting` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/SetGoldenActive` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/LabelGoldenPosting` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListEvalRuns` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetEvalRun` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetMetrics` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

### AdminService.GetGate

`POST /api/career.v1.AdminService/GetGate` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/ListDecisionLog` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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
  "limit": 0
}
```

</details>

### AdminService.ReviewDecision

`POST /api/career.v1.AdminService/ReviewDecision` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

Records the owner's own verdict and note on one logged decision.
Saving again overwrites the label; the model's output is never
changed.

**Request** — [`ReviewDecisionRequest`](#reviewdecisionrequest)

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `id` | `string` | string |  | Row id to label. |
| `humanVerdict` | `string` | string |  | The owner's verdict; the vocabulary of the row's output (met / partial / unmet for requirement verdicts). |
| `humanNote` | `string` | string |  | Optional note explaining the label. |

**Response** — [`ReviewDecisionResponse`](#reviewdecisionresponse)

_No fields; send `{}`._

<details><summary>Example request body</summary>

```json
{
  "id": "string",
  "humanVerdict": "string",
  "humanNote": "string"
}
```

</details>

### AdminService.ExportDecisionLog

`POST /api/career.v1.AdminService/ExportDecisionLog` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/GetJdFitBands` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/SetJdFitBands` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

### AdminService.GetJdSubmissionLimit

`POST /api/career.v1.AdminService/GetJdSubmissionLimit` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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

`POST /api/career.v1.AdminService/SetJdSubmissionLimit` · **Auth:** Admin (fresh MFA) · **Rate limit:** default/min

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
| [`GetVersion`](#systemservice-getversion) | `/api/career.v1.SystemService/GetVersion` | Public | 60 | `GetVersionRequest` → `GetVersionResponse` | Returns the deployed version and build metadata. |
| [`GetGovernanceStatus`](#systemservice-getgovernancestatus) | `/api/career.v1.SystemService/GetGovernanceStatus` | Public | 60 | `GetGovernanceStatusRequest` → `GetGovernanceStatusResponse` | Returns the governance status shown on the public "How this site was built" page: UxTS frameworks, spec counts, last verification, pass rate, and hash-integrity summary, read from the reports CI publishes. |

### SystemService.GetVersion

`POST /api/career.v1.SystemService/GetVersion` · **Auth:** Public · **Rate limit:** 60/min

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

`POST /api/career.v1.SystemService/GetGovernanceStatus` · **Auth:** Public · **Rate limit:** 60/min

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

## JdService

JD-upload flow, members only: a signed-in session is required to
submit or poll, and the submitting member is recorded on the row.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`SubmitJd`](#jdservice-submitjd) | `/api/career.v1.JdService/SubmitJd` | Member | 3 | `SubmitJdRequest` → `SubmitJdResponse` | Accepts a job description and stores it for scoring. |
| [`GetJdResult`](#jdservice-getjdresult) | `/api/career.v1.JdService/GetJdResult` | Member | 30 | `GetJdResultRequest` → `GetJdResultResponse` | Returns the current state of a submission (received / scoring / below-threshold / generating / ready / failed) with progress, the score and fit category once known, and, for the submitting member or a caller holding the result token, the requirement verdicts plus the résumé and its PDF link when status is `ready`. |
| [`ListMySubmissions`](#jdservice-listmysubmissions) | `/api/career.v1.JdService/ListMySubmissions` | Member | 30 | `ListMySubmissionsRequest` → `ListMySubmissionsResponse` | Lists the signed-in member's own submissions, newest first, so a review can be reopened after the tab that submitted it is gone. |
| [`GetJdReviewConfig`](#jdservice-getjdreviewconfig) | `/api/career.v1.JdService/GetJdReviewConfig` | Member | 60 | `GetJdReviewConfigRequest` → `GetJdReviewConfigResponse` | Returns the fit bands in force (the gate is the "strong" edge), so the JD pages quote the numbers the pipeline actually uses. |

### JdService.SubmitJd

`POST /api/career.v1.JdService/SubmitJd` · **Auth:** Member · **Rate limit:** 3/min

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

`POST /api/career.v1.JdService/GetJdResult` · **Auth:** Member · **Rate limit:** 30/min

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

`POST /api/career.v1.JdService/ListMySubmissions` · **Auth:** Member · **Rate limit:** 30/min

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

`POST /api/career.v1.JdService/GetJdReviewConfig` · **Auth:** Member · **Rate limit:** 60/min

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

## EventService

Accepts browser-minted events. Public so the landing funnel is
visible before sign-in; the api attaches identity (session member,
anonymous id cookie), the client address hash and the device class
itself and never trusts those from the client.

| Method | Path | Auth | Rate limit /min | Request → Response | Summary |
|---|---|---|---|---|---|
| [`Record`](#eventservice-record) | `/api/career.v1.EventService/Record` | Public | 120 | `RecordRequest` → `RecordResponse` | Records up to 50 events. |

### EventService.Record

`POST /api/career.v1.EventService/Record` · **Auth:** Public · **Rate limit:** 120/min

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
| `llmReady` | `bool` | boolean |  | LLM provider configured (does not probe the model). |
| `llmProvider` | `string` | string |  | LLM provider name (`stub`, `ollama:<model>`) so the API can decide whether structured pipelines are meaningful. |
| `rendererReady` | `bool` | boolean |  | PDF renderer available (Typst compiler importable). |

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
| `llmReady` | `bool` | boolean |  | LLM provider configured (does not probe the model). |
| `llmProvider` | `string` | string |  | LLM provider name (`stub`, `ollama:<model>`) so the API can decide whether structured pipelines are meaningful. |
| `rendererReady` | `bool` | boolean |  | PDF renderer available (Typst compiler importable). |

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

### ListDecisionLogRequest

Filters for the decision-review list.

| Field (JSON) | Type | JSON encoding | Rules | Description |
|---|---|---|---|---|
| `kind` | `string` | string |  | Restrict to one kind; empty for all kinds. |
| `refId` | `string` | string |  | Restrict to one ref id (e.g. a JD submission); empty for all. |
| `unreviewedOnly` | `bool` | boolean |  | Only rows the owner has not reviewed yet. |
| `limit` | `int32` | number |  | Page size, newest first; server caps at 500 (default 200). |

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
| `humanNote` | `string` | string |  | Optional note explaining the label. |

### ReviewDecisionResponse

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

Who may call a method. Enforced before the handler runs; a caller below
the required level receives the Connect code `unauthenticated` (no
session) or `permission_denied` (session without the role).

| Value | Number | Description |
|---|---|---|
| `AUTH_LEVEL_UNSPECIFIED` | 0 | Not set. Lint fails any method that leaves the level unspecified. |
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

### GetExportResponse.Status

Export lifecycle.

| Value | Number | Description |
|---|---|---|
| `STATUS_UNSPECIFIED` | 0 | Not set. |
| `STATUS_PENDING` | 1 | Queued or running. |
| `STATUS_READY` | 2 | Ready for download. |
| `STATUS_FAILED` | 3 | Failed; the member may request again. |
| `STATUS_EXPIRED` | 4 | Download window elapsed; request again. |
