"use client";

import { useEffect, useMemo, useState, useTransition } from "react";

import {
  bookMeetingAction,
  getAvailability,
  type AvailabilityState,
  type BookState,
  type Meeting,
  type MeetingOptions,
  type Slot,
} from "./actions";

// Every time on this page is rendered in the owner's zone, never the
// visitor's, and the zone is printed beside it. Converting to the
// visitor's clock would be friendlier right up to the moment somebody
// travels, and a meeting an hour out is worse than a label to read.
function formatter(zone: string, opts: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat("en-US", { ...opts, timeZone: zone || "UTC" });
}

function dayKey(iso: string, zone: string) {
  return formatter(zone, { year: "numeric", month: "2-digit", day: "2-digit" }).format(new Date(iso));
}

function dayLabel(iso: string, zone: string) {
  return formatter(zone, { weekday: "long", month: "long", day: "numeric" }).format(new Date(iso));
}

function timeLabel(iso: string, zone: string) {
  return formatter(zone, { hour: "numeric", minute: "2-digit" }).format(new Date(iso));
}

type Props = {
  options: MeetingOptions;
  initialMeetings: Meeting[];
};

export function Booking({ options, initialMeetings }: Props) {
  const durations = options.durationMinutes;
  const [duration, setDuration] = useState(durations[0] ?? 30);
  const [availability, setAvailability] = useState<AvailabilityState | null>(null);
  const [selected, setSelected] = useState<Slot | null>(null);
  const [topic, setTopic] = useState("");
  const [contact, setContact] = useState("");
  const [result, setResult] = useState<BookState>({});
  const [loading, startLoading] = useTransition();
  const [booking, startBooking] = useTransition();

  // Refetch whenever the length changes, because availability is
  // computed per length rather than filtered from one list. Clearing
  // the selection is done where the length is changed rather than here:
  // it is a consequence of the click, and doing it in the effect body
  // costs a second render pass for nothing.
  useEffect(() => {
    let cancelled = false;
    startLoading(async () => {
      const next = await getAvailability(duration);
      if (!cancelled) setAvailability(next);
    });
    return () => {
      cancelled = true;
    };
  }, [duration]);

  const zone = availability?.zone || options.zone;

  const days = useMemo(() => {
    const slots = availability?.slots ?? [];
    const grouped = new Map<string, { label: string; slots: Slot[] }>();
    for (const slot of slots) {
      const key = dayKey(slot.start, zone);
      if (!grouped.has(key)) grouped.set(key, { label: dayLabel(slot.start, zone), slots: [] });
      grouped.get(key)!.slots.push(slot);
    }
    return [...grouped.values()];
  }, [availability, zone]);

  function refresh() {
    startLoading(async () => setAvailability(await getAvailability(duration)));
  }

  function submit(formData: FormData) {
    startBooking(async () => {
      const state = await bookMeetingAction({}, formData);
      setResult(state);
      if (state.ok) {
        setSelected(null);
        setTopic("");
        setContact("");
      }
      // A lost race is the one error where the list on screen is the
      // problem, so it is refetched rather than left to be clicked again.
      if (state.stale) {
        setSelected(null);
        refresh();
      }
    });
  }

  if (!options.available) {
    return (
      <div className="mt-10 rounded-md border border-line bg-paper-2/60 p-6">
        <p className="text-sm text-ink-2">
          {options.unavailableReason || "Booking is unavailable right now."}
        </p>
        <p className="mt-3 text-sm text-ink-3">
          The contact form reaches Roger in the meantime.{" "}
          <a
            href="/contact"
            className="underline decoration-line decoration-1 underline-offset-4 hover:text-accent hover:decoration-accent"
          >
            Send a message
          </a>
        </p>
      </div>
    );
  }

  if (result.ok && result.meeting) {
    const m = result.meeting;
    return (
      <div className="mt-10 rounded-md border border-accent/40 bg-paper-2/60 p-6">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">booked</p>
        <p className="font-display mt-3 text-2xl text-ink">
          {dayLabel(m.start, m.zone)}, {timeLabel(m.start, m.zone)}
        </p>
        <p className="mt-2 text-sm text-ink-2">
          {m.durationMinutes} minutes, {options.zoneLabel}. It is on Roger&rsquo;s calendar.
        </p>
        <div className="mt-5 flex flex-wrap items-center gap-x-5 gap-y-3">
          <a
            href={m.icsUrl}
            className="inline-flex items-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white no-underline transition-colors hover:bg-accent-hover"
          >
            Add to your calendar
          </a>
          <button
            type="button"
            onClick={() => {
              setResult({});
              refresh();
            }}
            className="text-sm text-ink-2 underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
          >
            Book another
          </button>
        </div>
      </div>
    );
  }

  return (
    <div className="mt-10">
      <fieldset>
        <legend className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          how long do you need
        </legend>
        <div className="mt-3 flex flex-wrap gap-2">
          {durations.map((d) => (
            <button
              key={d}
              type="button"
              aria-pressed={d === duration}
              onClick={() => {
                // A start time that fits 15 minutes may not fit 45, so
                // a selection cannot survive the length changing.
                setSelected(null);
                setDuration(d);
              }}
              className={
                d === duration
                  ? "rounded-md bg-accent px-4 py-2 text-sm font-medium text-white transition-colors"
                  : "rounded-md border border-line px-4 py-2 text-sm text-ink-2 transition-colors hover:border-accent hover:text-accent"
              }
            >
              {d} minutes
            </button>
          ))}
        </div>
      </fieldset>

      <p className="mt-6 text-sm text-ink-3">
        All times {options.zoneLabel}
        {options.hoursSummary ? `, ${options.hoursSummary}` : ""}.
      </p>

      {loading ? (
        <p className="mt-8 text-sm text-ink-3">Checking the calendar...</p>
      ) : availability && !availability.available ? (
        <p className="mt-8 text-sm text-ink-2">
          {availability.unavailableReason || "Availability could not be loaded."}
        </p>
      ) : days.length === 0 ? (
        <p className="mt-8 text-sm text-ink-2">
          Nothing is free in the next {options.horizonDays} days for a {duration} minute meeting.
          Try a shorter one, or{" "}
          <a
            href="/contact"
            className="underline decoration-line decoration-1 underline-offset-4 hover:text-accent hover:decoration-accent"
          >
            send a message
          </a>
          .
        </p>
      ) : (
        <div className="mt-8 space-y-8">
          {days.map((day) => (
            <div key={day.label}>
              <h2 className="text-sm font-medium text-ink">{day.label}</h2>
              <div className="mt-3 flex flex-wrap gap-2">
                {day.slots.map((slot) => {
                  const active = selected?.start === slot.start;
                  return (
                    <button
                      key={slot.start}
                      type="button"
                      aria-pressed={active}
                      onClick={() => setSelected(active ? null : slot)}
                      className={
                        active
                          ? "rounded-md bg-accent px-4 py-2 text-sm font-medium text-white transition-colors"
                          : "rounded-md border border-line px-4 py-2 text-sm text-ink-2 transition-colors hover:border-accent hover:text-accent"
                      }
                    >
                      {timeLabel(slot.start, zone)}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>
      )}

      {selected ? (
        <form action={submit} className="mt-10 rounded-md border border-line bg-paper-2/60 p-6">
          <input type="hidden" name="start" value={selected.start} />
          <input type="hidden" name="duration_minutes" value={duration} />
          <p className="font-display text-xl text-ink">
            {dayLabel(selected.start, zone)}, {timeLabel(selected.start, zone)}
          </p>
          <p className="mt-1 text-sm text-ink-3">
            {duration} minutes, {options.zoneLabel}.
          </p>

          <label htmlFor="topic" className="mt-6 block text-sm text-ink">
            What would you like to discuss?
          </label>
          <textarea
            id="topic"
            name="topic"
            rows={3}
            required
            minLength={10}
            maxLength={500}
            value={topic}
            onChange={(e) => setTopic(e.target.value)}
            className="mt-2 w-full rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            placeholder="A role you are hiring for, a plant problem, or the reviewer's output on a posting."
          />

          <label htmlFor="contact_preference" className="mt-4 block text-sm text-ink">
            How should he reach you? <span className="text-ink-3">Optional</span>
          </label>
          <input
            id="contact_preference"
            name="contact_preference"
            type="text"
            maxLength={200}
            value={contact}
            onChange={(e) => setContact(e.target.value)}
            className="mt-2 w-full rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            placeholder="A phone number, or leave it and he will send a link."
          />

          {result.error ? (
            <p role="alert" className="mt-4 text-sm text-signal">
              {result.error}
            </p>
          ) : null}

          <div className="mt-6 flex flex-wrap items-center gap-x-5 gap-y-3">
            <button
              type="submit"
              disabled={booking}
              className="inline-flex items-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-60"
            >
              {booking ? "Booking..." : `Book ${duration} minutes`}
            </button>
            <button
              type="button"
              onClick={() => setSelected(null)}
              className="text-sm text-ink-3 transition-colors hover:text-accent"
            >
              Pick another time
            </button>
          </div>
        </form>
      ) : result.error ? (
        <p role="alert" className="mt-6 text-sm text-signal">
          {result.error}
        </p>
      ) : null}

      <MyMeetings initial={initialMeetings} zoneLabel={options.zoneLabel} />
    </div>
  );
}

function MyMeetings({ initial, zoneLabel }: { initial: Meeting[]; zoneLabel: string }) {
  const upcoming = initial.filter((m) => !m.cancelledAt);
  if (upcoming.length === 0) return null;
  return (
    <section className="mt-16 border-t border-line pt-8">
      <h2 className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        your meetings
      </h2>
      <ul className="mt-4 space-y-3">
        {upcoming.map((m) => (
          <li key={m.id} className="flex flex-wrap items-baseline gap-x-4 gap-y-1">
            <span className="text-sm text-ink">
              {dayLabel(m.start, m.zone)}, {timeLabel(m.start, m.zone)}
            </span>
            <span className="text-sm text-ink-3">
              {m.durationMinutes} minutes, {zoneLabel}
            </span>
            <a
              href={m.icsUrl}
              className="text-sm text-ink-2 underline decoration-line decoration-1 underline-offset-4 transition-colors hover:text-accent hover:decoration-accent"
            >
              Calendar file
            </a>
          </li>
        ))}
      </ul>
    </section>
  );
}
