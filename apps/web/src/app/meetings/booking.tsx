"use client";

import { useEffect, useMemo, useRef, useState, useTransition } from "react";

import { track } from "@/lib/events-client";

import {
  bookMeetingAction,
  getAvailability,
  listMyMeetings,
  type AvailabilityState,
  type BookState,
  type Meeting,
  type MeetingOptions,
  type Slot,
} from "./actions";
import { COUNTRY_CODES, DEFAULT_COUNTRY, numberHint } from "./countries";
import { VIDEO_PROVIDERS } from "./providers";
import { MyMeetings } from "./my-meetings";

// Every time on this page is rendered in the owner's zone, never the
// visitor's, and the zone is stated beside the times. Converting would
// be friendlier right up to the moment somebody travels, and a meeting
// an hour out is worse than a label to read.
function fmt(iso: string, zone: string, opts: Intl.DateTimeFormatOptions) {
  return new Intl.DateTimeFormat("en-US", { ...opts, timeZone: zone || "UTC" }).format(
    new Date(iso),
  );
}

// "Time displayed in Eastern Time Zone (UTC -4)". The offset is read
// from the zone at the date being shown, not from today, because the
// horizon crosses a daylight-saving boundary twice a year and an
// offset stated from today would be an hour wrong for half the list.
function zoneLine(zone: string, label: string, on: Date) {
  const part = new Intl.DateTimeFormat("en-US", {
    timeZone: zone || "UTC",
    timeZoneName: "longOffset",
  })
    .formatToParts(on)
    .find((x) => x.type === "timeZoneName")?.value;
  // "GMT-04:00" to "-4"; "GMT" itself means no offset.
  const m = part?.match(/GMT([+-])(\d{2}):(\d{2})/);
  // The server's label is "Eastern time"; appending " Zone" to that
  // gives "Eastern time Zone". Capitalise the word so it reads as the
  // proper noun it is.
  const proper = label.replace(/\btime\b/, "Time");
  if (!m) return `Time displayed in ${proper}`;
  const hours = Number(m[2]);
  const mins = Number(m[3]);
  const off = mins === 0 ? `${m[1]}${hours}` : `${m[1]}${hours}:${m[3]}`;
  return `Time displayed in ${proper} Zone (UTC ${off})`;
}

