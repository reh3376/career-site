"use client";

import { useEffect, useRef } from "react";

// The per-question countdown, shown to half of all runs.
//
// Why it is isolated in its own component
// ---------------------------------------
// It ticks about sixty times a second. Held as state in the run screen
// that would re-render the question, the options and the confidence
// buttons on every frame, and the thing being measured is how long
// somebody takes to answer a question. An instrument that repaints the
// question sixty times a second is not measuring the same question.
//
// So the value never enters React state. The component mounts once per
// question and writes into its own DOM node from a rAF loop; nothing
// above it re-renders.
//
// Why milliseconds
// ----------------
// The owner asked for seconds and milliseconds. The design reason is
// the point of the experiment: a counter that moves visibly is a
// counter that competes for attention, which is the mechanism under
// test. A quiet whole-second tick would be a weaker version of the
// same manipulation and would make a null result uninterpretable.

export function Countdown({
  deadline,
  totalMs,
}: {
  /** performance.now() value the question expires at. */
  deadline: number;
  totalMs: number;
}) {
  const out = useRef<HTMLSpanElement>(null);
  const bar = useRef<HTMLSpanElement>(null);

  useEffect(() => {
    let raf = 0;
    const paint = () => {
      const left = Math.max(0, deadline - performance.now());
      const node = out.current;
      if (node) {
        const s = Math.floor(left / 1000);
        const ms = Math.floor(left % 1000);
        node.textContent = `${s}.${String(ms).padStart(3, "0")}`;
      }
      const b = bar.current;
      if (b) b.style.transform = `scaleX(${totalMs ? left / totalMs : 0})`;
      if (left > 0) raf = requestAnimationFrame(paint);
    };
    paint();
    return () => cancelAnimationFrame(raf);
  }, [deadline, totalMs]);

  return (
    <div
      className="pointer-events-none select-none"
      // Read by nobody: a screen reader announcing a number sixty times
      // a second is unusable, and the countdown is a visual condition.
      // The question's own time limit is stated in the briefing.
      aria-hidden="true"
    >
      <span
        ref={out}
        className="font-display block text-center text-2xl tabular-nums text-ink-3"
      >
        0.000
      </span>
      <span className="mt-1 block h-[2px] w-full overflow-hidden bg-line">
        <span
          ref={bar}
          className="block h-full w-full origin-left bg-accent"
          style={{ transform: "scaleX(1)" }}
        />
      </span>
    </div>
  );
}
