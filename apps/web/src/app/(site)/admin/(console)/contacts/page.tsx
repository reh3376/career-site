import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { ResolveButton } from "./resolve-button";

export const metadata: Metadata = { title: "Admin · Contact messages" };
export const dynamic = "force-dynamic";

// Shape mirrors the SupportMessage proto, snake_case as it arrives
// from the ConnectRPC JSON response, hand-rolled here rather than
// hooking the buf-generated client because the admin console is
// server-rendered and each page fetches with the caller's session
// cookie via callApi.
type SupportMessage = {
  id: string;
  ticket_id?: string;
  ticketId?: string;
  category: string;
  status: string;
  subject: string;
  body: string;
  sender_name?: string;
  senderName?: string;
  sender_email?: string;
  senderEmail?: string;
  user_id?: string;
  userId?: string;
  created_at?: string;
  createdAt?: string;
  updated_at?: string;
  updatedAt?: string;
  resolved_at?: string;
  resolvedAt?: string;
  hiring_role?: string;
  hiringRole?: string;
  hiring_jd_url?: string;
  hiringJdUrl?: string;
  hiring_target_start?: string;
  hiringTargetStart?: string;
};

type ListResp = {
  messages?: SupportMessage[];
  page?: { total_count?: number };
  open_count?: number;
  resolved_count?: number;
};

type FetchResult =
  | { ok: true; data: ListResp }
  | { ok: false; status: number; message: string };

async function fetchMessages(
  status?: "SUPPORT_STATUS_OPEN" | "SUPPORT_STATUS_RESOLVED",
): Promise<FetchResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, status: 401, message: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListContactMessages",
    body: {
      status,
      page: { pageSize: 50 },
    },
    cookie,
  });
  if (!resp.ok) {
    let message = `HTTP ${resp.status}`;
    try {
      const j = (await resp.json()) as { message?: string };
      if (j.message) message = j.message;
    } catch {
      /* keep default */
    }
    return { ok: false, status: resp.status, message };
  }
  return { ok: true, data: (await resp.json()) as ListResp };
}

const CATEGORY_LABEL: Record<string, string> = {
  SUPPORT_CATEGORY_GENERAL_QUESTION: "General question",
  SUPPORT_CATEGORY_HIRING_INQUIRY: "Hiring inquiry",
  SUPPORT_CATEGORY_BUG_REPORT: "Bug report",
  SUPPORT_CATEGORY_FEATURE_REQUEST: "Feature request",
  SUPPORT_CATEGORY_CONTRIBUTOR_ACCESS: "Contributor access",
  SUPPORT_CATEGORY_PRESS_INQUIRY: "Press / interview",
  SUPPORT_CATEGORY_OTHER: "Other",
};

export default async function AdminContactsPage({
  searchParams,
}: {
  searchParams: Promise<{ status?: string }>;
}) {
  const params = await searchParams;
  const filter =
    params.status === "resolved"
      ? "SUPPORT_STATUS_RESOLVED"
      : params.status === "all"
        ? undefined
        : "SUPPORT_STATUS_OPEN";
  const view = params.status === "resolved"
    ? "resolved"
    : params.status === "all"
      ? "all"
      : "open";

  const result = await fetchMessages(filter);

  if (!result.ok) {
    return (
      <>
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
          admin surface
        </p>
        <h1
          className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
          style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
        >
          Contact messages.
        </h1>
        <p className="mt-6 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load messages, {result.message}.
        </p>
      </>
    );
  }

  const { messages = [], open_count = 0, resolved_count = 0 } = result.data;
  const total = open_count + resolved_count;

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        admin surface
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Contact messages.
      </h1>

      <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-3">
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">open</dt>
          <dd className="m-0 tabular text-ink text-2xl">{open_count}</dd>
        </div>
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">resolved</dt>
          <dd className="m-0 tabular text-ink text-2xl">{resolved_count}</dd>
        </div>
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">total</dt>
          <dd className="m-0 tabular text-ink text-2xl">{total}</dd>
        </div>
      </dl>

      <nav
        aria-label="Filter"
        className="mt-6 flex items-center gap-3 font-mono text-[11px] uppercase tracking-[0.14em]"
      >
        <FilterTab href="/admin/contacts" active={view === "open"}>
          Open
        </FilterTab>
        <FilterTab
          href="/admin/contacts?status=resolved"
          active={view === "resolved"}
        >
          Resolved
        </FilterTab>
        <FilterTab href="/admin/contacts?status=all" active={view === "all"}>
          All
        </FilterTab>
      </nav>

      {messages.length === 0 ? (
        <p className="mt-10 text-sm text-ink-3">
          Nothing to show in this view.
        </p>
      ) : (
        <ul className="mt-8 divide-y divide-line border-y border-line">
          {messages.map((m) => (
            <MessageRow key={m.id} m={m} />
          ))}
        </ul>
      )}
    </>
  );
}

