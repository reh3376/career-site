"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { clearSession, getSessionCookie } from "@/lib/session";

// Shared sign-out server action. Any component that needs to render a
// sign-out button should import this rather than duplicating the flow.
// Fire-and-forget the API logout: even if the API is unreachable we
// still clear the browser's cookie so the session view is consistent.
export async function signOutAction(): Promise<void> {
  const cookie = await getSessionCookie();
  if (cookie) {
    await callApi({
      path: "/api/career.v1.AuthService/Logout",
      body: {},
      cookie,
    }).catch(() => undefined);
  }
  await clearSession();
  redirect("/");
}
