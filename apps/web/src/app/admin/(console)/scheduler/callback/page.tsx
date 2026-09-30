import type { Metadata } from "next";
import Link from "next/link";

import { connectCalendar } from "../actions";

export const metadata: Metadata = { title: "Connecting calendar" };
export const dynamic = "force-dynamic";

// Where Google sends the owner back after he authorises. The URL
// registered with Google is this route, so the code arrives in the
// browser; the exchange happens server-side, because the client secret
// must never reach a page.
//
// The code is single-use, so this runs the exchange once as the page
// renders and shows the outcome. A refresh will fail, which is correct:
// the second attempt is spending a code Google has already retired.
export default async function CalendarCallbackPage({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}) {
  const params = await searchParams;
  const one = (v: string | string[] | undefined) => (Array.isArray(v) ? v[0] : v) ?? "";

  const code = one(params.code);
  const state = one(params.state);
  // Google reports a refusal in the query string rather than by failing
  // the redirect, so a declined consent screen lands here looking like
  // a success until this is read.
  const denied = one(params.error);

  let result: { ok: boolean; error?: string };
  if (denied) {
    result = {
      ok: false,
      error:
        denied === "access_denied"
          ? "You declined the consent screen, so nothing was connected."
          : `Google reported: ${denied}`,
    };
  } else if (!code || !state) {
    result = { ok: false, error: "That callback was missing its code or state." };
  } else {
    result = await connectCalendar(code, state);
  }

  return (
    <div className="max-w-2xl">
      <h1 className="font-display text-3xl text-ink">
        {result.ok ? "Calendar connected." : "Not connected."}
      </h1>
      <p className="mt-4 text-sm leading-relaxed text-ink-2">
        {result.ok
          ? "Booking is live. Free/busy is read before every set of times offered, and again at the moment a member confirms."
          : result.error}
      </p>
      {result.ok ? null : (
        <p className="mt-3 text-xs text-ink-3">
          Authorisation codes are single use, so reloading this page will not retry. Start again
          from the scheduler.
        </p>
      )}
      <Link
        href="/admin/scheduler"
        className="mt-8 inline-flex items-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white no-underline transition-colors hover:bg-accent-hover"
      >
        Back to the scheduler
      </Link>
    </div>
  );
}