function FilterTab({
  href,
  active,
  children,
}: {
  href: string;
  active: boolean;
  children: React.ReactNode;
}) {
  return (
    <a
      href={href}
      className={
        "border px-3 py-1.5 no-underline transition-colors " +
        (active
          ? "border-accent bg-accent text-white"
          : "border-line-strong text-ink-2 hover:border-accent hover:text-accent")
      }
    >
      {children}
    </a>
  );
}

function MessageRow({ m }: { m: SupportMessage }) {
  const createdIso = m.created_at ?? m.createdAt;
  const created = createdIso ? new Date(createdIso) : null;
  const when = created ? formatWhen(created) : "-";
  const resolved = m.status === "SUPPORT_STATUS_RESOLVED";
  const ticket = m.ticket_id ?? m.ticketId ?? "";
  const senderName = m.sender_name ?? m.senderName ?? "";
  const senderEmail = m.sender_email ?? m.senderEmail ?? "";
  const hiringRole = m.hiring_role ?? m.hiringRole ?? "";
  const hiringUrl = m.hiring_jd_url ?? m.hiringJdUrl ?? "";
  const hiringStart = m.hiring_target_start ?? m.hiringTargetStart ?? "";
  const isHiring = m.category === "SUPPORT_CATEGORY_HIRING_INQUIRY";
  return (
    <li className="py-5">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <span className="text-ink">{ticket}</span>
        <span className={isHiring ? "text-accent" : ""}>
          {CATEGORY_LABEL[m.category] ?? m.category}
        </span>
        <span>{when}</span>
        {resolved ? (
          <span className="text-success">· resolved</span>
        ) : (
          <span className="text-signal">· open</span>
        )}
      </div>
      <p
        className="font-display mt-2 text-lg leading-snug text-ink"
        style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
      >
        {m.subject}
      </p>
      <p className="mt-1 text-sm text-ink-2">
        <span className="text-ink">{senderName}</span>{" "}
        <a
          href={`mailto:${senderEmail}?subject=Re%3A%20${encodeURIComponent(
            m.subject,
          )}%20%5B${ticket}%5D`}
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          &lt;{senderEmail}&gt;
        </a>
      </p>
      {isHiring && (hiringRole || hiringUrl || hiringStart) ? (
        <dl className="mt-3 grid gap-1 border-l-2 border-accent bg-accent-soft/30 py-2 pl-3 pr-2 font-mono text-[11px] text-ink-2 sm:grid-cols-[7rem_1fr]">
          {hiringRole ? (
            <>
              <dt className="uppercase tracking-[0.14em] text-ink-3">role</dt>
              <dd className="m-0 text-ink">{hiringRole}</dd>
            </>
          ) : null}
          {hiringStart ? (
            <>
              <dt className="uppercase tracking-[0.14em] text-ink-3">
                target start
              </dt>
              <dd className="m-0 text-ink">{hiringStart}</dd>
            </>
          ) : null}
          {hiringUrl ? (
            <>
              <dt className="uppercase tracking-[0.14em] text-ink-3">jd</dt>
              <dd className="m-0">
                <a
                  href={hiringUrl}
                  target="_blank"
                  rel="noopener noreferrer"
                  className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
                >
                  {hiringUrl}
                </a>
              </dd>
            </>
          ) : null}
        </dl>
      ) : null}
      <p className="mt-3 whitespace-pre-wrap text-sm leading-relaxed text-ink-2">
        {m.body}
      </p>
      <div className="mt-4">
        <ResolveButton
          id={m.id}
          currentlyResolved={resolved}
          ticketId={ticket}
        />
      </div>
    </li>
  );
}

function formatWhen(d: Date): string {
  const days = Math.max(0, Math.round((Date.now() - d.getTime()) / 86_400_000));
  if (days < 1) return "today";
  if (days === 1) return "1d ago";
  if (days < 7) return `${days}d ago`;
  if (days < 30) return `${Math.round(days / 7)}w ago`;
  return `${Math.round(days / 30)}mo ago`;
}
