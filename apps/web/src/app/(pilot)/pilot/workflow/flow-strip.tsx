// A small flow diagram: boxes left to right with an arrow between each,
// wrapping on a narrow screen. Used for the five stages on the overview
// and for one stage's steps on its own screen, so that a reader never
// has to zoom or pan to see a whole picture. A decision point is drawn
// with the accent border; a return edge (a step that can send the work
// back) is written under the strip in words rather than drawn, which
// reads better than a loop on a strip this small.
export type FlowItem = { id: string; label: string; decision?: boolean; current?: boolean };

export function FlowStrip({
  items,
  returns,
  label,
  enters,
  leaves,
}: {
  items: FlowItem[];
  returns?: string[];
  label: string;
  enters?: string;
  leaves?: string;
}) {
  return (
    <figure className="mt-4">
      <ol className="flex flex-wrap items-stretch gap-y-3" aria-label={label}>
        {enters ? <End text={enters} arrow /> : null}
        {items.map((it, i) => (
          <li key={it.id} className="flex items-stretch">
            <div
              className={`flex w-[150px] flex-col justify-center border px-3 py-2 ${
                it.decision ? "border-accent bg-canvas" : "border-line-strong bg-paper-2"
              } ${it.current ? "outline outline-2 outline-offset-2 outline-accent" : ""}`}
            >
              <span className={`font-mono text-[10px] uppercase tracking-[0.14em] ${it.decision ? "text-accent" : "text-ink-3"}`}>
                {it.id}
                {it.decision ? " · decision" : ""}
              </span>
              <span className="mt-1 text-xs leading-snug text-ink">{it.label}</span>
            </div>
            {i < items.length - 1 || leaves ? <Arrow /> : null}
          </li>
        ))}
        {leaves ? <End text={leaves} /> : null}
      </ol>
      {returns?.length ? (
        <figcaption className="mt-2 space-y-0.5 text-xs text-ink-3">
          {returns.map((r) => (
            <p key={r}>{r}</p>
          ))}
        </figcaption>
      ) : null}
    </figure>
  );
}

// What a stage starts from and what it hands on, drawn as dashed boxes
// at either end so that a stage read on its own screen still shows
// where it sits in the whole (the owner's review of revision 3 found
// the stages lost their continuity when the chart was split).
function End({ text, arrow }: { text: string; arrow?: boolean }) {
  return (
    <li className="flex items-stretch">
      <div className="flex w-[150px] flex-col justify-center border border-dashed border-line-strong px-3 py-2">
        <span className="text-xs italic leading-snug text-ink-3">{text}</span>
      </div>
      {arrow ? <Arrow /> : null}
    </li>
  );
}

function Arrow() {
  return (
    <svg aria-hidden="true" viewBox="0 0 24 24" className="h-4 w-7 shrink-0 self-center text-ink-4">
      <path d="M2 12h17m0 0-5-5m5 5-5 5" fill="none" stroke="currentColor" strokeWidth="1.5" strokeLinecap="round" strokeLinejoin="round" />
    </svg>
  );
}
