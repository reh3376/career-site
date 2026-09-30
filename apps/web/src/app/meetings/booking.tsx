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
// visitor's, and the zone is stated beside the times. Converting would
// be friendlier right up to the moment somebody travels, and a meeting
// an hour out is worse than a label to read.
function fmt(iso: string, zone: string, opts: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat("en-US", { ...opts, timeZone: zone || "UTC" }).format(
    new Date(iso),
  );
}

type Day = {
  key: string;
  weekday: string;
  dayNum: string;
  month: string;
  long: string;
  slots: Slot[];
};

// The first version of this page rendered every slot at once: 139
// buttons over 21 days in one scroll, with no way to reach a
// particular day. Availability is now read a day at a time, which is
// the shape people already know from every other booking flow and
// turns a wall of times into about fifteen.
function groupByDay(slots: Slot[], zone: string): Day[] {
  const out = new Map<string, Day>();
  for (const slot of slots) {
    const key = fmt(slot.start, zone, { year: "numeric", month: "2-digit", day: "2-digit" });
    if (!out.has(key)) {
      out.set(key, {
        key,
        weekday: fmt(slot.start, zone, { weekday: "short" }),
        dayNum: fmt(slot.start, zone, { day: "numeric" }),
        month: fmt(slot.start, zone, { month: "short" }),
        long: fmt(slot.start, zone, { weekday: "long", month: "long", day: "numeric" }),
        slots: [],
      });
    }
    out.get(key)!.slots.push(slot);
  }
  return [...out.values()];
}

