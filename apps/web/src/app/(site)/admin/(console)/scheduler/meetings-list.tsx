"use client";

import { useState, useTransition } from "react";

import { cancelMeetingAsAdmin, type AdminMeeting } from "./actions";

function fmt(iso: string, zone: string, opts: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat("en-US", { ...opts, timeZone: zone }).format(new Date(iso));
}

export function MeetingsList({
  meetings,
  zone,
}: {
  meetings: AdminMeeting[];
  zone: string;
}) {
  const [error, setError] = useState("");
  const [confirming, setConfirming] = useState("");
  const [busy, start] = useTransition();

  function cancel(id: string) {
    setError("");
    start(async () => {
      const { ok, error: err } = await cancelMeetingAsAdmin(id);
      if (!ok) setError(err || "The meeting could not be cancelled.");
      setConfirming("");
    });
  }

  return (
    <section className="mt-12 border-t border-line pt-8">
      <h2 className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        booked meetings
      </h2>

      {error ? (
        <p role="alert" className="mt-3 text-sm text-signal">
          {error}
        </p>
      ) : null}

      {meetings.length === 0 ? (
        <p className="mt-4 text-sm text-ink-3">Nothing booked ahead.</p>
      ) : (
        <ul className="mt-4 space-y-4">
          {meetings.map((m) => (
            <li key={m.id} className="border-l-2 border-line pl-4">
              <p className="text-sm text-ink">
                {fmt(m.start, zone, { weekday: "short", month: "short", day: "numeric" })}
                {", "}
                {fmt(m.start, zone, { hour: "numeric", minute: "2-digit" })}
                <span className="text-ink-3"> · {m.durationMinutes} min</span>
                {m.cancelledAt ? <span className="text-signal"> · cancelled</span> : null}
              </p>
              <p className="mt-1 text-sm text-ink-2">
                {m.memberName || "a member"}{" "}
                <span className="text-ink-3">{m.memberEmail}</span>
              </p>
              {m.topic ? <p className="mt-1 text-sm text-ink-3">{m.topic}</p> : null}
              {/* A claim with no event means the slot is held here but
                  nothing reached the calendar, which is worth seeing
                  rather than discovering by missing the meeting. */}
              {!m.eventId && !m.cancelledAt ? (
                <p className="mt-1 text-xs text-signal">
                  Held here, but no calendar event was created.
                </p>
              ) : null}

              {m.cancelledAt ? null : confirming === m.id ? (
                <p className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1 text-sm">
                  <span className="text-ink-2">Cancel and free the slot?</span>
                  <button
                    type="button"
                    onClick={() => cancel(m.id)}
                    disabled={busy}
                    className="text-signal underline decoration-line decoration-1 underline-offset-4"
                  >
                    Yes, cancel
                  </button>
                  <button
                    type="button"
                    onClick={() => setConfirming("")}
                    className="text-ink-3 hover:text-accent"
                  >
                    Keep it
                  </button>
                </p>
              ) : (
                <button
                  type="button"
                  onClick={() => setConfirming(m.id)}
                  className="mt-2 text-sm text-ink-3 transition-colors hover:text-signal"
                >
                  Cancel
                </button>
              )}
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
