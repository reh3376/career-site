// Run history for one submission (data layer D2).
//
// The rest of this page shows the latest run, because that is what the
// submission row holds. A re-score overwrites it, so without this
// section the question "what did it score before the prompt changed,
// and what was different about that run" has no answer in the console.
// Each row here is immutable and carries its own provenance.

export type Run = {
  run_id?: string;
  runId?: string;
  attempt?: number;
  trigger?: string;
  status?: string;
  error?: string;
  app_commit?: string;
  appCommit?: string;
  host?: string;
  model?: string;
  num_ctx?: number;
  numCtx?: number;
  embedder_model?: string;
  embedderModel?: string;
  prompts_json?: string;
  promptsJson?: string;
  corpus_fingerprint?: string;
  corpusFingerprint?: string;
  corpus_documents?: number;
  corpusDocuments?: number;
  corpus_chunks?: number;
  corpusChunks?: number;
  score_formula?: string;
  scoreFormula?: string;
  retrieval_score?: number;
  retrievalScore?: number;
  match_score?: number;
  matchScore?: number;
  threshold?: number;
  fit?: string;
  requirement_count?: number;
  requirementCount?: number;
  met_count?: number;
  metCount?: number;
  partial_count?: number;
  partialCount?: number;
  unmet_count?: number;
  unmetCount?: number;
  resume_generated?: boolean;
  resumeGenerated?: boolean;
  queued_ms?: number;
  queuedMs?: number;
  duration_ms?: number;
  durationMs?: number;
  started_at?: string;
  startedAt?: string;
};

// Connect's JSON uses camelCase; some call paths here have historically
// produced snake_case. Reading both is the rule on every admin page.
function pick<T>(a: T | undefined, b: T | undefined): T | undefined {
  return a !== undefined ? a : b;
}

const STATUS_TONE: Record<string, string> = {
  ready: "text-signal",
  below_threshold: "text-ink-2",
  failed: "text-danger",
  running: "text-accent",
};

function ms(v: number | undefined): string {
  const n = Number(v ?? 0);
  if (n <= 0) return "0s";
  if (n < 1000) return `${n}ms`;
  if (n < 60000) return `${(n / 1000).toFixed(1)}s`;
  const m = Math.floor(n / 60000);
  const s = Math.round((n % 60000) / 1000);
  return `${m}m ${s}s`;
}

function score(v: number | undefined): string {
  return v === undefined || v === null ? "-" : Number(v).toFixed(3);
}

