"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export type ReindexResult =
  | {
      ok: true;
      root: string;
      files_scanned: number;
      docs_ingested: number;
      docs_skipped: number;
      chunks_inserted: number;
      chunks_embedded: number;
      errors: string[];
    }
  | { ok: false; error: string };

export async function reindexCorpusAction(
  sourceKind: string,
): Promise<ReindexResult> {
  const kind = sourceKind.trim();
  if (!kind) return { ok: false, error: "source_kind is required." };

  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in." };

  const resp = await callApi({
    path: "/api/career.v1.AdminService/ReindexCorpus",
    body: { sourceKind: kind },
    cookie,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep default */
    }
    return { ok: false, error: msg };
  }
  const j = (await resp.json()) as {
    root?: string;
    filesScanned?: number;
    files_scanned?: number;
    docsIngested?: number;
    docs_ingested?: number;
    docsSkipped?: number;
    docs_skipped?: number;
    chunksInserted?: number;
    chunks_inserted?: number;
    chunksEmbedded?: number;
    chunks_embedded?: number;
    errors?: string[];
  };
  revalidatePath("/admin/corpus", "layout");
  return {
    ok: true,
    root: j.root ?? "",
    files_scanned: j.filesScanned ?? j.files_scanned ?? 0,
    docs_ingested: j.docsIngested ?? j.docs_ingested ?? 0,
    docs_skipped: j.docsSkipped ?? j.docs_skipped ?? 0,
    chunks_inserted: j.chunksInserted ?? j.chunks_inserted ?? 0,
    chunks_embedded: j.chunksEmbedded ?? j.chunks_embedded ?? 0,
    errors: j.errors ?? [],
  };
}

export type IngestResult =
  | {
      ok: true;
      document_id: string;
      chunks_inserted: number;
      chunks_embedded: number;
      skipped: boolean;
      chunker_name: string;
      embedder_model: string;
    }
  | { ok: false; error: string };

// Server action for the paste-a-document form on /admin/corpus.
// Chunk + embed happens server-side; UI just displays the result.
export async function ingestCorpusTextAction(
  formData: FormData,
): Promise<IngestResult> {
  const sourceKind = String(formData.get("source_kind") ?? "").trim();
  const sourcePath = String(formData.get("source_path") ?? "").trim();
  const title = String(formData.get("title") ?? "").trim();
  const body = String(formData.get("body") ?? "");

  if (!sourceKind || !sourcePath) {
    return { ok: false, error: "source_kind and source_path are required." };
  }
  if (body.trim().length < 10) {
    return { ok: false, error: "Body must be at least 10 characters." };
  }
  if (body.length > 204800) {
    return {
      ok: false,
      error: "Body is over the 200 KiB cap — split into smaller documents.",
    };
  }

  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in." };

  const resp = await callApi({
    path: "/api/career.v1.AdminService/IngestCorpusText",
    body: {
      sourceKind,
      sourcePath,
      title,
      body,
    },
    cookie,
  });
  if (!resp.ok) {
    let msg = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      /* keep default */
    }
    return { ok: false, error: msg };
  }
  const j = (await resp.json()) as {
    documentId?: string;
    document_id?: string;
    chunksInserted?: number;
    chunks_inserted?: number;
    chunksEmbedded?: number;
    chunks_embedded?: number;
    skipped?: boolean;
    chunkerName?: string;
    chunker_name?: string;
    embedderModel?: string;
    embedder_model?: string;
  };
  revalidatePath("/admin/corpus", "layout");
  return {
    ok: true,
    document_id: j.documentId ?? j.document_id ?? "",
    chunks_inserted: j.chunksInserted ?? j.chunks_inserted ?? 0,
    chunks_embedded: j.chunksEmbedded ?? j.chunks_embedded ?? 0,
    skipped: Boolean(j.skipped),
    chunker_name: j.chunkerName ?? j.chunker_name ?? "",
    embedder_model: j.embedderModel ?? j.embedder_model ?? "",
  };
}
