"use server";

import { cookies } from "next/headers";

import { callApi } from "@/lib/api-fetch";
import { SESSION_COOKIE } from "@/lib/session";

// Feedback from the thanks page, sent through the same inbox as the
// contact form.
//
// A separate action rather than a branch inside submitContactAction,
// because almost every rule differs: no category to pick, no subject to
// write, no name or email required, and a run id to carry. Folding it
// in would mean four conditionals in a function whose job is already
// the general case.
export type FeedbackState = { ok?: boolean; error?: string };

export async function submitFeedbackAction(
  message: string,
  name: string,
  email: string,
  dtSessionPublicId: string,
  turnstileToken: string,
): Promise<FeedbackState> {
  const body = message.trim();
  if (!body) return { error: "There is nothing in the box yet." };
  if (body.length > 5000) {
    return { error: `That is ${body.length} characters, and the limit is 5,000.` };
  }

  // The subject is the start of the note, not the category again.
  //
  // It was a fixed "Decision test feedback", which is also what the
  // category renders as, so the notification email printed the same
  // five words twice, one above the other, and every row in the inbox
  // looked identical to every other. The first line of what somebody
  // wrote is the one string that distinguishes these from each other.
  const firstLine = body.split("\n")[0].trim();
  const subject =
    firstLine.length > 72 ? `${firstLine.slice(0, 69)}...` : firstLine;

  const jar = await cookies();
  const cookie = jar.get(SESSION_COOKIE)?.value ?? null;

  const resp = await callApi({
    path: "/api/career.v1.ContactService/SubmitContact",
    body: {
      subject,
      message: body,
      category: "SUPPORT_CATEGORY_DECISION_TEST_FEEDBACK",
      // Both may be empty, and the API allows that for this category
      // only. The test tells participants the measurement does not
      // need their name; demanding one before they can say the test
      // was hard would contradict the page this arrives from.
      name: name.trim(),
      email: email.trim(),
      replyChannel: "REPLY_CHANNEL_EMAIL",
      // Verified server-side against dt_sessions before it is stored,
      // so a note carrying a run id is evidence the sender finished a
      // run rather than a claim that they did.
      dtSessionPublicId: dtSessionPublicId.trim(),
      turnstileToken,
    },
    cookie,
  });

  if (!resp.ok) {
    let msg = "That did not send. Nothing was lost, so try again.";
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      // keep the generic message
    }
    return { error: msg };
  }
  return { ok: true };
}
