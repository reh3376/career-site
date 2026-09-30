import type { Metadata } from "next";

import { getSchedulerSettings } from "./actions";
import { WindowsForm } from "./windows-form";

export const metadata: Metadata = { title: "Scheduler" };
export const dynamic = "force-dynamic";

// The owner's own availability. Everything here is stored in
// app_settings and read by the booking flow within seconds, so
// "not next Tuesday" never needs a deploy.
export default async function AdminSchedulerPage() {
  const { settings, calendarConnected, calendarStatus } = await getSchedulerSettings();

  return (
    <div className="max-w-3xl">
      <h1 className="font-display text-3xl text-ink">Scheduler</h1>
      <p className="mt-3 text-sm leading-relaxed text-ink-2">
        The hours members may book into. These windows are the outer bound: your calendar
        subtracts from them and never adds, so a free evening stays unbookable because
        availability is a decision you made rather than a gap Google happened to report.
      </p>

      {/* Whether a calendar is connected is a different question from
          what the windows say, and showing a full week without it would
          imply bookings are possible when none are. */}
      <div
        className={
          calendarConnected
            ? "mt-6 rounded-md border border-line bg-paper-2/60 p-4"
            : "mt-6 rounded-md border border-signal/40 bg-paper-2/60 p-4"
        }
      >
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">calendar</p>
        <p className="mt-2 text-sm text-ink-2">
          {calendarConnected
            ? "Connected. Free/busy is read before every set of times offered, and again at the moment of booking."
            : calendarStatus || "Not connected, so nothing can be booked yet."}
        </p>
        {calendarConnected ? null : (
          <p className="mt-2 text-xs text-ink-3">
            These settings are still saved and will take effect as soon as a calendar is
            connected. Members see that booking is not switched on rather than an empty calendar.
          </p>
        )}
      </div>

      <WindowsForm initial={settings} />
    </div>
  );
}
