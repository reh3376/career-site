"use server";

import { callApi } from "@/lib/api-fetch";
import { relaySetCookies } from "@/lib/session";

export type CodeState = {
  error?: string;
  values?: { email?: string };
  ok?: boolean;
  name?: string;
  status?: string;
  pilot?: boolean;
};

// The second half of the verification email: the six-digit code, for
// the person whose mail client will not open the link, or who is
// reading the email on one device and the site on another. Found
// missing in the owner's live review (15 §11 step 2): the email
// promised a place to enter the code and the page had none.
export async function verifyByCodeAction(_prev: CodeState, formData: FormData): Promise<CodeState> {
  const email = String(formData.get("email") ?? "").trim().toLowerCase();
  const code = String(formData.get("code") ?? "").replace(/\D/g, "");
  const values = { email };
  if (!email) return { error: "Enter the email address you registered with.", values };
  if (code.length !== 6) return { error: "The code is the six digits from the email.", values };

  const resp = await callApi({
    path: "/api/career.v1.AuthService/Verify",
    body: { code, email },
  });
  if (!resp.ok) {
    let message = "That code didn't work.";
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) message = j.message;
    } catch {
      /* keep the generic message */
    }
    return { error: message, values };
  }
  // If the API ever starts a session here, the browser keeps it.
  await relaySetCookies(resp.headers);
  const j = (await resp.json()) as { me?: { name?: string; status?: string; pilotApproved?: boolean } };
  return { ok: true, name: j.me?.name ?? "", status: j.me?.status ?? "", pilot: j.me?.pilotApproved === true, values };
}
