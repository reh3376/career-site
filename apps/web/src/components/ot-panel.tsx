import type { ReactNode } from "react";

// OT-mode panel wrapper. Renders a plant-HMI-style tag/title band
// above the child content. Callers pass the tag (e.g. "MSG-01"), the
// title (e.g. "MESSAGE OUT · CONTACT"), and an optional right-side
// note. The wrapper doesn't change the existing page body — it just
// adds framing so the OT-mode page feels like a discrete HMI screen
// rather than a re-toned web page. Pages should render this ONLY
// when getUiMode() returns "ot"; the IT surface stays unchanged.
export function OtPanel({
  tag,
  title,
  note,
  children,
}: {
  tag: string;
  title: string;
  note?: string;
  children: ReactNode;
}) {
  return (
    <div className="mx-auto max-w-5xl px-4 py-4 sm:px-6">
      <div className="mb-1 flex items-baseline justify-between font-mono text-[10px] uppercase tracking-[0.14em] text-ink-3">
        <span>
          <span className="text-ink">{title}</span>
        </span>
        <span className="flex items-center gap-3">
          {note ? <span>{note}</span> : null}
          <span className="text-ink-4">[{tag}]</span>
        </span>
      </div>
      <div className="border border-line-strong bg-paper-2 p-6 sm:p-8">
        {children}
      </div>
    </div>
  );
}
