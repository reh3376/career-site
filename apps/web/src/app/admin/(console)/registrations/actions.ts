"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Server actions for the console-side approve / decline buttons on
// /admin/registrations. Forwards the admin session cookie via callApi
// and revalidates the layout so counts, list, and the row's own state
// all update on the next paint.
export async function approveRegistrationAction(formData: FormData): Promise<void> {
  const memberId = String(formData.get("member_id") ?? "");
  if (!memberId) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/ApproveRegistration",
    body: { memberId },
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
    body: { memberId },
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
    body: { memberId, status },
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
  };
  const body: Body = { memberId };
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
