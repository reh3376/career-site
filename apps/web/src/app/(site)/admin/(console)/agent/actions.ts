"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Server actions behind /admin/agent (FSD FR-ADM-24). Each reports what
// the API said rather than swallowing it, the way the Q&A console does:
// a refused creation is worth reading.

export type CreateState = {
  plaintext?: string;
  username?: string;
  expiresAt?: string;
  error?: string;
};

async function readError(resp: Response): Promise<string> {
  let msg = `HTTP ${resp.status}`;
  try {
    const j = (await resp.json()) as { message?: string };
    if (j.message) msg = j.message;
  } catch {
    /* keep the status */
  }
  return msg;
}

// Creates a one-time password. The plaintext comes back in the state
// for this one render and is never stored or logged anywhere else.
export async function createAgentPasswordAction(
  _prev: CreateState,
  formData: FormData,
): Promise<CreateState> {
  const reason = String(formData.get("reason") ?? "").trim().slice(0, 1000);
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };

  let resp: Response;
  try {
    resp = await callApi({
      path: "/api/career.v1.AdminService/CreateAgentPassword",
      body: { reason },
      cookie,
    });
  } catch (e) {
    return { error: e instanceof Error ? e.message : "The request did not reach the API." };
  }
  if (!resp.ok) return { error: await readError(resp) };

  const j = (await resp.json()) as {
    plaintext?: string;
    username?: string;
    password?: { expiresAt?: string; expires_at?: string };
  };
  revalidatePath("/admin/agent", "layout");
  return {
    plaintext: j.plaintext,
    username: j.username,
    expiresAt: j.password?.expiresAt ?? j.password?.expires_at,
  };
}

export async function revokeAgentPasswordAction(formData: FormData): Promise<void> {
  const id = String(formData.get("id") ?? "");
  const reason = String(formData.get("reason") ?? "").trim().slice(0, 1000);
  if (!id) return;
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/RevokeAgentPassword",
    body: { id, reason },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/agent", "layout");
}

export async function revokeAgentSessionsAction(formData: FormData): Promise<void> {
  const reason = String(formData.get("reason") ?? "").trim().slice(0, 1000);
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.AdminService/RevokeAgentSessions",
    body: { reason },
    cookie,
  }).catch(() => undefined);
  revalidatePath("/admin/agent", "layout");
}
