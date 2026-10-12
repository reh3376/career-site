"use server";

import { signOutAction } from "@/app/actions/session";

// The /home sign-out button funnels through the shared session
// action so every sign-out path (hamburger, member home, admin
// surface) uses the same code. Wrapped in an async function
// because "use server" files only allow async-function exports.
export async function logoutAction(): Promise<void> {
  await signOutAction();
}
