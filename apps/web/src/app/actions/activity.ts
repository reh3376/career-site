"use server";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Activity instrumentation, shared by every page that records a view.
//
// This lived under app/(site)/home/ and was reachable only from there,
// which is why /home was the only instrumented page in the app: the
// member history held 27 view events across exactly one distinct
// content id, and that id was /home itself. FR-HIST-02's "up to three
// items viewed last time" had nothing to list. Moving the action here
// is what makes the rest of the instrumentation possible.

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
