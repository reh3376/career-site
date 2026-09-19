"use server";

import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { clearSession, getSessionCookie } from "@/lib/session";

export async function logoutAction(): Promise<void> {
  const cookie = await getSessionCookie();
  if (cookie) {
    // Fire-and-forget: even if the API is unavailable, we clear the cookie
    // client-side so the user is signed out from their browser's view.
    await callApi({
      path: "/api/career.v1.AuthService/Logout",
      body: {},
      cookie,
    }).catch(() => undefined);
  }
  await clearSession();
  redirect("/");
}
