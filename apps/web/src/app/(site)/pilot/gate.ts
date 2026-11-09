import "server-only";

import { notFound, redirect } from "next/navigation";

import { getSessionUser } from "@/lib/session-user";

import type { Fetched } from "./lib";

// The two halves of the pilot gate on every page under /pilot.
//
// First the session: no session goes to sign-in with the way back.
// Then the data call, whose failure codes are the gate's own answers:
// 403 is the API refusing a member without the flag, and the page
// gives the same 404 a page that does not exist gives, so the pilot's
// existence is not discoverable from here (FR-PLT-01); 412 is "start
// the pilot first" or "answer section A first" and goes back to the
// pilot home, which knows where the participant stands; 404 is a
// section that is not in the instrument. Anything else is reported.
export async function requireSignedIn(nextPath: string) {
  const me = await getSessionUser();
  if (!me) redirect(`/login?next=${encodeURIComponent(nextPath)}`);
  return me;
}

export function unwrap<T>(res: Fetched<T>): T {
  if (res.ok) return res.data;
  if (res.status === 403) notFound();
  if (res.status === 404) notFound();
  if (res.status === 412) redirect("/pilot");
  if (res.status === 401) redirect("/login?next=/pilot");
  throw new Error(`The pilot could not be loaded, ${res.message}.`);
}
