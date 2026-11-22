"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import type { Report } from "./report-state";

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

// The run's results report (FSD FR-DT-23).
//
// Three actions, each returning what the panel needs to say next rather
// than a bare success: a refusal from the api carries a reason the owner
// can act on ("the participant did not ask for their results"), and
// turning that into "something went wrong" would hide the one sentence
// that explains the button.

type ReportResult = { report?: Report; error?: string };

async function refusal(r: Response, fallback: string): Promise<string> {
  try {
    const body = (await r.json()) as { message?: string };
    if (body.message) return body.message.charAt(0).toUpperCase() + body.message.slice(1) + ".";
  } catch {
    // A body that is not JSON says nothing useful; the fallback does.
  }
  return fallback;
}

// One copy, as bytes for the browser to save. Base64 because that is how
// the api's JSON carries a PDF, and the client component decodes it: a
// server action can return bytes but cannot hand the browser a file.
export async function downloadDecisionTestReportAction(
  sessionKey: string,
  owner: boolean,
): Promise<{ filename: string; pdfBase64: string; version: number } | { error: string }> {
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "You are signed out." };
  const r = await callApi({
    path: "/api/career.v1.AdminService/DownloadDecisionTestReport",
    body: {
      sessionKey,
      copy: owner ? "DECISION_TEST_REPORT_COPY_OWNER" : "DECISION_TEST_REPORT_COPY_PARTICIPANT",
    },
    cookie,
  });
  if (!r.ok) return { error: await refusal(r, "The report did not come back. Check the api logs.") };
  const body = (await r.json()) as { filename?: string; pdf?: string; version?: number };
  if (!body.pdf) return { error: "The report came back empty." };
  return {
    filename: body.filename ?? "decision-test-results.pdf",
    pdfBase64: body.pdf,
    version: body.version ?? 0,
  };
}

// A new version, against whoever has finished by now. Sends nothing.
export async function generateDecisionTestReportAction(sessionKey: string): Promise<ReportResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "You are signed out." };
  const r = await callApi({
    path: "/api/career.v1.AdminService/GenerateDecisionTestReport",
    body: { sessionKey },
    cookie,
  });
  if (!r.ok) return { error: await refusal(r, "The report could not be built. Check the api logs.") };
  const body = (await r.json()) as { report?: Report };
  revalidatePath(`/admin/decision-test/${sessionKey}`);
  return { report: body.report };
}

// The participant's copy, to the participant. The api refuses unless
// they asked for results and left an address.
export async function sendDecisionTestReportAction(sessionKey: string): Promise<ReportResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "You are signed out." };
  const r = await callApi({
    path: "/api/career.v1.AdminService/SendDecisionTestReport",
    body: { sessionKey },
    cookie,
  });
  if (!r.ok) return { error: await refusal(r, "The email could not be sent. Check the api logs.") };
  const body = (await r.json()) as { report?: Report };
  revalidatePath(`/admin/decision-test/${sessionKey}`);
  return { report: body.report };
}