// How many days the picker shows at once, with arrows either side.
const DAYS_PER_PAGE = 6;

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
  const [page, setPage] = useState(0);
  const [selected, setSelected] = useState<Slot | null>(null);
  const [topic, setTopic] = useState("");
  const [contact, setContact] = useState("");
  const [meetingType, setMeetingType] = useState<"video" | "phone" | "">("");
  const [provider, setProvider] = useState("");
  const [country, setCountry] = useState(DEFAULT_COUNTRY);
  const [phone, setPhone] = useState("");
  const [phoneInComments, setPhoneInComments] = useState(false);
  const [result, setResult] = useState<BookState>({});
  // Seeded from the server render, then kept current here: booking adds
  // one and cancelling removes one, and a list that only reloads with
  // the page is wrong the moment either happens.
  const [meetings, setMeetings] = useState<Meeting[]>(initialMeetings);
  // Set by the effect below; called from the pickers so the abandon
  // event knows how far they got.
  const stepRef = useRef<(s: string) => void>(() => {});

  // The funnel in front of a booking. Three bookings tell you nothing
  // about the people who looked and left, and that is the group a
  // scheduler is actually tuned for. step is how far they got, so an
  // exit at "picked a time, never confirmed" reads differently from
  // one at "saw the page".
  useEffect(() => {
    track("meeting.view", {});
    const openedAt = Date.now();
    let step = "viewed";
    stepRef.current = (s: string) => {
      step = s;
    };
    const onHide = () => {
      if (step === "booked") return;
      track(
        "meeting.abandoned",
        { step, elapsed_ms: Date.now() - openedAt },
        { flush: true },
      );
    };
    window.addEventListener("pagehide", onHide);
    return () => window.removeEventListener("pagehide", onHide);
  }, []);
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

  // Six at a time, as a sliding window rather than fixed pages.
  //
  // Paging in blocks meant the last one held whatever was left: with
  // nine days the second page showed three, and with eight it showed
  // two, which is not "the next six days" by any reading. Clamping the
  // start instead keeps six on screen whenever six exist, so the row
  // never collapses to a stub.
  const maxStart = Math.max(0, days.length - DAYS_PER_PAGE);
  const start = Math.min(page * DAYS_PER_PAGE, maxStart);
  const shown = days.slice(start, start + DAYS_PER_PAGE);

  function refresh() {
    startLoading(async () => setAvailability(await getAvailability(duration)));
  }

  // Both lists move together. A booking removes times and adds a
  // meeting; a cancellation does the reverse, and showing one without
  // the other leaves the page contradicting itself.
  function refreshAll() {
    startLoading(async () => {
      const [next, mine] = await Promise.all([getAvailability(duration), listMyMeetings()]);
      setAvailability(next);
      setMeetings(mine);
    });
  }

  function submit(formData: FormData) {
    startBooking(async () => {
      const state = await bookMeetingAction({}, formData);
      setResult(state);
      if (state.ok) {
        // The api writes meeting.booked; this only stops the exit from
        // being counted as an abandonment of a booking that succeeded.
        stepRef.current("booked");
        setSelected(null);
        setTopic("");
        setContact("");
        setMeetingType("");
        setProvider("");
        setCountry(DEFAULT_COUNTRY);
        setPhone("");
        setPhoneInComments(false);
        refreshAll();
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
    const setup = VIDEO_PROVIDERS.find((v) => v.value === m.videoProvider);
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
        {/* The site creates no video rooms, so the member is sent to
            their own calendar to make one. A new tab rather than a
            redirect: losing this page would lose the calendar file. */}
        {setup ? (
          <p className="mt-4 rounded-md border border-line bg-paper p-4 text-sm text-ink-2">
            You said you would host on <span className="text-ink">{setup.label}</span>. Create the
            room in your calendar and send Roger the link.
          </p>
        ) : null}
        <div className="mt-6 flex flex-wrap items-center gap-x-5 gap-y-3">
          {setup ? (
            <a
              href={setup.setupUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex items-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white no-underline transition-colors hover:bg-accent-hover"
            >
              Set up the {setup.label} link
            </a>
          ) : null}
          <a
            href={m.icsUrl}
            className={
              setup
                ? "inline-flex items-center rounded-md border border-line bg-paper-2 px-5 py-2.5 text-sm font-medium text-ink no-underline transition-colors hover:border-accent hover:text-accent"
                : "inline-flex items-center rounded-md bg-accent px-5 py-2.5 text-sm font-medium text-white no-underline transition-colors hover:bg-accent-hover"
            }
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
                setPage(0);
                setDuration(d);
                track("meeting.duration_picked", { minutes: d });
                stepRef.current("duration_picked");
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
            <div className="flex flex-wrap items-baseline justify-between gap-2">
              <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                pick a day
              </p>
              {options.hoursSummary ? (
                <p className="text-sm text-ink-3">
                  Roger takes meetings {options.hoursSummary}.
                </p>
              ) : null}
            </div>
            <div className="mt-3 flex items-stretch gap-2 overflow-x-auto pb-1">
              {/* Back only when there is something behind, so the row
                  does not carry a control that does nothing. */}
              {start > 0 ? (
                <button
                  type="button"
                  onClick={() => setPage(Math.max(0, Math.ceil(start / DAYS_PER_PAGE) - 1))}
                  aria-label="Earlier days"
                  className="shrink-0 rounded-md border border-line px-3 text-ink-2 transition-colors hover:border-accent hover:text-accent"
                >
                  &#8249;
                </button>
              ) : null}

              {shown.map((d) => {
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
                      "w-[4.75rem] shrink-0 rounded-md border px-2 py-3 text-center transition-colors " +
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
                    <span className="mt-1 block text-[10px] opacity-70">
                      {d.slots.length} {d.slots.length === 1 ? "time" : "times"}
                    </span>
                  </button>
                );
              })}

              {start < maxStart ? (
                <button
                  type="button"
                  onClick={() => setPage(Math.floor(start / DAYS_PER_PAGE) + 1)}
                  aria-label="Later days"
                  className="shrink-0 rounded-md border border-line px-3 text-ink-2 transition-colors hover:border-accent hover:text-accent"
                >
                  &#8250;
                </button>
              ) : null}
            </div>
            {days.length > DAYS_PER_PAGE ? (
              <p className="mt-2 font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
                days {start + 1} to {start + shown.length} of {days.length}
              </p>
            ) : null}
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
                <p className="text-sm text-ink-3">
                  {zoneLine(zone, options.zoneLabel, new Date(day.slots[0].start))}
                </p>
              </div>
              <div
                className={
                  "mt-4 grid grid-cols-2 gap-2 transition-opacity sm:grid-cols-4 " +
                  (loading ? "opacity-50" : "")
                }
                aria-busy={loading}
              >
                {day.slots.map((slot) => {
                  const active = selected?.start === slot.start;
                  return (
                    <button
                      key={slot.start}
                      type="button"
                      aria-pressed={active}
                      onClick={() => {
                        setSelected(active ? null : slot);
                        if (!active) {
                          track("meeting.slot_picked", {
                            minutes: duration,
                            lead_days: Math.round(
                              (new Date(slot.start).getTime() - Date.now()) /
                                86400000,
                            ),
                          });
                          stepRef.current("slot_picked");
                        }
                      }}
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

          {/* How the meeting happens. Asked here because a time with
              no way to reach the other person is a second exchange of
              emails to arrange the thing this was supposed to have
              arranged. No video room is created by this site: you host
              it and send the link. */}
          <input type="hidden" name="meeting_type" value={
            meetingType === "video" ? "MEETING_TYPE_VIDEO"
            : meetingType === "phone" ? "MEETING_TYPE_PHONE" : ""
          } />
          <input type="hidden" name="video_provider" value={meetingType === "video" ? provider : ""} />
          <input
            type="hidden"
            name="phone_number"
            value={
              meetingType === "phone" && !phoneInComments && phone.trim()
                ? `${country} ${phone.replace(/[^0-9]/g, "")}`
                : ""
            }
          />

          <fieldset className="mt-6">
            <legend className="text-sm text-ink">How should you meet?</legend>
            <div className="mt-2 inline-flex rounded-md border border-line p-1">
              {(["video", "phone"] as const).map((t) => (
                <button
                  key={t}
                  type="button"
                  aria-pressed={meetingType === t}
                  onClick={() => {
                    setMeetingType(t);
                    // The two are exclusive, and carrying a stale
                    // provider into a phone call is refused by the
                    // server anyway.
                    setProvider("");
                  }}
                  className={
                    meetingType === t
                      ? "rounded px-4 py-2 text-sm font-medium text-white bg-accent transition-colors"
                      : "rounded px-4 py-2 text-sm text-ink-2 transition-colors hover:text-accent"
                  }
                >
                  {t === "video" ? "Video" : "Phone only"}
                </button>
              ))}
            </div>
          </fieldset>

          {meetingType === "video" ? (
            <div className="mt-4">
              <label htmlFor="provider" className="block text-sm text-ink">
                Which service will you host on?
              </label>
              <select
                id="provider"
                required
                value={provider}
                onChange={(e) => setProvider(e.target.value)}
                className="mt-2 w-full rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink outline-none focus:border-accent"
              >
                <option value="">Choose one</option>
                {VIDEO_PROVIDERS.map((v) => (
                  <option key={v.value} value={v.value}>
                    {v.label}
                  </option>
                ))}
              </select>
              <p className="mt-1 text-xs text-ink-3">
                You create the room and send the link. After booking, a new tab opens on your
                calendar so you can set it up straight away.
              </p>
            </div>
          ) : null}

          {meetingType === "phone" ? (
            <div className="mt-4">
              <label htmlFor="phone_display" className="block text-sm text-ink">
                What number will you call from?
              </label>
              {/* Country code and number are separate controls. Typing
                  a code into a free-text field is where the format
                  arguments come from, and +1 is the default because
                  that is what almost every caller needs. */}
              <div className="mt-2 flex gap-2">
                <select
                  aria-label="Country code"
                  disabled={phoneInComments}
                  value={country}
                  onChange={(e) => setCountry(e.target.value)}
                  className="w-44 shrink-0 rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink outline-none focus:border-accent disabled:opacity-50"
                >
                  {COUNTRY_CODES.map((c) => (
                    <option key={c.code} value={c.code}>
                      {c.label}
                    </option>
                  ))}
                </select>
                <input
                  id="phone_display"
                  type="tel"
                  inputMode="tel"
                  autoComplete="tel-national"
                  disabled={phoneInComments}
                  value={phone}
                  onChange={(e) => setPhone(e.target.value)}
                  className="w-full rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink outline-none focus:border-accent disabled:opacity-50"
                  placeholder={country === "+1" ? "5135551234" : "Your number"}
                />
              </div>
              <p className="mt-1 text-xs text-ink-3">{numberHint(country)}</p>
              <label className="mt-2 flex items-center gap-2 text-sm text-ink-2">
                <input
                  type="checkbox"
                  checked={phoneInComments}
                  onChange={(e) => setPhoneInComments(e.target.checked)}
                />
                My number is in the comments above
              </label>
            </div>
          ) : null}

          <label htmlFor="contact_preference" className="mt-4 block text-sm text-ink">
            Anything else he should know? <span className="text-ink-3">Optional</span>
          </label>
          <input
            id="contact_preference"
            name="contact_preference"
            type="text"
            maxLength={200}
            value={contact}
            onChange={(e) => setContact(e.target.value)}
            className="mt-2 w-full rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink outline-none focus:border-accent"
            placeholder="An extension, a preferred half of the call, anything useful."
          />

          {result.error ? (
            <p role="alert" className="mt-4 text-sm text-signal">
              {result.error}
            </p>
          ) : null}

          <div className="mt-6 flex flex-wrap items-center gap-x-5 gap-y-3">
            <button
              type="submit"
              disabled={booking || !meetingType || (meetingType === "video" && !provider)}
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

      <MyMeetings
        meetings={meetings}
        zoneLabel={options.zoneLabel}
        onChanged={refreshAll}
      />
    </div>
  );
}
