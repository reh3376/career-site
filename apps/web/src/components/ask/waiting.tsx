"use client";

import { useEffect, useState } from "react";

// The wait, told honestly.
//
// On the box that answers these questions, the first word arrives 10 to
// 25 seconds after the question is sent. That is measured, not
// pessimistic: prompt evaluation runs at roughly 32 tokens a second on
// a CPU with no GPU, and every retrieved passage is about five seconds
// of it.
//
// A spinner is the wrong instrument for a wait that long. A spinner
// says "a moment", and after eight seconds of one a reader concludes
// the page is broken and leaves. So this says what is happening, says
// roughly how long it takes before the clock starts, and then shows the
// clock. Someone who knows a fifteen second answer is coming will wait
// for it; someone watching an unlabelled spinner will not.
//
// The visual language is the site's own. Safety orange is reserved
// across this design system for live-status indicators and nothing
// else, which is exactly what this is, and a mono readout with a
// running count is the control-room idiom the rest of the site speaks.
//
// The count is deliberately not a progress bar. There is no progress to
// report: the server cannot say how far through generation it is, and a
// bar that fills on a timer is a spinner that lies with more confidence.

type Props = {
  /** Shown before the elapsed clock. */
  label?: string;
  /** Seconds after which the wait is acknowledged as unusual. */
  slowAfter?: number;
};

export function Waiting({
  label = "Reading his records",
  slowAfter = 30,
}: Props) {
  const [seconds, setSeconds] = useState(0);

  useEffect(() => {
    const started = Date.now();
    const id = setInterval(
      () => setSeconds(Math.floor((Date.now() - started) / 1000)),
      1000,
    );
    return () => clearInterval(id);
  }, []);

  const slow = seconds >= slowAfter;

  return (
    <div
      className="flex items-baseline gap-3 py-3"
      role="status"
      aria-live="polite"
    >
      <span
        aria-hidden="true"
        className={`mt-1 inline-block size-2 shrink-0 rounded-full ${
          slow ? "bg-warning" : "animate-pulse bg-signal"
        }`}
      />
      <p className="font-mono text-[11px] tracking-[0.12em] text-ink-2 uppercase">
        {slow ? "Taking longer than usual" : label}
        <span className="ml-3 text-ink-3 tabular-nums">{seconds}s</span>
      </p>
    </div>
  );
}

// WaitingNote sets the expectation before the clock starts, so the
// first long wait is not a surprise. Shown once per conversation rather
// than on every question: a reader who has waited once already knows.
export function WaitingNote() {
  return (
    <p className="mt-2 text-sm leading-relaxed text-ink-3">
      Answers take ten to twenty seconds. Everything runs on one small
      server, and it reads his actual records before it says anything.
    </p>
  );
}
