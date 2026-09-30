import type { Metadata } from "next";
import { redirect } from "next/navigation";

import { getSessionUser } from "@/lib/session-user";

import { getMeetingOptions, listMyMeetings } from "./actions";
import { Booking } from "./booking";

export const metadata: Metadata = {
  title: "Book a meeting",
  description:
    "Pick a time from Roger's real availability and book it. Fifteen, thirty or forty-five minutes, confirmed against his calendar as you book.",
};
export const dynamic = "force-dynamic";

// Members only. The API rejects anonymous calls too; this sends a
// visitor to sign in rather than showing a calendar that cannot book.
//
// The decision to keep this behind an account is the owner's, and the
// reason is that the calendar underneath is his own: an open booking
// page would let anyone hold real hours on it.
export default async function MeetingsPage() {
  const me = await getSessionUser();
  if (!me) redirect("/login?next=/meetings");

  const [options, meetings] = await Promise.all([getMeetingOptions(), listMyMeetings()]);

  return (
    <div className="mx-auto max-w-3xl px-6 py-16 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        talk to roger
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Book a meeting.
      </h1>
      <p className="mt-6 max-w-2xl text-lg leading-relaxed text-ink-2">
        These are real openings on Roger&rsquo;s calendar, not a form that
        starts a negotiation. Pick a length and a time and it is booked,
        checked against his calendar at the moment you confirm. You will get
        a calendar file either way, so you can send your own invitation if
        you would rather.
      </p>

      <Booking options={options} initialMeetings={meetings} />
    </div>
  );
}
