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
