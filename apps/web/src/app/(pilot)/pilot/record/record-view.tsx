import type { ReactNode } from "react";

// The match record as embedded in the API (services/api/internal/pilot/
// data/stimulus-<version>.json), typed loosely: every field optional,
// because the record is versioned data and a later version may add or
// drop fields without this component having to know. v1.1 (2026-11-09)
// added the labelled parts, the definitions panel, the record status,
// the r1 callout and the follow-up and inclusion lines, all from the
// owner's comments after his own run (docs/framework-research/15 §14).
export type RecordRow = {
  id: string;
  requirement?: string;
  source_quote?: string;
  category?: string;
  type?: string;
  weight?: number;
  determination?: string;
  determination_label?: string;
  basis?: string;
  basis_label?: string;
  basis_code?: string;
  history?: string;
  rationale?: string;
  evidence?: string;
  flagged_for_audit?: boolean;
  no_override?: boolean;
  callout?: { title?: string; lines?: string[] };
  inclusion?: { status?: string; reason?: string };
  followup?: { status?: string; next?: string };
  note?: string;
  question?: {
    label?: string;
    text?: string;
    fields?: { label?: string; options?: string[] }[];
    footnote?: string;
  };
};

export type RecordData = {
  version?: string;
  header?: {
    run_label?: string;
    role_title?: string;
    subtitle?: string;
    intro?: string;
    run_note?: string;
    status_label?: string;
    status?: string;
    status_note?: string;
    summary?: string;
    coverage_label?: string;
    evidenced_pct?: number;
    unresolved_pct?: number;
    unresolved_note?: string;
    unmet_pct?: number;
    equivalency_line?: string;
    self_attested_line?: string;
  };
  legend?: { determinations?: string[]; must?: string; nice?: string; weight?: string };
  definitions?: {
    title?: string;
    intro?: string;
    groups?: { title?: string; terms?: { term?: string; meaning?: string }[] }[];
  };
  rows?: RecordRow[];
  footer?: { how_title?: string; how?: string; versions?: string[] };
};

const TONE: Record<string, { text: string; bar: string }> = {
  met: { text: "text-success", bar: "bg-success" },
  met_equivalent: { text: "text-success", bar: "bg-success/60" },
  unresolved: { text: "text-signal", bar: "bg-signal" },
  not_met: { text: "text-danger", bar: "bg-danger" },
  not_assessed: { text: "text-ink-3", bar: "bg-ink-4" },
};

const CLASS_LABEL: Record<string, string> = { must: "Required", nice: "Preferred", ideal: "Ideal" };

const MONO = "font-mono text-[11px] uppercase tracking-[0.14em]";

