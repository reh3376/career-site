import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

type Row = {
  kind: string;
  prompt_id?: string;
  promptId?: string;
  prompt_version?: number;
  promptVersion?: number;
  model?: string;
  output_json?: string;
  outputJson?: string;
  human_verdict?: string;
  humanVerdict?: string;
  reviewed_at?: string;
  reviewedAt?: string;
};

const VERDICTS = ["met", "partial", "unmet"] as const;
type Verdict = (typeof VERDICTS)[number];

type Tally = {
  reviewed: number;
  agree: number;
  matrix: Record<Verdict, Record<Verdict, number>>;
  byVersion: Map<string, { reviewed: number; agree: number }>;
  byModel: Map<string, { reviewed: number; agree: number }>;
};

function emptyMatrix(): Record<Verdict, Record<Verdict, number>> {
  const m = {} as Record<Verdict, Record<Verdict, number>>;
  for (const a of VERDICTS) {
    m[a] = { met: 0, partial: 0, unmet: 0 };
  }
  return m;
}

function isVerdict(s: string): s is Verdict {
  return (VERDICTS as readonly string[]).includes(s);
}

// Agreement between the judge and the owner on the requirement
// verdicts reviewed so far. Computed from the last 500 verdict rows;
// once the labelled set is larger than that this moves server-side.
export async function AgreementPanel() {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListDecisionLog",
    body: { kind: "jd_requirement_verdict", unreviewedOnly: false, limit: 500 },
    cookie,
  }).catch(() => null);
  if (!resp || !resp.ok) return null;
  const data = (await resp.json()) as { decisions?: Row[] };
  const rows = (data.decisions ?? []).filter((r) => r.reviewed_at ?? r.reviewedAt);
  if (rows.length === 0) {
    return (
      <p className="mt-6 text-sm text-ink-3">
        No graded verdicts yet. Agreement between the judge and you appears here
        once a few rows carry your verdict.
      </p>
    );
  }

  const t: Tally = { reviewed: 0, agree: 0, matrix: emptyMatrix(), byVersion: new Map(), byModel: new Map() };
  for (const r of rows) {
    let model = "";
    try {
      const out = JSON.parse(r.output_json ?? r.outputJson ?? "{}") as { verdict?: string };
      model = out.verdict ?? "";
    } catch {
      /* skip */
    }
    const human = (r.human_verdict ?? r.humanVerdict ?? "").toLowerCase();
    if (!isVerdict(model) || !isVerdict(human)) continue;
    t.reviewed++;
    const agree = model === human;
    if (agree) t.agree++;
    t.matrix[model][human]++;
    const ver = `${r.prompt_id ?? r.promptId ?? "?"} v${r.prompt_version ?? r.promptVersion ?? "?"}`;
    const v = t.byVersion.get(ver) ?? { reviewed: 0, agree: 0 };
    v.reviewed++;
    if (agree) v.agree++;
    t.byVersion.set(ver, v);
    const mk = r.model ?? "?";
    const m = t.byModel.get(mk) ?? { reviewed: 0, agree: 0 };
    m.reviewed++;
    if (agree) m.agree++;
    t.byModel.set(mk, m);
  }
  if (t.reviewed === 0) return null;
  const pct = (a: number, n: number) => (n ? `${Math.round((100 * a) / n)}%` : "");

  // Where the judge errs: rows off the diagonal, summarised in words.
  const stricter = t.matrix.unmet.met + t.matrix.unmet.partial + t.matrix.partial.met;
  const lenient = t.matrix.met.partial + t.matrix.met.unmet + t.matrix.partial.unmet;

  return (
    <section aria-labelledby="agreement-heading" className="mt-10 border border-line p-5">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">agreement</p>
      <h2
        id="agreement-heading"
        className="font-display mt-2 text-2xl leading-tight text-ink"
        style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
      >
        You and the judge agree on {pct(t.agree, t.reviewed)} of {t.reviewed} graded verdicts.
      </h2>
      <p className="mt-2 text-sm text-ink-2">
        {stricter} where the judge was stricter than you, {lenient} where it was more lenient.
        {stricter > lenient
          ? " A judge that is mostly too strict points at evidence gaps: check what the corpus and the facts sheet say."
          : lenient > stricter
            ? " A judge that is mostly too lenient points at the prompt: tighten the verdict rules."
            : ""}
      </p>

      <div className="mt-5 grid gap-8 md:grid-cols-[auto_1fr]">
        <table className="text-sm">
          <caption className="mb-2 text-left font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            judge (rows) vs you (columns)
          </caption>
          <thead>
            <tr>
              <th className="pr-4 text-left font-normal text-ink-3"></th>
              {VERDICTS.map((v) => (
                <th key={v} className="px-3 text-right font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  {v}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {VERDICTS.map((m) => (
              <tr key={m}>
                <th className="pr-4 text-left font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">{m}</th>
                {VERDICTS.map((h) => (
                  <td
                    key={h}
                    className={"px-3 text-right font-mono tabular " + (m === h ? "text-signal" : t.matrix[m][h] ? "text-ink" : "text-ink-4")}
                  >
                    {t.matrix[m][h]}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
        </table>

        <div className="grid gap-4 sm:grid-cols-2">
          <dl className="text-sm">
            <dt className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">by prompt version</dt>
            {[...t.byVersion.entries()].map(([k, v]) => (
              <dd key={k} className="m-0 mt-1 flex justify-between gap-4 text-ink-2">
                <span className="font-mono text-xs">{k}</span>
                <span className="font-mono text-xs text-ink">
                  {pct(v.agree, v.reviewed)} of {v.reviewed}
                </span>
              </dd>
            ))}
          </dl>
          <dl className="text-sm">
            <dt className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">by model</dt>
            {[...t.byModel.entries()].map(([k, v]) => (
              <dd key={k} className="m-0 mt-1 flex justify-between gap-4 text-ink-2">
                <span className="font-mono text-xs">{k}</span>
                <span className="font-mono text-xs text-ink">
                  {pct(v.agree, v.reviewed)} of {v.reviewed}
                </span>
              </dd>
            ))}
          </dl>
        </div>
      </div>
    </section>
  );
}
