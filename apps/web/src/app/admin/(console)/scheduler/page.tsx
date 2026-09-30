import type { Metadata } from "next";

import { getCalendarStatus, getSchedulerSettings } from "./actions";
import { ConnectPanel } from "./connect-panel";
import { WindowsForm } from "./windows-form";

export const metadata: Metadata = { title: "Scheduler" };
export const dynamic = "force-dynamic";

// The owner's own availability. Everything here is stored in
// app_settings and read by the booking flow within seconds, so
// "not next Tuesday" never needs a deploy.
export default async function AdminSchedulerPage() {
  const [{ settings }, calendar] = await Promise.all([
    getSchedulerSettings(),
    getCalendarStatus(),
  ]);

  return (
    <div className="max-w-3xl">
      <h1 className="font-display text-3xl text-ink">Scheduler</h1>
      <p className="mt-3 text-sm leading-relaxed text-ink-2">
        The hours members may book into. These windows are the outer bound: your calendar
        subtracts from them and never adds, so a free evening stays unbookable because
        availability is a decision you made rather than a gap Google happened to report.
      </p>

      <ConnectPanel status={calendar} />

      <WindowsForm initial={settings} />
    </div>
  );
}
