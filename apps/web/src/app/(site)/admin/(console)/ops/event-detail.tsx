"use client";

import { useEffect, useRef, useState } from "react";

import { getEventDetailAction, type EventDetail } from "./actions";

// A timeline event opens the record it is about.
//
// The timeline answers "how fast is this going". It cannot answer "and
// how did that posting do", which is the next question every time, and
// until now meant leaving /admin/ops, finding the run on /admin/evals
// and matching rows up by name.
//
// This is a second dialog over the first rather than a replacement for
// it, because the timeline is the context: the reader is comparing one
// posting against the pace of the others, and closing the list to see
// one row loses the thing being compared against.

function score(v: number | undefined): string {
  return v === undefined || v === null ? "-" : Number(v).toFixed(4);
}

function Field({
  label,
  value,
  tone = "text-ink",
}: {
  label: string;
  value: string;
  tone?: string;
}) {
  return (
    <div>
      <dt className="text-ink-4">{label}</dt>
      <dd className={`${tone} break-words`}>{value}</dd>
    </div>
  );
}

export function EventDetailDialog({
  eventRef,
  label,
  timing,
  onClose,
}: {
  eventRef: string;
  label: string;
  timing: { at: string; gap: string; took: string; progress?: number };
  onClose: () => void;
}) {
  const [detail, setDetail] = useState<EventDetail | null>(null);
  const panelRef = useRef<HTMLDivElement>(null);
  const returnFocusTo = useRef<Element | null>(null);

  useEffect(() => {
    returnFocusTo.current = document.activeElement;
    let stop = false;
    void (async () => {
      const d = await getEventDetailAction(eventRef);
      if (!stop) setDetail(d);
    })();
    return () => {
      stop = true;
      // Back to the timeline row that was clicked, not to the top of
      // the dialog underneath, or the reader loses their place in a
      // list they opened this from.
      if (returnFocusTo.current instanceof HTMLElement) {
        returnFocusTo.current.focus();
      }
    };
  }, [eventRef]);

  useEffect(() => {
    panelRef.current?.focus();
    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape") return;
      // Close this one only. The dialog underneath listens for Escape
      // too and would otherwise close with it, which reads as the key
      // having done twice what was asked.
      e.stopPropagation();
      onClose();
    }
    // Capture, so this runs before the parent's listener on the same
    // target rather than after it.
    document.addEventListener("keydown", onKey, true);
    return () => document.removeEventListener("keydown", onKey, true);
  }, [onClose]);

  const t = detail?.ok ? detail.target : null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="event-detail-title"
      className="fixed inset-0 z-[60] flex items-end justify-center bg-ink/70 p-4 backdrop-blur-sm sm:items-center"
      onClick={(e) => {
        // Stop the parent dialog seeing this click as a backdrop click
        // on itself, which would close both.
        e.stopPropagation();
        if (e.target === e.currentTarget) onClose();
      }}
    >
      <div
        ref={panelRef}
        tabIndex={-1}
        className="max-h-[85vh] w-full max-w-lg overflow-y-auto border border-line-strong bg-canvas p-6 shadow-2xl ring-1 ring-line-strong outline-none"
      >
        <div className="flex items-start justify-between gap-4">
          <div>
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              report <span className="text-ink-4">·</span> {timing.at}
            </p>
            <h3
              id="event-detail-title"
              className="mt-1 text-sm leading-relaxed text-ink"
            >
              {label}
            </h3>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="shrink-0 border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 hover:text-ink"
          >
            Back
          </button>
        </div>

        <dl className="mt-5 grid grid-cols-2 gap-x-6 gap-y-2 border-y border-line py-3 font-mono text-xs sm:grid-cols-4">
          <Field label="reported" value={timing.at} />
          {/* "since previous" is the step BEFORE this one, and saying
              so avoids the reading that cost a wrong conclusion on the
              timeline: a report is written when a step begins, so the
              gap behind it belongs to the step behind it. */}
          <Field label="previous step took" value={timing.gap || "first"} />
          <Field label="this step took" value={timing.took || "still running"} />
          <Field
            label="progress"
            value={timing.progress ? `${timing.progress}%` : "-"}
          />
        </dl>

        {detail === null ? (
          <p className="mt-5 text-sm text-ink-3">Loading...</p>
        ) : !detail.ok ? (
          <p className="mt-5 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
            {detail.error}
          </p>
        ) : t?.kind === "pending" ? (
          <p className="mt-5 border-l-2 border-ink-3 bg-paper-2 px-4 py-3 text-sm leading-relaxed text-ink-2">
            No result yet. The report is written when the posting starts
            scoring, not when it finishes, so this row exists before its
            result does. Reopen it once the next report appears.
          </p>
        ) : t?.kind === "eval-item" ? (
          <>
            <p className="mt-5 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              result
            </p>
            {t.error ? (
              <p className="mt-2 border-l-2 border-danger bg-paper-2 px-4 py-3 text-sm leading-relaxed text-ink">
                {t.error}
              </p>
            ) : (
              <>
                <dl className="mt-2 grid grid-cols-2 gap-x-6 gap-y-2 font-mono text-xs sm:grid-cols-4">
                  <Field label="score" value={score(t.matchScore)} />
                  <Field label="gate" value={score(t.threshold)} />
                  <Field label="expected" value={t.expectedGate || "-"} />
                  <Field
                    label="landed"
                    value={t.gateSide || "-"}
                    tone={t.passed ? "text-ink" : "text-danger"}
                  />
                </dl>
                <p className="mt-3 text-sm leading-relaxed text-ink-2">
                  {t.passed
                    ? `On the expected side of the gate, in the ${t.fit || "unbanded"} band.`
                    : `Expected ${t.expectedGate || "?"} the gate and landed ${t.gateSide || "?"}, in the ${t.fit || "unbanded"} band.`}
                </p>
              </>
            )}
            <div className="mt-5 flex flex-wrap gap-3">
              {t.submissionId ? (
                <a
                  href={`/admin/jd/${t.submissionId}`}
                  className="border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-accent no-underline hover:text-accent-hover"
                >
                  Open the derivation
                </a>
              ) : null}
              <a
                href={`/admin/evals/${t.runId}`}
                className="border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-accent no-underline hover:text-accent-hover"
              >
                Open run #{t.runId}
              </a>
            </div>
          </>
        ) : t?.kind === "eval-run" ? (
          <>
            <p className="mt-5 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              run #{t.runId} {t.runNote ? `· ${t.runNote}` : ""}
            </p>
            <dl className="mt-2 grid grid-cols-2 gap-x-6 gap-y-2 font-mono text-xs sm:grid-cols-4">
              <Field
                label="on the expected side"
                value={`${t.gateCorrect} of ${t.scored}`}
              />
              <Field
                label="inversions"
                value={String(t.orderViolations)}
                tone={t.orderViolations ? "text-danger" : "text-ink"}
              />
              <Field label="margin" value={score(t.margin)} />
              <Field
                label="errors"
                value={String(t.errors)}
                tone={t.errors ? "text-danger" : "text-ink"}
              />
            </dl>
            <p className="mt-3 text-sm leading-relaxed text-ink-2">
              The margin is the gap between the lowest score above the gate and
              the highest below it. It narrows before gate accuracy breaks,
              which is why it is read alongside the count.
            </p>
            <div className="mt-5">
              <a
                href={`/admin/evals/${t.runId}`}
                className="border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-accent no-underline hover:text-accent-hover"
              >
                Open run #{t.runId}
              </a>
            </div>
          </>
        ) : (
          <p className="mt-5 text-sm text-ink-3">
            Nothing more is recorded for this report.
          </p>
        )}
      </div>
    </div>
  );
}
