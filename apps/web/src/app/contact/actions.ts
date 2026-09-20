"use server";

import { create } from "@bufbuild/protobuf";
import { ConnectError } from "@connectrpc/connect";
import { cookies } from "next/headers";

import { SubmitContactRequestSchema, SupportCategory } from "@/gen/career/v1/contact_pb";
import { callApi } from "@/lib/api-fetch";
import { SESSION_COOKIE } from "@/lib/session";

export type ContactState = {
  ok?: boolean;
  ticketId?: string;
  error?: string;
  values?: {
    category?: string;
    subject?: string;
    message?: string;
    name?: string;
    email?: string;
  };
};

const CATEGORY_BY_KEY: Record<string, SupportCategory> = {
  general_question: SupportCategory.GENERAL_QUESTION,
  bug_report: SupportCategory.BUG_REPORT,
  feature_request: SupportCategory.FEATURE_REQUEST,
  contributor_access: SupportCategory.CONTRIBUTOR_ACCESS,
  press_inquiry: SupportCategory.PRESS_INQUIRY,
  other: SupportCategory.OTHER,
};

export async function submitContactAction(
  _prev: ContactState,
  formData: FormData,
): Promise<ContactState> {
  const category = String(formData.get("category") ?? "");
  const subject = String(formData.get("subject") ?? "").trim();
  const message = String(formData.get("message") ?? "").trim();
  const name = String(formData.get("name") ?? "").trim();
  const email = String(formData.get("email") ?? "").trim();
  const values = { category, subject, message, name, email };

  const categoryEnum = CATEGORY_BY_KEY[category];
  if (categoryEnum === undefined || categoryEnum === SupportCategory.UNSPECIFIED) {
    return { error: "Pick a category.", values };
  }
  if (!subject) return { error: "Subject is required.", values };
  if (!message) return { error: "Message is required.", values };
  if (message.length > 5000) return { error: "Message is over 5,000 characters.", values };

  // Signed-in visitors don't need name+email; the API pulls them from the session.
  const jar = await cookies();
  const cookie = jar.get(SESSION_COOKIE)?.value ?? null;
  if (!cookie) {
    if (!name) return { error: "Name is required.", values };
    if (!email) return { error: "Email is required.", values };
  }

  const req = create(SubmitContactRequestSchema, {
    subject,
    message,
    category: categoryEnum,
    name,
    email,
    replyChannel: 1, // REPLY_CHANNEL_EMAIL
  });

  // Serialize the message manually because callApi wants a JSON body.
  // ConnectRPC's default over HTTP+JSON matches the wire format we use.
  const body: Record<string, unknown> = {
    subject: req.subject,
    message: req.message,
    category: category.toUpperCase(),
    name: req.name,
    email: req.email,
    replyChannel: "REPLY_CHANNEL_EMAIL",
  };

  const resp = await callApi({
    path: "/api/career.v1.ContactService/SubmitContact",
    body,
    cookie,
  });

  if (!resp.ok) {
    let msg = "Sending failed. Try again in a minute.";
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) msg = j.message;
    } catch {
      // ignore; msg stays generic
    }
    return { error: msg, values };
  }

  const j = (await resp.json()) as { ticketId?: string };
  return { ok: true, ticketId: j.ticketId };
}

// Kept for callers that want the ConnectError sentinel; not currently used
// but the import is dead-code otherwise.
export const _connectErrorType = ConnectError;
