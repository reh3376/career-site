"use client";

import { useState } from "react";
import { useFormStatus } from "react-dom";

import { ingestCorpusTextAction, type IngestResult } from "./actions";

const SOURCE_KINDS = [
  { key: "article", label: "Article" },
  { key: "resume", label: "Résumé variant" },
  { key: "career_note", label: "Career note" },
  { key: "adr", label: "ADR / decision" },
  { key: "other", label: "Other" },
] as const;

// Paste form for seeding the Ask Roger corpus by hand. Chunks +
// embeds server-side; result renders inline with counts so the
// admin can tell whether it landed (or was a no-op due to
// content-hash match).
export function IngestForm() {
  const [result, setResult] = useState<IngestResult | null>(null);

  async function onSubmit(formData: FormData): Promise<void> {
    setResult(null);
    const res = await ingestCorpusTextAction(formData);
    setResult(res);
  }

  return (
    <form
      action={onSubmit}
      className="grid gap-5 border border-line-strong bg-canvas p-5"
    >
      <div className="grid gap-4 sm:grid-cols-[180px_1fr_1fr]">
        <label className="block text-xs">
          <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
            source kind
          </span>
          <select
            name="source_kind"
            defaultValue="article"
            className="mt-1 block w-full border border-line-strong bg-canvas px-2 py-1.5 font-mono text-sm text-ink outline-none focus:border-accent"
          >
            {SOURCE_KINDS.map((k) => (
              <option key={k.key} value={k.key}>
                {k.label}
              </option>
            ))}
          </select>
        </label>
        <label className="block text-xs">
          <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
            source path
          </span>
          <input
            type="text"
            name="source_path"
            required
            maxLength={200}
            placeholder="paste/roger-2026-09-21-notes"
            className="mt-1 block w-full border border-line-strong bg-canvas px-2 py-1.5 font-mono text-sm text-ink outline-none focus:border-accent"
          />
        </label>
        <label className="block text-xs">
          <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
            title (optional)
          </span>
          <input
            type="text"
            name="title"
            maxLength={300}
            placeholder="Falls back to first non-blank line"
            className="mt-1 block w-full border border-line-strong bg-canvas px-2 py-1.5 font-mono text-sm text-ink outline-none focus:border-accent"
          />
        </label>
      </div>

      <label className="block text-xs">
        <span className="font-mono uppercase tracking-[0.14em] text-ink-3">
          body
        </span>
        <textarea
          name="body"
          required
          rows={16}
          maxLength={204800}
          placeholder="Paste the document text. Markdown OK, front-matter is fine as long as the body renders as prose. 200 KiB cap."
          className="mt-1 block w-full border border-line-strong bg-paper-2 px-3 py-2 font-mono text-[13px] leading-snug text-ink outline-none focus:border-accent"
        />
        <span className="mt-1 block text-[11px] text-ink-3">
          Ingest is idempotent on (source_kind, source_path). A re-paste of
          the same body is a no-op (skipped=true).
        </span>
      </label>

      <Submit />

      {result ? (
        result.ok ? (
          <div
            className={
              "border-l-2 px-4 py-3 text-sm " +
              (result.skipped
                ? "border-ink-3 bg-paper-2 text-ink-2"
                : "border-success bg-success-soft/50 text-ink")
            }
          >
            <p className="font-mono text-[11px] uppercase tracking-[0.14em]">
              {result.skipped ? "skipped, unchanged" : "ingested"}
            </p>
            <p className="mt-1 font-mono text-xs">
              doc #{result.document_id}
              {result.skipped ? null : (
                <>
                  {" "}
                  · chunks {result.chunks_inserted}, embedded{" "}
                  {result.chunks_embedded}
                </>
              )}
              {result.chunker_name ? (
                <> · chunker {result.chunker_name}</>
              ) : null}
              {result.embedder_model ? (
                <> · embedder {result.embedder_model}</>
              ) : null}
            </p>
          </div>
        ) : (
          <div className="border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
            {result.error}
          </div>
        )
      ) : null}
    </form>
  );
}

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex h-9 w-fit items-center border border-accent bg-accent px-4 font-mono text-[11px] uppercase tracking-[0.14em] text-white transition-colors hover:bg-accent-strong disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Ingesting…" : "Ingest document"}
    </button>
  );
}
