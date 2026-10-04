"use server";

import { signOutAction } from "@/app/actions/session";
import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// The /home sign-out button funnels through the shared session
// action so every sign-out path (hamburger, member home, admin
// surface) uses the same code. Wrapped in an async function
// because "use server" files only allow async-function exports.
export async function logoutAction(): Promise<void> {
  await signOutAction();
}

// Records one activity event on behalf of the signed-in caller. The
// component that calls this passes a stable `clientEventId` so a
// double-invocation (React Strict Mode, back/forward navigation) is
// deduped at the API's ON CONFLICT layer. Failures never surface,
// the beacon is best-effort telemetry.
export async function recordActivityAction(input: {
  kind: string;
  contentId?: string;
  clientEventId: string;
  dwellMs?: number;
}): Promise<void> {
  const cookie = await getSessionCookie();
  if (!cookie) return;
  await callApi({
    path: "/api/career.v1.ActivityService/RecordEvents",
    body: {
      events: [
        {
          kind: input.kind,
          contentId: input.contentId ?? "",
          clientEventId: input.clientEventId,
          dwellMs: input.dwellMs ?? 0,
          occurredAt: new Date().toISOString(),
        },
      ],
    },
    cookie,
  }).catch(() => undefined);
}