export function Booking({
  options,
  initialMeetings,
}: {
  options: MeetingOptions;
  initialMeetings: Meeting[];
}) {
  const durations = options.durationMinutes;
  const [duration, setDuration] = useState(durations[1] ?? durations[0] ?? 30);
  const [availability, setAvailability] = useState<AvailabilityState | null>(null);
  const [dayKey, setDayKey] = useState<string>("");
  const [selected, setSelected] = useState<Slot | null>(null);
  const [topic, setTopic] = useState("");
  const [contact, setContact] = useState("");
  const [result, setResult] = useState<BookState>({});
  const [loading, startLoading] = useTransition();
  const [booking, startBooking] = useTransition();

  // Refetched per length, because a 45-minute meeting has fewer places
  // to go than a 15-minute one and cannot be filtered out of one list.
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
  const days = useMemo(
    () => groupByDay(availability?.slots ?? [], zone),
    [availability, zone],
  );

  // Derived rather than synced: if the chosen day is not in the current
  // list, because the length changed or the calendar moved underneath,
  // fall back to the first available day instead of keeping a stale
  // selection or needing an effect to clear it.
  const day = days.find((d) => d.key === dayKey) ?? days[0] ?? null;

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
      // Losing the race is the one error where the list on screen is
      // itself the problem, so it is refetched rather than left to be
      // clicked again.
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
      <div className="mt-10 rounded-md border border-accent/40 bg-paper-2/60 p-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">booked</p>
        <p
          className="font-display mt-4 text-3xl leading-tight text-ink"
          style={{ fontVariationSettings: '"opsz" 100, "SOFT" 40' }}
        >
          {fmt(m.start, m.zone, { weekday: "long", month: "long", day: "numeric" })}
        </p>
        <p className="mt-1 text-lg text-ink-2">
          {fmt(m.start, m.zone, { hour: "numeric", minute: "2-digit" })}, {m.durationMinutes}{" "}
          minutes, {options.zoneLabel}
        </p>
        <p className="mt-4 text-sm text-ink-3">
          It is on Roger&rsquo;s calendar and you should have an invitation by email.
        </p>
        <div className="mt-6 flex flex-wrap items-center gap-x-5 gap-y-3">
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
    <div className="mt-12">
      {/* 1. Length */}
      <fieldset>
        <legend className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          how long do you need
        </legend>
        <div className="mt-3 inline-flex rounded-md border border-line p-1">
          {durations.map((d) => (
            <button
              key={d}
              type="button"
              aria-pressed={d === duration}
              onClick={() => {
                // A start that fits 15 minutes may not fit 45, so a
                // selection cannot survive the length changing.
                setSelected(null);
                setDuration(d);
              }}
              className={
                d === duration
                  ? "rounded px-5 py-2 text-sm font-medium text-white bg-accent transition-colors"
                  : "rounded px-5 py-2 text-sm text-ink-2 transition-colors hover:text-accent"
              }
            >
              {d} min
            </button>
          ))}
        </div>
      </fieldset>

      {loading && !availability ? (
        <p className="mt-10 text-sm text-ink-3">Checking Roger&rsquo;s calendar...</p>
      ) : availability && !availability.available ? (
        <p className="mt-10 text-sm text-ink-2">
          {availability.unavailableReason || "Availability could not be loaded."}
        </p>
      ) : days.length === 0 ? (
        <p className="mt-10 text-sm text-ink-2">
          Nothing is free in the next {options.horizonDays} days for a {duration} minute
          meeting. Try a shorter one, or{" "}
          <a
            href="/contact"
            className="underline decoration-line decoration-1 underline-offset-4 hover:text-accent hover:decoration-accent"
          >
            send a message
          </a>
          .
        </p>
      ) : (
        <>
          {/* 2. Day. Only days he is actually free appear, so the
              choice is small and every option leads somewhere. */}
          <div className="mt-10">
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              pick a day
            </p>
            <div className="mt-3 flex gap-2 overflow-x-auto pb-2">
              {days.map((d) => {
                const active = d.key === day?.key;
                return (
                  <button
                    key={d.key}
                    type="button"
                    aria-pressed={active}
                    onClick={() => {
                      setDayKey(d.key);
                      setSelected(null);
                    }}
                    className={
                      "shrink-0 rounded-md border px-4 py-3 text-center transition-colors " +
                      (active
                        ? "border-accent bg-accent text-white"
                        : "border-line bg-paper-2 text-ink-2 hover:border-accent hover:text-accent")
                    }
                  >
                    <span className="block font-mono text-[10px] uppercase tracking-[0.14em] opacity-80">
                      {d.weekday}
                    </span>
                    <span className="mt-1 block text-lg font-medium leading-none">{d.dayNum}</span>
                    <span className="mt-1 block font-mono text-[10px] uppercase tracking-[0.1em] opacity-80">
                      {d.month}
                    </span>
                  </button>
                );
              })}
            </div>
          </div>

          {/* 3. Time, for that day only. */}
          {day ? (
            <div className="mt-10">
              <div className="flex flex-wrap items-baseline justify-between gap-2">
                <h2
                  className="font-display text-2xl text-ink"
                  style={{ fontVariationSettings: '"opsz" 72, "SOFT" 40' }}
                >
                  {day.long}
                </h2>
                <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  all times {options.zoneLabel}
                </p>
              </div>
              <div className="mt-4 grid grid-cols-2 gap-2 sm:grid-cols-4">
                {day.slots.map((slot) => {
                  const active = selected?.start === slot.start;
                  return (
                    <button
                      key={slot.start}
                      type="button"
                      aria-pressed={active}
                      onClick={() => setSelected(active ? null : slot)}
                      className={
                        "rounded-md border px-3 py-3 text-sm transition-colors " +
                        (active
                          ? "border-accent bg-accent font-medium text-white"
                          : "border-line bg-paper text-ink-2 hover:border-accent hover:text-accent")
                      }
                    >
                      {fmt(slot.start, zone, { hour: "numeric", minute: "2-digit" })}
                    </button>
                  );
                })}
              </div>
            </div>
          ) : null}
        </>
      )}

      {/* 4. Confirm. */}
      {selected ? (
        <form action={submit} className="mt-10 rounded-md border border-accent/40 bg-paper-2/60 p-6">
          <input type="hidden" name="start" value={selected.start} />
          <input type="hidden" name="duration_minutes" value={duration} />
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
            you are booking
          </p>
          <p
            className="font-display mt-2 text-2xl text-ink"
            style={{ fontVariationSettings: '"opsz" 72, "SOFT" 40' }}
          >
            {fmt(selected.start, zone, { weekday: "long", month: "long", day: "numeric" })},{" "}
            {fmt(selected.start, zone, { hour: "numeric", minute: "2-digit" })}
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
              {fmt(m.start, m.zone, { weekday: "long", month: "long", day: "numeric" })},{" "}
              {fmt(m.start, m.zone, { hour: "numeric", minute: "2-digit" })}
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
