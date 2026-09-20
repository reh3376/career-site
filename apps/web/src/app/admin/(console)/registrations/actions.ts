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
