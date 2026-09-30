"use client";

import { useState, useTransition } from "react";

import { cancelMeetingAction, type Meeting } from "./actions";

function fmt(iso: string, zone: string, opts: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat("en-US", { ...opts, timeZone: zone || "UTC" }).format(
    new Date(iso),
  );
}

// A member's own meetings, with a way out of them.
//
// The cancel path existed as an RPC from the first commit and was never
// wired to anything, so a member could book and then had no recourse
// but to email Roger and ask him to undo it. That is the whole point of
// letting them book in the first place.
export function MyMeetings({
  meetings,
  zoneLabel,
  onChanged,
}: {
  meetings: Meeting[];
  zoneLabel: string;
  // Cancelling frees the slot, so the times on the page above are stale
  // the moment it succeeds.
  onChanged: () => void;
}) {
  const [confirming, setConfirming] = useState("");
  const [error, setError] = useState("");
  const [busy, start] = useTransition();

  const upcoming = meetings.filter((m) => !m.cancelledAt);
  if (upcoming.length === 0) return null;

  function cancel(id: string) {
    setError("");
    start(async () => {
      const { ok, error: err } = await cancelMeetingAction(id);
      if (!ok) setError(err || "The meeting could not be cancelled.");
      setConfirming("");
      onChanged();
    });
  }

  return (
    <section className="mt-16 border-t border-line pt-8">
      <h2 className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        your meetings
      </h2>

      {error ? (
        <p role="alert" className="mt-3 text-sm text-signal">
          {error}
        </p>
      ) : null}

      <ul className="mt-4 space-y-5">
        {upcoming.map((m) => (
          <li key={m.id} className="border-l-2 border-line pl-4">
            <p className="text-sm text-ink">
              {fmt(m.start, m.zone, { weekday: "long", month: "long", day: "numeric" })}
              {", "}
              {fmt(m.start, m.zone, { hour: "numeric", minute: "2-digit" })}
              <span className="text-ink-3">
                {" "}
                · {m.durationMinutes} minutes, {zoneLabel}
              </span>
            </p>
            {m.topic ? <p className="mt-1 text-sm text-ink-3">{m.topic}</p> : null}

            <div className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
              <a
                href={m.icsUrl}
                className="text-ink-2 underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
              >
                Calendar file
              </a>
              {confirming === m.id ? (
                <>
                  <span className="text-ink-2">Cancel this meeting?</span>
                  <button
                    type="button"
                    onClick={() => cancel(m.id)}
                    disabled={busy}
                    className="text-signal underline decoration-line decoration-1 underline-offset-4 disabled:opacity-60"
                  >
                    Yes, cancel
                  </button>
                  <button
                    type="button"
                    onClick={() => setConfirming("")}
                    className="text-ink-3 transition-colors hover:text-accent"
                  >
                    Keep it
                  </button>
                </>
              ) : (
                <button
                  type="button"
                  onClick={() => setConfirming(m.id)}
                  className="text-ink-3 transition-colors hover:text-signal"
                >
                  Cancel
                </button>
              )}
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}
