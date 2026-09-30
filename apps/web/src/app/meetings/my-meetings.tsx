"use client";

import { useState, useTransition } from "react";

import { cancelMeetingAction, VIDEO_PROVIDERS, type Meeting } from "./actions";

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
  const [open, setOpen] = useState("");
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

      <ul className="mt-4 space-y-3">
        {upcoming.map((m) => {
          const expanded = open === m.id;
          const setup = VIDEO_PROVIDERS.find((v) => v.value === m.videoProvider);
          return (
            <li key={m.id} className="rounded-md border border-line bg-paper-2/60">
              {/* The row is the control. A meeting is a thing you look
                  into, not a line of metadata, and folding the detail
                  away keeps the list scannable when there are several. */}
              <button
                type="button"
                aria-expanded={expanded}
                onClick={() => setOpen(expanded ? "" : m.id)}
                className="flex w-full items-baseline justify-between gap-4 px-4 py-3 text-left"
              >
                <span className="text-sm text-ink">
                  {fmt(m.start, m.zone, { weekday: "long", month: "long", day: "numeric" })}
                  {", "}
                  {fmt(m.start, m.zone, { hour: "numeric", minute: "2-digit" })}
                </span>
                <span className="shrink-0 text-sm text-ink-3">
                  {m.durationMinutes} min {expanded ? "\u2013" : "+"}
                </span>
              </button>

              {expanded ? (
                <div className="border-t border-line px-4 py-3">
                  <dl className="space-y-2 text-sm">
                    <div className="flex gap-3">
                      <dt className="w-24 shrink-0 text-ink-3">When</dt>
                      <dd className="m-0 text-ink-2">
                        {fmt(m.start, m.zone, {
                          weekday: "long",
                          month: "long",
                          day: "numeric",
                          hour: "numeric",
                          minute: "2-digit",
                        })}
                        , {zoneLabel}
                      </dd>
                    </div>
                    <div className="flex gap-3">
                      <dt className="w-24 shrink-0 text-ink-3">How</dt>
                      <dd className="m-0 text-ink-2">
                        {setup
                          ? `Video on ${setup.label}, which you host`
                          : m.meetingType === "MEETING_TYPE_PHONE"
                            ? m.phoneNumber
                              ? `Phone, you calling from ${m.phoneNumber}`
                              : "Phone, number in the comments"
                            : "Not recorded"}
                      </dd>
                    </div>
                    {m.topic ? (
                      <div className="flex gap-3">
                        <dt className="w-24 shrink-0 text-ink-3">About</dt>
                        <dd className="m-0 text-ink-2">{m.topic}</dd>
                      </div>
                    ) : null}
                  </dl>

                  <div className="mt-4 flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
                    <a
                      href={m.icsUrl}
                      className="text-ink-2 underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
                    >
                      Calendar file
                    </a>
                    {setup ? (
                      <a
                        href={setup.setupUrl}
                        target="_blank"
                        rel="noopener noreferrer"
                        className="text-ink-2 underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
                      >
                        Set up the {setup.label} link
                      </a>
                    ) : null}
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
                </div>
              ) : null}
            </li>
          );
        })}
      </ul>
    </section>
  );
}
