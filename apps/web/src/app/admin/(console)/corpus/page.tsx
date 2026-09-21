import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { IngestForm } from "./ingest-form";
import { ReindexPanel } from "./reindex-panel";

export const metadata: Metadata = { title: "Admin · Corpus" };
export const dynamic = "force-dynamic";

type CorpusDoc = {
  id: string;
  source_kind?: string;
  sourceKind?: string;
  source_path?: string;
  sourcePath?: string;
  title: string;
  visibility?: string;
  chunk_count?: number;
  chunkCount?: number;
  embedded_count?: number;
  embeddedCount?: number;
  ingested_at?: string;
  ingestedAt?: string;
  updated_at?: string;
  updatedAt?: string;
};

type ListResp = {
  documents?: CorpusDoc[];
  total_documents?: number;
  totalDocuments?: number;
  total_chunks?: number;
  totalChunks?: number;
  total_embedded?: number;
  totalEmbedded?: number;
  total_public?: number;
  totalPublic?: number;
  total_corpus_only?: number;
  totalCorpusOnly?: number;
};

type FetchResult =
  | { ok: true; data: ListResp }
  | { ok: false; error: string };

async function fetchCorpus(): Promise<FetchResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListCorpusDocuments",
    body: {},
    cookie,
  });
  if (!resp.ok) return { ok: false, error: `HTTP ${resp.status}` };
  return { ok: true, data: (await resp.json()) as ListResp };
}

export default async function AdminCorpusPage() {
  const result = await fetchCorpus();
  const nowMs = getNowMs();

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        admin surface
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Corpus.
      </h1>
      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        The Ask Roger corpus. Reindex from the two mounts, or paste a
        document to chunk + embed it. Public documents can be quoted
        and cited on the site; corpus-only documents inform answers
        and JD scoring but are never quoted or named. Chunking is
        deterministic (paragraph.v1) and embedding runs through the
        sidecar (stub by default, ollama in prod when configured).
      </p>

      {!result.ok ? (
        <p className="mt-8 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load corpus, {result.error}.
        </p>
      ) : (
        <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-5">
          <div className="flex items-baseline gap-3">
            <dt className="text-ink-3">documents</dt>
            <dd className="m-0 tabular text-ink text-2xl">
              {result.data.total_documents ?? result.data.totalDocuments ?? 0}
            </dd>
          </div>
          <div className="flex items-baseline gap-3">
            <dt className="text-ink-3">public</dt>
            <dd className="m-0 tabular text-ink text-2xl">
              {result.data.total_public ?? result.data.totalPublic ?? 0}
            </dd>
          </div>
          <div className="flex items-baseline gap-3">
            <dt className="text-ink-3">corpus-only</dt>
            <dd className="m-0 tabular text-ink text-2xl">
              {result.data.total_corpus_only ?? result.data.totalCorpusOnly ?? 0}
            </dd>
          </div>
          <div className="flex items-baseline gap-3">
            <dt className="text-ink-3">chunks</dt>
            <dd className="m-0 tabular text-ink text-2xl">
              {result.data.total_chunks ?? result.data.totalChunks ?? 0}
            </dd>
          </div>
          <div className="flex items-baseline gap-3">
            <dt className="text-ink-3">embedded</dt>
            <dd className="m-0 tabular text-ink text-2xl">
              {result.data.total_embedded ?? result.data.totalEmbedded ?? 0}
            </dd>
          </div>
        </dl>
      )}

      <section aria-label="Reindex from filesystem" className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          reindex from filesystem
        </p>
        <div className="mt-3">
          <ReindexPanel />
        </div>
      </section>

      <section aria-label="Ingest a document" className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          paste + ingest
        </p>
        <div className="mt-3">
          <IngestForm />
        </div>
      </section>

      <section aria-label="Corpus documents" className="mt-12">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          stored documents
        </p>
        {result.ok && (result.data.documents ?? []).length > 0 ? (
          <ul className="mt-4 divide-y divide-line border-y border-line">
            {(result.data.documents ?? []).map((d) => (
              <DocRow key={d.id} d={d} nowMs={nowMs} />
            ))}
          </ul>
        ) : (
          <p className="mt-4 text-sm text-ink-3">Nothing ingested yet.</p>
        )}
      </section>
    </>
  );
}

function DocRow({ d, nowMs }: { d: CorpusDoc; nowMs: number }) {
  const sourceKind = d.source_kind ?? d.sourceKind ?? "";
  const sourcePath = d.source_path ?? d.sourcePath ?? "";
  const chunks = d.chunk_count ?? d.chunkCount ?? 0;
  const embedded = d.embedded_count ?? d.embeddedCount ?? 0;
  const ingestedIso = d.ingested_at ?? d.ingestedAt;
  const ingested = ingestedIso ? new Date(ingestedIso) : null;
  const embedComplete = chunks > 0 && embedded === chunks;
  return (
    <li className="py-4">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <span className="text-ink">{sourceKind}</span>
        <span
          className={
            d.visibility === "corpus_only" ? "text-signal" : "text-success"
          }
        >
          {d.visibility === "corpus_only" ? "corpus-only" : "public"}
        </span>
        <span>{sourcePath}</span>
        {ingested ? <span>added {relative(ingested, nowMs)}</span> : null}
      </div>
      <p className="mt-1 text-sm text-ink">{d.title}</p>
      <p className="mt-1 font-mono text-[11px] text-ink-3">
        chunks {chunks}
        <span className="text-ink-4"> · </span>
        embedded{" "}
        <span className={embedComplete ? "text-success" : "text-signal"}>
          {embedded}
        </span>
        {embedComplete ? null : (
          <span className="text-ink-3">
            {" "}
            (re-run when the sidecar embedder is up)
          </span>
        )}
      </p>
    </li>
  );
}

function getNowMs(): number {
  return Date.now();
}

function relative(d: Date, nowMs: number): string {
  const ms = nowMs - d.getTime();
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return "just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 48) return `${h}h ago`;
  const days = Math.round(h / 24);
  if (days < 30) return `${days}d ago`;
  if (days < 365) return `${Math.round(days / 30)}mo ago`;
  return `${Math.round(days / 365)}y ago`;
}
