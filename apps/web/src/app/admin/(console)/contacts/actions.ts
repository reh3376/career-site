"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Server action wired to the resolve/re-open button on each row of
// /admin/contacts. Forwards the admin session cookie via callApi, then
// revalidates the page so the row's state updates without a full
// reload.
export async function setContactResolvedAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  const target = String(formData.get("target") ?? "SUPPORT_STATUS_RESOLVED");
  if (!id) return;

  const cookie = await getSessionCookie();
  if (!cookie) return;

  await callApi({
    path: "/api/career.v1.AdminService/ResolveContactMessage",
    body: { id, status: target },
    cookie,
  }).catch(() => undefined);

  // The list page filters by ?status; revalidating the layout keeps
  // both the row and the header counts in sync on the next paint.
  revalidatePath("/admin/contacts", "layout");
}
