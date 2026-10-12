"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Server actions for the console-side approve / decline buttons on
// /admin/registrations. Forwards the admin session cookie via callApi
// and revalidates the layout so counts, list, and the row's own state
// all update on the next paint.
//
// Each of these four carries a `reason`, read from the row's reason box.
// The proto fields have existed since the first contract, with comments
// saying the value is "recorded in the audit log"; until 2026-10-12 no
// handler read them and no audit log existed to record them in. Now the
// audit interceptor writes the reason into the row's payload, so a blank
// box is a real choice rather than the only possibility.

// reasonFrom reads the optional free-text reason for an action. Empty is
// allowed: requiring one would mean an admin with nothing to add types a
// character to get past the form, and a log full of "." is worse than a
// log of blanks.
function reasonFrom(formData: FormData): string {
  return String(formData.get("reason") ?? "").trim().slice(0, 1000);
}
export async function approveRegistrationAction(formData: FormData): Promise<void> {
  const memberId = String(formData.get("member_id") ?? "");
  if (!memberId) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/ApproveRegistration",
    body: { memberId, reason: reasonFrom(formData) },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/registrations", "layout");
}

export async function declineRegistrationAction(formData: FormData): Promise<void> {
  const memberId = String(formData.get("member_id") ?? "");
  if (!memberId) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/DeclineRegistration",
    body: { memberId, reason: reasonFrom(formData) },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/registrations", "layout");
}

// Console flip between ACTIVE and DISABLED for a non-admin member.
// The `status` field must be one of MEMBER_STATUS_ACTIVE /
// MEMBER_STATUS_DISABLED; anything else is rejected server-side.
export async function setMemberStatusAction(formData: FormData): Promise<void> {
  const memberId = String(formData.get("member_id") ?? "");
  const status = String(formData.get("status") ?? "");
  if (!memberId || !status) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/SetMemberStatus",
    body: { memberId, status, reason: reasonFrom(formData) },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/registrations", "layout");
}

// Re-fire the approval / decline email. The API audits the attempt
// and the page re-renders with the new delivery row at the top of
// the history, so the admin sees the provider's verdict inline.
export async function resendNotificationAction(formData: FormData): Promise<void> {
  const memberId = String(formData.get("member_id") ?? "");
  const kind = String(formData.get("kind") ?? "");
  if (!memberId || !kind) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/ResendNotification",
    body: { memberId, kind },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/registrations", "layout");
}

// Extend a member's access. `mode` picks which of the three ExtendAccess
// request fields to populate: "days" (relative), "permanent" (clears
// expires_at). "days" reads from formData.get("days"), the button
// component passes a hidden `days` field for each preset.
export async function extendAccessAction(formData: FormData): Promise<void> {
  const memberId = String(formData.get("member_id") ?? "");
  const mode = String(formData.get("mode") ?? "");
  if (!memberId || !mode) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;

  type Body = {
    memberId: string;
    extendDays?: number;
    permanent?: boolean;
    reason?: string;
  };
  const body: Body = { memberId, reason: reasonFrom(formData) };
  if (mode === "permanent") {
    body.permanent = true;
  } else if (mode === "days") {
    const n = Number(formData.get("days") ?? "0");
    if (!Number.isFinite(n) || n <= 0) return;
    body.extendDays = Math.floor(n);
  } else {
    return;
  }

  await callApi({
    path: "/api/career.v1.AdminService/ExtendAccess",
    body,
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/registrations", "layout");
}