export function RunHistory({ runs }: { runs: Run[] }) {
  if (runs.length === 0) {
    return (
      <section className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          run history
        </p>
        <p className="mt-3 text-sm text-ink-3">
          No runs recorded. Submissions scored before the run table existed have
          none; the next re-score will create one.
        </p>
      </section>
    );
  }

  return (
    <section className="mt-10">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        run history <span className="text-ink-4">·</span> {runs.length}
      </p>
      <p className="mt-2 max-w-2xl text-sm leading-relaxed text-ink-2">
        Every run this submission has had, newest first. Each keeps what
        produced it, so two attempts can be compared: a score that moved while
        the corpus fingerprint and prompt hashes stayed the same moved because
        of the model, not the evidence.
      </p>

      <ul className="mt-6 space-y-4">
        {runs.map((run) => {
          const status = pick(run.status, undefined) ?? "";
          const prompts = pick(run.prompts_json, run.promptsJson) ?? "{}";
          let promptPairs: [string, string][] = [];
          try {
            promptPairs = Object.entries(
              JSON.parse(prompts) as Record<string, string>,
            );
          } catch {
            promptPairs = [];
          }
          const started = pick(run.started_at, run.startedAt);
          return (
            <li
              key={pick(run.run_id, run.runId) ?? run.attempt}
              className="border border-line bg-paper-2 px-4 py-3"
            >
              <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
                <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2">
                  attempt {pick(run.attempt, undefined) ?? "?"}
                  <span className="text-ink-4"> · </span>
                  {pick(run.trigger, undefined) ?? "submit"}
                  <span className="text-ink-4"> · </span>
                  <span className={STATUS_TONE[status] ?? "text-ink"}>
                    {status || "unknown"}
                  </span>
                </p>
                <p className="font-mono text-[11px] text-ink-3">
                  {started ? new Date(started).toLocaleString() : ""}
                </p>
              </div>

              <dl className="mt-3 grid gap-x-6 gap-y-2 text-sm sm:grid-cols-2 lg:grid-cols-3">
                <Field label="match">
                  {score(pick(run.match_score, run.matchScore))}
                  <span className="text-ink-3">
                    {" "}
                    vs {score(pick(run.threshold, undefined))}
                  </span>
                  {run.fit ? (
                    <span className="text-ink-3"> · {run.fit}</span>
                  ) : null}
                </Field>
                <Field label="retrieval">
                  {score(pick(run.retrieval_score, run.retrievalScore))}
                </Field>
                <Field label="requirements">
                  {pick(run.requirement_count, run.requirementCount) ?? 0}
                  <span className="text-ink-3">
                    {" "}
                    · {pick(run.met_count, run.metCount) ?? 0} met,{" "}
                    {pick(run.partial_count, run.partialCount) ?? 0} partial,{" "}
                    {pick(run.unmet_count, run.unmetCount) ?? 0} unmet
                  </span>
                </Field>
                <Field label="model">
                  {pick(run.model, undefined) || "-"}
                  {pick(run.num_ctx, run.numCtx) ? (
                    <span className="text-ink-3">
                      {" "}
                      · {pick(run.num_ctx, run.numCtx)} ctx
                    </span>
                  ) : null}
                  {pick(run.host, undefined) ? (
                    <span className="text-ink-3">
                      {" "}
                      · {pick(run.host, undefined)}
                    </span>
                  ) : null}
                </Field>
                <Field label="corpus">
                  {pick(run.corpus_documents, run.corpusDocuments) ?? 0} docs,{" "}
                  {pick(run.corpus_chunks, run.corpusChunks) ?? 0} chunks
                  <span className="font-mono text-xs text-ink-3">
                    {" "}
                    {pick(run.corpus_fingerprint, run.corpusFingerprint) || ""}
                  </span>
                </Field>
                <Field label="took">
                  {ms(pick(run.duration_ms, run.durationMs))}
                  <span className="text-ink-3">
                    {" "}
                    · queued {ms(pick(run.queued_ms, run.queuedMs))}
                  </span>
                </Field>
              </dl>

              <p className="mt-3 font-mono text-[11px] leading-relaxed text-ink-3">
                {promptPairs.map(([id, v]) => `${id} ${v}`).join("  ·  ")}
                {pick(run.score_formula, run.scoreFormula) ? (
                  <>
                    {promptPairs.length ? "  ·  " : ""}
                    formula {pick(run.score_formula, run.scoreFormula)}
                  </>
                ) : null}
                {pick(run.embedder_model, run.embedderModel) ? (
                  <>
                    {"  ·  "}embed {pick(run.embedder_model, run.embedderModel)}
                  </>
                ) : null}
                {pick(run.app_commit, run.appCommit) ? (
                  <>
                    {"  ·  "}build {pick(run.app_commit, run.appCommit)}
                  </>
                ) : null}
              </p>

              {run.error ? (
                <p className="mt-2 border-l-2 border-danger pl-3 text-sm text-ink-2">
                  {run.error}
                </p>
              ) : null}
            </li>
          );
        })}
      </ul>
    </section>
  );
}

function Field({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div>
      <dt className="font-mono text-[10px] uppercase tracking-[0.14em] text-ink-4">
        {label}
      </dt>
      <dd className="mt-0.5 text-ink">{children}</dd>
    </div>
  );
}
