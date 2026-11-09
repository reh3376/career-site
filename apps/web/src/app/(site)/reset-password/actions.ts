"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { relaySetCookies } from "@/lib/session";

export type ResetState = {
  error?: string;
  values?: { email?: string };
};

// Server action for /reset-password. Sends the address, the emailed
// 5-digit code and the new password; on success relays the fresh
// session cookie from the API and redirects to /home (FSD FR-AUTH-19).
// Errors from the backend (wrong or expired code, breached password)
// surface as state.error so the client can render them inline.
export async function resetPasswordAction(
  _prev: ResetState,
  formData: FormData,
): Promise<ResetState> {
  const email = String(formData.get("email") ?? "").trim();
  const code = String(formData.get("code") ?? "").replace(/\D/g, "");
  const newPassword = String(formData.get("new_password") ?? "");
  const confirm = String(formData.get("confirm") ?? "");

  if (!email) {
    return { error: "Enter the email address the code was sent to.", values: { email } };
  }
  if (code.length !== 5) {
    return { error: "The code is the 5 digits from the email.", values: { email } };
  }
  if (newPassword.length < 12) {
    return { error: "Password must be at least 12 characters.", values: { email } };
  }
  if (newPassword !== confirm) {
    return { error: "Passwords do not match.", values: { email } };
  }

  const resp = await callApi({
    path: "/api/career.v1.AuthService/ResetPassword",
    body: { email, code, newPassword },
  });

  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep default */
    }
    return { error: msg, values: { email } };
  }

  await relaySetCookies(resp.headers);
  redirect("/home");
}
