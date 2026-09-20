"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { relaySetCookies } from "@/lib/session";

export type ResetState = {
  error?: string;
  values?: { token?: string };
};

// Server action for /reset-password. Consumes the single-use token,
// updates the password, and (on success) relays the fresh session
// cookie from the API before redirecting to /home. Errors from the
// backend (expired token, breached password, etc.) surface as
// state.error so the client can render inline.
export async function resetPasswordAction(
  _prev: ResetState,
  formData: FormData,
): Promise<ResetState> {
  const token = String(formData.get("token") ?? "").trim();
  const newPassword = String(formData.get("new_password") ?? "");
  const confirm = String(formData.get("confirm") ?? "");

  if (!token) {
    return {
      error: "This reset link is missing its token. Request a new one.",
      values: { token },
    };
  }
  if (newPassword.length < 12) {
    return {
      error: "Password must be at least 12 characters.",
      values: { token },
    };
  }
  if (newPassword !== confirm) {
    return { error: "Passwords do not match.", values: { token } };
  }

  const resp = await callApi({
    path: "/api/career.v1.AuthService/ResetPassword",
    body: { token, newPassword },
  });

  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep default */
    }
    return { error: msg, values: { token } };
  }

  await relaySetCookies(resp.headers);
  redirect("/home");
}
