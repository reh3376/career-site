"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// Changing a timing changes the instrument, and the instrument version
// is derived from the timings rather than kept beside them, so there is
// no version to forget to bump. Every run after a change records the
// new one, and runs under different timings never pool into one dataset.
export async function setDecisionTestTimingsAction(formData: FormData): Promise<void> {
  const cookie = await getSessionCookie();
  if (!cookie) return;
  const sec = (name: string) => Math.round(Number(formData.get(name) ?? 0) * 1000);
  await callApi({
    path: "/api/career.v1.AdminService/SetDecisionTestSettings",
    body: {
      memoriseMs: sec("memorise_s"),
      questionMs: sec("question_s"),
      recallMs: sec("recall_s"),
    },
    cookie,
  });
  revalidatePath("/admin/decision-test");
}

// The dataset, as a file.
//
// Returns the bytes rather than writing them anywhere: the action runs
// on the server and only the browser can offer a download, so
// export.tsx turns this into a saved file.
export async function exportDecisionTestAction(
  includeSynthetic: boolean,
): Promise<{ filename: string; csv: string; rows: number; sessions: number } | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const r = await callApi({
    path: "/api/career.v1.AdminService/ExportDecisionTestData",
    body: { includeSynthetic },
    cookie,
  });
  // callApi returns the Response, not the body. Parsing it is not
  // optional and the cast is not the parse, which is the mistake that
  // made this page show zeroes on 2026-10-04.
  if (!r.ok) return null;
  const body = (await r.json()) as {
    filename?: string;
    csv?: string;
    rows?: number;
    sessions?: number;
  };
  return {
    filename: body.filename ?? "decision-test.csv",
    csv: body.csv ?? "",
    rows: body.rows ?? 0,
    sessions: body.sessions ?? 0,
  };
}

// Curation. One action for both levels, because the RPC is one call and
// the difference is a block number.
//
// revalidatePath on both the detail page and the list: the list carries
// the status tags and the unreviewed count, and a review that did not
// change the queue would read as not having saved.
export async function reviewDecisionTestRunAction(formData: FormData): Promise<void> {
  const cookie = await getSessionCookie();
  if (!cookie) return;
  const key = String(formData.get("session_key") ?? "");
  if (!key) return;

  const blockNo = Number(formData.get("block_no") ?? 0);
  const status = String(formData.get("status") ?? "");
  // A reason only means anything against an exclusion. Sending one with
  // "good" is refused by the api, so it is dropped here rather than
  // turned into an error the reviewer has to read.
  const reason = status === "do_not_use" ? String(formData.get("reason") ?? "") : "";

  await callApi({
    path: "/api/career.v1.AdminService/ReviewDecisionTestRun",
    body: {
      sessionKey: key,
      blockNo,
      status,
      reason,
      note: String(formData.get("note") ?? ""),
    },
    cookie,
  });
  revalidatePath(`/admin/decision-test/${key}`);
  revalidatePath("/admin/decision-test");
}

// One run, as the two files its page shows.
//
// Same shape as exportDecisionTestAction: the action fetches the bytes
// and the client component turns them into saved files, because a
// server action cannot hand the browser a download.
export async function exportDecisionTestRunAction(
  sessionKey: string,
  includeKey: boolean,
): Promise<{
  filenameStem: string;
  blocksCsv: string;
  answersCsv: string;
  blockRows: number;
  answerRows: number;
} | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const r = await callApi({
    path: "/api/career.v1.AdminService/ExportDecisionTestRun",
    body: { sessionKey, includeKey },
    cookie,
  });
  if (!r.ok) return null;
  const body = (await r.json()) as {
    filenameStem?: string;
    blocksCsv?: string;
    answersCsv?: string;
    blockRows?: number;
    answerRows?: number;
  };
  return {
    filenameStem: body.filenameStem ?? "decision-test-run",
    blocksCsv: body.blocksCsv ?? "",
    answersCsv: body.answersCsv ?? "",
    blockRows: body.blockRows ?? 0,
    answerRows: body.answerRows ?? 0,
  };
}
