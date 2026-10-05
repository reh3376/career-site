import Link from "next/link";

// The run list.
//
// Shows whether an address was left rather than the address itself. The
// console needs to know whether results can be sent; it does not need to
// be a mailing list, and a screen full of participant emails is a
// screen that should not exist.
export type Run = {
  sessionKey: string;
  status: string;
  displayName?: string;
  education?: string;
  occupation?: string;
  gaveEmail?: boolean;
  audioMode?: string;
  deviceClass?: string;
  tapCheckPassed?: boolean;
  isRepeat?: boolean;
  isSynthetic?: boolean;
  instrumentVersion?: string;
  itemSetVersion?: string;
  correct?: number;
  answered?: number;
  expired?: number;
  meanConfidence?: number;
  durationS?: number;
  startedAt?: string;
};

export function RunTable({ runs }: { runs: Run[] }) {
  if (!runs.length) {
    return (
      <p className="mt-10 rounded-md border border-line bg-paper-2 px-5 py-4 text-base text-ink-2">
        No runs yet. A run appears here the moment somebody presses start,
        finished or not: an abandoned run is kept, because where people
        stop is the only measurement of whether fifteen minutes is too
        much to ask.
      </p>
    );
  }
  return (
    <ul className="mt-10 divide-y divide-line border-y border-line">
      {runs.map((r) => (
        <li key={r.sessionKey}>
          <Link
            href={`/admin/decision-test/${r.sessionKey}`}
            className="group grid gap-2 py-5 no-underline sm:grid-cols-[1fr_auto] sm:gap-8"
          >
            <div className="min-w-0">
              <p className="text-base text-ink transition-colors group-hover:text-accent">
                {r.displayName || "Anonymous"}
                {r.isRepeat ? <Tag>repeat</Tag> : null}
                {r.isSynthetic ? <Tag>agent</Tag> : null}
                {r.status !== "completed" ? <Tag>{r.status}</Tag> : null}
                {r.audioMode === "visual" ? <Tag>visual</Tag> : null}
                {r.tapCheckPassed === false ? <Tag>tap failed</Tag> : null}
              </p>
              <p className="mt-1 font-mono text-[11px] tracking-[0.1em] text-ink-3">
                {[
                  r.startedAt?.slice(0, 16).replace("T", " "),
                  r.education || null,
                  r.deviceClass,
                  r.gaveEmail ? "email given" : "no email",
                  r.instrumentVersion,
                ]
                  .filter(Boolean)
                  .join("  ·  ")}
              </p>
            </div>
            <div className="shrink-0 text-right">
              <p className="font-display text-xl text-ink tabular-nums">
                {r.correct ?? 0}
                <span className="text-ink-3">/{r.answered ?? 0}</span>
              </p>
              <p className="mt-1 font-mono text-[11px] tracking-[0.1em] text-ink-3">
                {r.meanConfidence ?? 0}% confident
                {r.expired ? `  ·  ${r.expired} expired` : ""}
                {r.durationS ? `  ·  ${Math.round(r.durationS / 60)} min` : ""}
              </p>
            </div>
          </Link>
        </li>
      ))}
    </ul>
  );
}

function Tag({ children }: { children: React.ReactNode }) {
  return (
    <span className="ml-2 rounded-sm bg-paper-3 px-2 py-0.5 font-mono text-[10px] tracking-[0.14em] text-ink-3 uppercase">
      {children}
    </span>
  );
}