export function RecordView({ record, rowComment }: { record: RecordData; rowComment: (rowId: string) => ReactNode }) {
  const h = record.header ?? {};
  const rows = record.rows ?? [];
  const ev = h.evidenced_pct ?? 0;
  const un = h.unresolved_pct ?? 0;
  const nm = h.unmet_pct ?? 0;
  return (
    <article className="mt-12 border border-line">
      <header className="border-b border-line bg-paper-2 px-6 py-6 sm:px-8">
        <p className={`${MONO} text-ink-3`}>{h.run_label}</p>
        <h2
          className="font-display mt-2 text-2xl leading-tight text-ink sm:text-3xl"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          {h.role_title}
        </h2>
        <p className="mt-1 text-sm text-ink-2">{h.subtitle}</p>
        {h.intro ? <p className="mt-4 max-w-2xl text-sm leading-relaxed text-ink-2">{h.intro}</p> : null}
        {h.run_note ? (
          <p className="mt-4 max-w-2xl border-l-2 border-line-strong bg-canvas px-4 py-3 text-sm leading-relaxed text-ink">{h.run_note}</p>
        ) : null}

        {h.status ? (
          <div className="mt-6 border border-signal/40 bg-signal-soft/40 px-4 py-4">
            <p className={`${MONO} text-ink-3`}>{h.status_label ?? "Status"}</p>
            <p className="mt-1 font-mono text-sm uppercase tracking-[0.14em] text-signal">{h.status}</p>
            {h.status_note ? <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink">{h.status_note}</p> : null}
          </div>
        ) : null}

        {h.summary ? <p className="mt-6 max-w-2xl text-base leading-relaxed text-ink">{h.summary}</p> : null}

        <div className="mt-6">
          <p className={`${MONO} text-ink-3`}>{h.coverage_label}</p>
          <div className="mt-2 flex h-3 w-full overflow-hidden bg-paper-3" role="img" aria-label={`${ev}% evidenced, ${un}% unresolved, ${nm}% unmet`}>
            <div className="h-full bg-success" style={{ width: `${ev}%` }} />
            <div className="h-full bg-signal" style={{ width: `${un}%` }} />
            <div className="h-full bg-danger" style={{ width: `${nm}%` }} />
          </div>
          <dl className="mt-3 grid gap-3 font-mono text-sm sm:grid-cols-3">
            <Num label="evidenced" value={`${ev}%`} tone="text-success" />
            <Num label="unresolved" value={`${un}%`} tone="text-signal" note={h.unresolved_note} />
            <Num label="unmet" value={`${nm}%`} tone="text-danger" />
          </dl>
          {h.equivalency_line ? <p className="mt-3 text-xs text-ink-3">{h.equivalency_line}</p> : null}
          {h.self_attested_line ? <p className="mt-1 text-xs text-ink-3">{h.self_attested_line}</p> : null}
        </div>

        {record.legend && !record.definitions ? (
          <p className={`mt-5 ${MONO} text-ink-4`}>
            {(record.legend.determinations ?? []).join(" · ")}
            {record.legend.must ? ` · must: ${record.legend.must}` : ""}
            {record.legend.nice ? ` · nice: ${record.legend.nice}` : ""}
            {record.legend.weight ? ` · ${record.legend.weight}` : ""}
          </p>
        ) : null}
      </header>

      {record.definitions ? <Definitions d={record.definitions} /> : null}

      <ol className="divide-y divide-line">
        {rows.map((r) => (
          <Row key={r.id} r={r}>
            {rowComment(r.id)}
          </Row>
        ))}
      </ol>

      {record.footer ? (
        <footer className="border-t border-line bg-paper-2 px-6 py-6 text-sm text-ink-2 sm:px-8">
          <p className={`${MONO} text-ink-3`}>{record.footer.how_title}</p>
          <p className="mt-2 max-w-3xl leading-relaxed">{record.footer.how}</p>
          {record.footer.versions?.length ? (
            <ul className="mt-3 space-y-1 font-mono text-[11px] text-ink-4">
              {record.footer.versions.map((v) => (
                <li key={v}>{v}</li>
              ))}
            </ul>
          ) : null}
        </footer>
      ) : null}
    </article>
  );
}

function Num({ label, value, tone, note }: { label: string; value: string; tone: string; note?: string }) {
  return (
    <div className="flex items-baseline gap-3">
      <dt className="text-ink-3">{label}</dt>
      <dd className={`m-0 text-2xl ${tone}`}>{value}</dd>
      {note ? <dd className="m-0 text-xs text-ink-3">{note}</dd> : null}
    </div>
  );
}

// The definitions panel sits between the header and the rows, open, so
// that a reader meets every term before the first row uses it.
function Definitions({ d }: { d: NonNullable<RecordData["definitions"]> }) {
  const groups = d.groups ?? [];
  return (
    <section aria-labelledby="record-definitions" className="border-b border-line px-6 py-6 sm:px-8">
      <p id="record-definitions" className={`${MONO} text-ink-3`}>
        {d.title ?? "Definitions"}
      </p>
      {d.intro ? <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-2">{d.intro}</p> : null}
      <div className="mt-5 grid gap-6 sm:grid-cols-2">
        {groups.map((g, i) => (
          <div key={g.title ?? i}>
            <p className="text-sm font-medium text-ink">{g.title}</p>
            <dl className="mt-2 space-y-2">
              {(g.terms ?? []).map((t, j) => (
                <div key={t.term ?? j} className="text-sm leading-relaxed">
                  <dt className="inline font-medium text-ink">{t.term}.</dt> <dd className="inline text-ink-2">{t.meaning}</dd>
                </div>
              ))}
            </dl>
          </div>
        ))}
      </div>
    </section>
  );
}

function Label({ children, className = "" }: { children: ReactNode; className?: string }) {
  return <p className={`${MONO} text-ink-3 ${className}`}>{children}</p>;
}

function Row({ r, children }: { r: RecordRow; children: ReactNode }) {
  const tone = TONE[r.determination ?? ""] ?? TONE.not_assessed;
  const cls = CLASS_LABEL[r.category ?? ""] ?? r.category;
  return (
    <li className="px-6 py-7 sm:px-8">
      <div className={`flex flex-wrap items-baseline gap-x-3 gap-y-1 ${MONO} text-ink-3`}>
        <span className="text-ink">{r.id}</span>
        {cls ? <span className={r.category === "must" ? "text-ink-2" : "text-ink-4"}>{cls}</span> : null}
        {typeof r.weight === "number" ? <span>weight {r.weight}</span> : null}
        {r.type ? <span>kind: {r.type}</span> : null}
        {r.flagged_for_audit ? <span className="text-signal">flagged for expert audit</span> : null}
      </div>

      <Label className="mt-4">Requirement</Label>
      <p className="mt-1 text-base leading-relaxed text-ink">{r.requirement}</p>
      {r.source_quote ? (
        <>
          <Label className="mt-3">As the posting worded it</Label>
          <p className="mt-1 text-sm italic text-ink-3">&ldquo;{r.source_quote.replace(/^…/, "")}&rdquo;</p>
        </>
      ) : null}

      {r.callout ? (
        <div className="mt-5 border border-signal/40 bg-signal-soft/40 px-4 py-4">
          <p className={`${MONO} text-signal`}>{r.callout.title ?? "What happened on this row"}</p>
          <div className="mt-2 space-y-2 text-sm leading-relaxed text-ink">
            {(r.callout.lines ?? []).map((line, i) => (
              <p key={i}>{line}</p>
            ))}
          </div>
        </div>
      ) : null}

      <div className="mt-5 grid gap-5 sm:grid-cols-[minmax(0,220px)_1fr]">
        <div>
          <Label>Verdict</Label>
          <p className={`mt-1 font-mono text-[12px] uppercase tracking-[0.14em] ${tone.text}`}>
            <span className={`mr-2 inline-block h-2 w-2 rounded-full align-middle ${tone.bar}`} aria-hidden="true" />
            {r.determination_label ?? r.determination}
          </p>
          {r.basis ? (
            <>
              <Label className="mt-3">{r.basis_label ?? "Basis"}</Label>
              <p className="mt-1 text-sm leading-relaxed text-ink-2">{r.basis}</p>
            </>
          ) : null}
        </div>
        <div>
          {r.history ? (
            <p className="mb-2 text-xs text-ink-3">
              <span className={MONO}>history</span> {r.history}
            </p>
          ) : null}
          <Label>Reasoning</Label>
          <p className="mt-1 text-sm leading-relaxed text-ink-2">{r.rationale}</p>
          {r.evidence ? (
            <>
              <Label className="mt-3">Evidence</Label>
              <p className="mt-1 font-mono text-[11px] text-ink-3">{r.evidence}</p>
            </>
          ) : null}
        </div>
      </div>

      {r.inclusion ? (
        <p className="mt-4 text-sm leading-relaxed text-ink-2">
          <span className={`${MONO} text-ink`}>{r.inclusion.status}</span>
          {r.inclusion.reason ? <> · {r.inclusion.reason}</> : null}
        </p>
      ) : null}

      {r.followup ? (
        <div className="mt-4 border-l-2 border-signal pl-4">
          <Label>Follow-up</Label>
          <p className="mt-1 text-sm text-ink">{r.followup.status}</p>
          {r.followup.next ? <p className="mt-1 text-sm leading-relaxed text-ink-2">{r.followup.next}</p> : null}
        </div>
      ) : null}

      {r.question ? <Question q={r.question} /> : null}

      {r.note ? <p className="mt-4 border-l-2 border-line-strong pl-4 text-sm leading-relaxed text-ink-2">{r.note}</p> : null}

      <div className="mt-5 flex flex-wrap items-center gap-3">
        <button
          type="button"
          disabled
          aria-disabled="true"
          className={`inline-flex h-[30px] cursor-not-allowed items-center border border-line px-3 ${MONO} text-ink-4`}
        >
          Audit
        </button>
        {r.no_override ? null : (
          <button
            type="button"
            disabled
            aria-disabled="true"
            className={`inline-flex h-[30px] cursor-not-allowed items-center border border-line px-3 ${MONO} text-ink-4`}
          >
            Override
          </button>
        )}
        <span className="text-xs text-ink-4">
          {r.no_override
            ? "Shown as the hiring manager sees them. Disabled in the pilot; this row is not scored and cannot be overridden."
            : "Shown as the hiring manager sees them. Disabled in the pilot; comments below never change the record."}
        </span>
      </div>

      {children}
    </li>
  );
}

function Question({ q }: { q: NonNullable<RecordRow["question"]> }) {
  return (
    <div className="mt-4 border border-line-strong bg-canvas px-4 py-4">
      <p className={`${MONO} text-signal`}>{q.label}</p>
      <p className="mt-2 text-sm leading-relaxed text-ink">{q.text}</p>
      {q.fields?.length ? (
        <div className="mt-3 grid gap-3 sm:grid-cols-2">
          {q.fields.map((f) => (
            <label key={f.label} className="flex flex-col gap-1">
              <span className={`${MONO} text-ink-3`}>{f.label}</span>
              <select disabled aria-disabled="true" className="border border-line bg-paper-2 px-2 py-1.5 text-sm text-ink-3" defaultValue="">
                <option value="">Awaiting the candidate</option>
                {(f.options ?? []).map((o) => (
                  <option key={o} value={o}>
                    {o}
                  </option>
                ))}
              </select>
            </label>
          ))}
        </div>
      ) : null}
      {q.footnote ? <p className="mt-3 text-xs text-ink-3">{q.footnote}</p> : null}
    </div>
  );
}
