"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Server actions behind /admin/access. Same shape as the contacts and
// registrations surfaces: FormData in, ConnectRPC JSON out through
// callApi with the admin's session cookie, then revalidate.

const TTL_VALUES = new Set([
  "GRANT_TTL_1D",
  "GRANT_TTL_3D",
  "GRANT_TTL_7D",
  "GRANT_TTL_30D",
  "GRANT_TTL_PERMANENT",
]);

export type UpsertGrantResult =
  | { ok: true }
  | { ok: false; message: string };

export async function upsertAccessGrantAction(
  formData: FormData,
): Promise<UpsertGrantResult> {
  const email = String(formData.get("email") ?? "").trim().toLowerCase();
  const defaultTtl = String(formData.get("default_ttl") ?? "");
  const notes = String(formData.get("notes") ?? "").trim();
  const entryExpiresAtRaw = String(formData.get("entry_expires_at") ?? "").trim();

  if (!email) return { ok: false, message: "Email is required." };
  if (!TTL_VALUES.has(defaultTtl)) {
    return { ok: false, message: "Pick a TTL." };
  }

  const body: Record<string, unknown> = {
    email,
    defaultTtl,
    notes,
  };
  if (entryExpiresAtRaw) {
    const t = new Date(entryExpiresAtRaw);
    if (Number.isNaN(t.getTime())) {
      return { ok: false, message: "Invalid entry_expires_at date." };
    }
    body.entryExpiresAt = t.toISOString();
  }

  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, message: "Not signed in." };

  const resp = await callApi({
    path: "/api/career.v1.AdminService/UpsertAccessGrant",
    body,
    cookie,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep default */
    }
    return { ok: false, message: msg };
  }
  revalidatePath("/admin/access", "layout");
  return { ok: true };
}

export async function deleteAccessGrantAction(
  formData: FormData,
): Promise<void> {
  const id = String(formData.get("id") ?? "");
  if (!id) return;

  const cookie = await getSessionCookie();
  if (!cookie) return;

  await callApi({
    path: "/api/career.v1.AdminService/DeleteAccessGrant",
    body: { id },
    cookie,
  }).catch(() => undefined);

  revalidatePath("/admin/access", "layout");
}
