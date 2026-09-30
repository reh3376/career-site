"use client";

import { useActionState, useState } from "react";

import {
  saveSchedulerSettings,
  type SaveState,
  type SchedulerSettings,
  type Window,
} from "./actions";

const DAYS = ["Sunday", "Monday", "Tuesday", "Wednesday", "Thursday", "Friday", "Saturday"];

// Windows are stored as minutes from local midnight; the form edits
// them as HH:MM because that is how a person thinks about an hour.
function toClock(mins: number) {
  const h = Math.floor(mins / 60);
  const m = mins % 60;
  return `${String(h).padStart(2, "0")}:${String(m).padStart(2, "0")}`;
}

function toMinutes(clock: string) {
  const [h, m] = clock.split(":").map((n) => Number(n) || 0);
  return h * 60 + m;
}

function sortWindows(ws: Window[]) {
  return [...ws].sort((a, b) =>
    a.weekday === b.weekday ? a.startMinutes - b.startMinutes : a.weekday - b.weekday,
  );
}

export function WindowsForm({ initial }: { initial: SchedulerSettings }) {
  const [state, action, pending] = useActionState<SaveState, FormData>(saveSchedulerSettings, {});
  const [s, setS] = useState<SchedulerSettings>(initial);

  function patch(next: Partial<SchedulerSettings>) {
    setS((cur) => ({ ...cur, ...next }));
  }

  function patchWindow(i: number, next: Partial<Window>) {
    setS((cur) => {
      const windows = cur.windows.map((w, j) => (j === i ? { ...w, ...next } : w));
      return { ...cur, windows };
    });
  }

  function addWindow() {
    patch({
      windows: [...s.windows, { weekday: 2, startMinutes: 9 * 60, endMinutes: 12 * 60 }],
    });
  }

  function removeWindow(i: number) {
    patch({ windows: s.windows.filter((_, j) => j !== i) });
  }

  function toggleDuration(d: number) {
    const has = s.durationMinutes.includes(d);
    patch({
      durationMinutes: has
        ? s.durationMinutes.filter((x) => x !== d)
        : [...s.durationMinutes, d].sort((a, b) => a - b),
    });
  }

  const numbers: Array<{ key: keyof SchedulerSettings; label: string; hint: string }> = [
    { key: "gapMinutes", label: "Gap between meetings", hint: "Minutes, applied on both sides. Not held until something is booked." },
    { key: "stepMinutes", label: "Start times every", hint: "Minutes. How finely starts are offered." },
    { key: "maxPerDay", label: "Most meetings a day", hint: "Counts meetings taken, not slots shown." },
    { key: "leadHours", label: "Earliest booking", hint: "Hours ahead. Keeps the next hour unbookable." },
    { key: "horizonDays", label: "Calendar runs", hint: "Days ahead." },
  ];

  return (
    <form action={action} className="mt-8">
      {/* The whole object goes over at once because the server
          validates and stores it whole. */}
      <input type="hidden" name="settings" value={JSON.stringify({ ...s, windows: sortWindows(s.windows) })} />

      <fieldset>
        <legend className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          bookable windows
        </legend>
        <p className="mt-2 text-sm text-ink-3">
          Two windows on one day is how a lunch break is expressed. They may not overlap.
        </p>
        <div className="mt-4 space-y-3">
          {sortWindows(s.windows).map((w) => {
            const i = s.windows.indexOf(w);
            return (
              <div key={`${w.weekday}-${w.startMinutes}-${i}`} className="flex flex-wrap items-center gap-3">
                <select
                  aria-label="Weekday"
                  value={w.weekday}
                  onChange={(e) => patchWindow(i, { weekday: Number(e.target.value) })}
                  className="rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink"
                >
                  {DAYS.map((d, n) => (
                    <option key={d} value={n}>
                      {d}
                    </option>
                  ))}
                </select>
                <input
                  aria-label="Opens"
                  type="time"
                  step={900}
                  value={toClock(w.startMinutes)}
                  onChange={(e) => patchWindow(i, { startMinutes: toMinutes(e.target.value) })}
                  className="rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink"
                />
                <span className="text-sm text-ink-3">to</span>
                <input
                  aria-label="Closes"
                  type="time"
                  step={900}
                  value={toClock(w.endMinutes)}
                  onChange={(e) => patchWindow(i, { endMinutes: toMinutes(e.target.value) })}
                  className="rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink"
                />
                <button
                  type="button"
                  onClick={() => removeWindow(i)}
                  className="text-sm text-ink-3 transition-colors hover:text-signal"
                >
                  Remove
                </button>
              </div>
            );
          })}
        </div>
        <button
          type="button"
          onClick={addWindow}
          className="mt-4 rounded-md border border-line px-4 py-2 text-sm text-ink-2 transition-colors hover:border-accent hover:text-accent"
        >
          Add a window
        </button>
      </fieldset>

      <fieldset className="mt-10">
        <legend className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          meeting lengths
        </legend>
        <div className="mt-3 flex flex-wrap gap-2">
          {[15, 30, 45, 60].map((d) => (
            <button
              key={d}
              type="button"
              aria-pressed={s.durationMinutes.includes(d)}
              onClick={() => toggleDuration(d)}
              className={
                s.durationMinutes.includes(d)
                  ? "rounded-md bg-accent px-4 py-2 text-sm font-medium text-white"
                  : "rounded-md border border-line px-4 py-2 text-sm text-ink-2 transition-colors hover:border-accent hover:text-accent"
              }
            >
              {d} min
            </button>
          ))}
        </div>
      </fieldset>

      <fieldset className="mt-10">
        <legend className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          the rest
        </legend>
        <div className="mt-4 grid gap-5 sm:grid-cols-2">
          <div>
            <label htmlFor="zone" className="block text-sm text-ink">
              Time zone
            </label>
            <input
              id="zone"
              type="text"
              value={s.zone}
              onChange={(e) => patch({ zone: e.target.value })}
              className="mt-1 w-full rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink"
            />
            <p className="mt-1 text-xs text-ink-3">
              An IANA name such as America/New_York. Never an offset: an offset is right for
              half the year and an hour wrong for the other half.
            </p>
          </div>
          {numbers.map((n) => (
            <div key={n.key}>
              <label htmlFor={n.key} className="block text-sm text-ink">
                {n.label}
              </label>
              <input
                id={n.key}
                type="number"
                min={0}
                value={Number(s[n.key])}
                onChange={(e) => patch({ [n.key]: Number(e.target.value) } as Partial<SchedulerSettings>)}
                className="mt-1 w-full rounded-md border border-line bg-paper px-3 py-2 text-sm text-ink"
              />
              <p className="mt-1 text-xs text-ink-3">{n.hint}</p>
            </div>
          ))}
        </div>
      </fieldset>

      {state.error ? (
        <p role="alert" className="mt-6 text-sm text-signal">
          {state.error}
        </p>
      ) : null}
      {state.saved ? (
        <p role="status" className="mt-6 text-sm text-ink-2">
          Saved. New bookings follow these within seconds. Meetings already booked are not moved.
        </p>
      ) : null}

      <button
        type="submit"
        disabled={pending}
        className="mt-8 inline-flex items-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white transition-colors hover:bg-accent-hover disabled:opacity-60"
      >
        {pending ? "Saving..." : "Save availability"}
      </button>
    </form>
  );
}
