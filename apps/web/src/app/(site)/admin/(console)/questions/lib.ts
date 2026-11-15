// The clarification loop as the web reads it (FSD FR-HF-13, 14, 22):
// the shapes HiringService sends for a questionnaire, and the few
// sentences both question pages share. Connect's JSON leaves out a
// field at its default, so every field is optional.

export type ApiOption = { value?: string; label?: string; source?: string };

export type ApiField = {
  name?: string;
  label?: string;
  // choice, multi, number, text
  kind?: string;
  required?: boolean;
  options?: ApiOption[];
  max?: number;
  maxLen?: number;
  hint?: string;
};

export type ApiAnswer = {
  // pending, answered, not_applicable, declined, withdrawn
  status?: string;
  values?: Record<string, { values?: string[] }>;
  freeText?: string;
};

export type ApiItem = {
  id?: string;
  requirementLocalId?: string;
  sourceQuote?: string;
  answerType?: string;
  question?: string;
  fields?: ApiField[];
  freeTextRequired?: boolean;
  freeTextMax?: number;
  answer?: ApiAnswer;
  answeredAt?: string;
  statement?: string;
};

export type ApiRequest = {
  id?: string;
  submissionId?: string;
  runId?: string;
  attempt?: number;
  roleTitle?: string;
  employer?: string;
  // sent, answered, expired
  status?: string;
  held?: boolean;
  askedBy?: string;
  generatedAt?: string;
  expiresAt?: string;
  answeredAt?: string;
  closedAt?: string;
  closedReason?: string;
  itemCount?: number;
  pendingCount?: number;
  items?: ApiItem[];
};

export function when(iso: string | undefined): string {
  if (!iso) return "";
  const d = new Date(iso);
  return Number.isNaN(d.getTime()) ? "" : `${d.toISOString().slice(0, 16).replace("T", " ")} UTC`;
}

export function plural(n: number, one: string, many: string): string {
  return n === 1 ? one : many;
}

// Where a questionnaire stands, in a few words and in a sentence.
export function requestState(q: ApiRequest): { label: string; tone: string; sentence: string } {
  const pending = q.pendingCount ?? 0;
  const asked = q.itemCount ?? 0;
  if (q.status === "sent") {
    const limit = when(q.expiresAt);
    return {
      label: pending > 0 ? `${pending} of ${asked} open` : "ready to submit",
      tone: "text-signal",
      sentence: q.held
        ? `No fit is given for the posting while a question is open.${
            limit ? ` Any question still open at ${limit} is closed unanswered, and the fit is then issued from what is evidenced.` : ""
          }`
        : "These were asked at your own request. The posting keeps the fit it was given until the answers are submitted.",
    };
  }
  if (q.status === "answered") {
    return {
      label: q.closedAt ? "answered" : "answered · being judged",
      tone: "text-ink-2",
      sentence: q.closedAt
        ? `Submitted ${when(q.answeredAt)}. The requirements the answers speak to were judged again.`
        : `Submitted ${when(q.answeredAt)}. The requirements the answers speak to are being judged again.`,
    };
  }
  if (q.closedReason === "superseded") {
    return {
      label: "replaced by a new run",
      tone: "text-ink-3",
      sentence: "A later run of the posting replaced the record these were asked about. Answers already given still stand as evidence.",
    };
  }
  return {
    label: "closed · time limit passed",
    tone: "text-ink-3",
    sentence: "The time for answering passed. Answers already given were used; the rest closed unanswered.",
  };
}
