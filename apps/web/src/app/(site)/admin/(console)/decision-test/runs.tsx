import Link from "next/link";

import { statusLabel } from "./review";

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
  ageRange?: string;
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
  baselineRtMs?: number;
  baselineRtSdMs?: number;
  repeatMatchedBy?: string;
  recallStrategy?: string;
  // Curation (M9). An empty reviewStatus means nobody has looked at
  // this run, which is a state rather than a verdict.
  reviewStatus?: string;
  reviewReason?: string;
  reviewNote?: string;
  reviewedAt?: string;
  blocksExcluded?: number;
  keyVersion?: string;
  wantsResults?: boolean;
  finishedAt?: string;
  reviewedByName?: string;
  // Where this sitting falls in the participant's sequence. attemptNo
  // is 1 for a first run and 0 when no identity could place it, which
  // is a different statement and must not be shown as "first".
  attemptNo?: number;
  attemptNoStrongest?: number;
  attemptSource?: string;
  attemptSourcesDisagree?: boolean;
  // What each identity could see when the run started. Absent means
  // there was no such link; 0 means there was one and it saw nothing.
  //
  // A nested message rather than three scalars: the flat
  // prior_by_* fields are plain int32 and a zero vanishes from the
  // JSON under proto3 implicit presence, which destroys exactly that
  // distinction. They are deprecated in the contract and still sent;
  // this is the one to read.
  priorSittings?: {
    byAccount?: number;
    byEmail?: number;
    byCookie?: number;
  };
};

/**
 * How a run's place in the sequence should read.
 *
 * The owner's requirement was that a first run be labelled as a first
 * run and every later one labelled as what it is. The awkward case is
 * the one that gets flattened by accident: a run no identity could
 * place is not a first run, it is a run with no sequence, and calling
 * it "first" would be inventing a fact about a person.
 *
 * Returns null when there is nothing worth showing, so callers can drop
 * it from a list of tags rather than render an empty one.
 */
export function attemptLabel(r: Run): string | null {
  const n = r.attemptNo ?? 0;
  if (!n) return "sequence unknown";
  if (n === 1) return "first run";
  return `run ${n}`;
}

/** Where that number came from, which is how much it is worth. */
export function attemptEvidence(r: Run): string | null {
  const n = r.attemptNo ?? 0;
  if (!n) return null;
  const parts: string[] = [];
  if (r.attemptSource && r.attemptSource !== "none") {
    parts.push(`by ${r.attemptSource}`);
  }
  if (r.attemptSourcesDisagree && r.attemptNoStrongest) {
    // Worth the words. The union and the strongest single source
    // disagreeing means the identities saw different histories, which
    // is exactly the case a reviewer should decide rather than a
    // default.
    parts.push(`strongest source says ${r.attemptNoStrongest}`);
  }
  return parts.length ? parts.join(", ") : null;
}

export type ItemPool = {
  category?: string;
  realItems?: number;
  fixtures?: number;
  perTest?: number;
  spare?: number;
  canVary?: boolean;
};

/**
 * Pool status, on the page rather than in a view nobody opens.
 *
 * A pool the same size as its quota is the one fault here that looks
 * exactly like success: the draw runs, every block is balanced, every
 * check passes, and every participant sits the identical test. It was
 * the state of the bank for three days, and the only way to notice was
 * to query a view by hand.
 */
export function PoolStatus({ pools }: { pools: ItemPool[] }) {
  if (!pools.length) return null;
  const stuck = pools.filter((p) => !p.canVary);
  if (!stuck.length) {
    return (
      <p className="mt-6 font-mono text-[11px] tracking-[0.1em] text-ink-3">
        item pools:{" "}
        {pools
          .map((p) => `${p.category} ${p.realItems}/${p.perTest}`)
          .join("  ·  ")}
      </p>
    );
  }
  return (
    <div className="mt-6 rounded-md border border-signal bg-paper-2 px-5 py-4">
      <p className="font-mono text-[11px] tracking-[0.14em] text-signal uppercase">
        Every participant is seeing the same questions
      </p>
      <p className="mt-3 text-sm leading-relaxed text-ink-2">
        {stuck.length === 1 ? "One category has" : `${stuck.length} categories have`}{" "}
        no more items than a single test uses, so the draw has nothing to
        leave behind. The test still works and every block is balanced;
        it just cannot vary.
      </p>
      <ul className="mt-3 space-y-1">
        {stuck.map((p) => (
          <li key={p.category} className="font-mono text-[11px] text-ink-3">
            {p.category}: {p.realItems} items for the {p.perTest} a test needs
            {(p.fixtures ?? 0) > 0 ? `, plus ${p.fixtures} placeholders` : ""}
          </li>
        ))}
      </ul>
    </div>
  );
}

export type ReviewCounts = {
  total?: number;
  unreviewed?: number;
  good?: number;
  incomplete?: number;
  hold?: number;
  doNotUse?: number;
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
            className={`group grid gap-2 py-5 no-underline sm:grid-cols-[1fr_auto] sm:gap-8 ${
              r.reviewStatus === "do_not_use" ? "opacity-50" : ""
            }`}
          >
            <div className="min-w-0">
              <p className="text-base text-ink transition-colors group-hover:text-accent">
                {r.displayName || "Anonymous"}
                {/* The sequence rather than a bare "repeat": which
                    sitting this was is the thing the owner asked to be
                    labelled, and "run 3" says it where "repeat" does
                    not. Shown for a first run too, because a list where
                    only repeats are marked makes an unplaceable run
                    look like a first one. */}
                <Tag>{attemptLabel(r)}</Tag>
                {r.isSynthetic ? <Tag>agent</Tag> : null}
                {r.status !== "completed" ? <Tag>{r.status}</Tag> : null}
                {r.reviewStatus ? <Tag>{statusLabel(r.reviewStatus)}</Tag> : null}
                {r.blocksExcluded ? <Tag>{r.blocksExcluded} block excluded</Tag> : null}
                {r.audioMode === "visual" ? <Tag>visual</Tag> : null}
                {r.tapCheckPassed === false ? <Tag>tap failed</Tag> : null}
              </p>
              <p className="mt-1 font-mono text-[11px] tracking-[0.1em] text-ink-3">
                {[
                  r.startedAt?.slice(0, 16).replace("T", " "),
                  r.ageRange || null,
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
