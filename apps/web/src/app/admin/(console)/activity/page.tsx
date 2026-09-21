import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin — Activity" };
export const dynamic = "force-dynamic";

type MemberActivity = {
  user_id: string;
  name: string;
  email: string;
  status: string;
  total_sessions?: number;
  total_active_secs?: string; // int64 arrives as a string
  ask_roger_count?: number;
  total_events?: number;
  last_kind?: string;
  last_event_at?: string;
};

type ListResp = { members?: MemberActivity[] };

type FetchResult =
  | { ok: true; data: ListResp }
  | { ok: false; error: string };

const SORT_MAP: Record<string, string> = {
  name: "ACTIVITY_SORT_UNSPECIFIED",
  recent: "ACTIVITY_SORT_LAST_EVENT_DESC",
  sessions: "ACTIVITY_SORT_SESSIONS_DESC",
  active: "ACTIVITY_SORT_ACTIVE_TIME_DESC",
  chat: "ACTIVITY_SORT_ASK_ROGER_DESC",
};

async function fetchActivity(sortKey: string): Promise<FetchResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListMemberActivity",
    body: { sort: SORT_MAP[sortKey] ?? SORT_MAP.name },
    cookie,
  });
  if (!resp.ok) return { ok: false, error: `HTTP ${resp.status}` };
  return { ok: true, data: (await resp.json()) as ListResp };
}

const KIND_LABEL: Record<string, string> = {
  login: "signed in",
  logout: "signed out",
  view: "viewed",
  download: "downloaded",
  search: "searched",
  save: "saved",
  chat: "asked Roger",
  escalate: "escalated",
};

const STATUS_LABEL: Record<string, string> = {
  MEMBER_STATUS_UNVERIFIED: "unverified",
  MEMBER_STATUS_PENDING_APPROVAL: "pending",
  MEMBER_STATUS_ACTIVE: "active",
  MEMBER_STATUS_DECLINED: "declined",
  MEMBER_STATUS_EXPIRED: "expired",
  MEMBER_STATUS_DISABLED: "disabled",
};

const STATUS_TONE: Record<string, string> = {
  MEMBER_STATUS_ACTIVE: "text-success",
  MEMBER_STATUS_PENDING_APPROVAL: "text-signal",
  MEMBER_STATUS_DISABLED: "text-danger",
};

export default async function AdminActivityPage({
  searchParams,
}: {
  searchParams: Promise<{ sort?: string }>;
}) {
  const { sort } = await searchParams;
  const sortKey = sort && SORT_MAP[sort] ? sort : "recent";
  const result = await fetchActivity(sortKey);
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
        Activity.
      </h1>
      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        Per-member engagement summary — sortable by name, most-recent
        event, session count, total active time, or Ask Roger count.
        Click any row for the full activity timeline on that member.
      </p>

      <nav
        aria-label="Sort"
        className="mt-8 flex flex-wrap items-center gap-2 font-mono text-[11px] uppercase tracking-[0.14em]"
      >
        <span className="text-ink-3">sort:</span>
        <SortTab href="/admin/activity?sort=name" active={sortKey === "name"}>
          Name
        </SortTab>
        <SortTab
          href="/admin/activity?sort=recent"
          active={sortKey === "recent"}
        >
          Most recent
        </SortTab>
        <SortTab
          href="/admin/activity?sort=sessions"
          active={sortKey === "sessions"}
        >
          Sessions
        </SortTab>
        <SortTab
          href="/admin/activity?sort=active"
          active={sortKey === "active"}
        >
          Active time
        </SortTab>
        <SortTab href="/admin/activity?sort=chat" active={sortKey === "chat"}>
          Ask Roger
        </SortTab>
      </nav>

      {!result.ok ? (
        <p className="mt-10 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load activity — {result.error}.
        </p>
      ) : (result.data.members ?? []).length === 0 ? (
        <p className="mt-10 text-sm text-ink-3">
          No members recorded yet.
        </p>
      ) : (
        <div className="mt-8 overflow-x-auto border border-line">
          <table className="w-full min-w-full border-collapse text-sm">
            <thead>
              <tr className="border-b border-line-strong bg-paper-2 text-left font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                <th className="px-3 py-2">Member</th>
                <th className="px-3 py-2">Status</th>
                <th className="px-3 py-2 text-right tabular">Sessions</th>
                <th className="px-3 py-2 text-right tabular">Active time</th>
                <th className="px-3 py-2 text-right tabular">Ask Roger</th>
                <th className="px-3 py-2 text-right tabular">Total events</th>
                <th className="px-3 py-2">Last event</th>
              </tr>
            </thead>
            <tbody>
              {(result.data.members ?? []).map((m) => (
                <Row key={m.user_id} m={m} nowMs={nowMs} />
              ))}
            </tbody>
          </table>
        </div>
      )}
    </>
  );
}

function SortTab({
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
        "border px-3 py-1 no-underline transition-colors " +
        (active
          ? "border-accent bg-accent text-white"
          : "border-line-strong text-ink-2 hover:border-accent hover:text-accent")
      }
    >
      {children}
    </a>
  );
}

function Row({ m, nowMs }: { m: MemberActivity; nowMs: number }) {
  const statusCls = STATUS_TONE[m.status] ?? "text-ink-3";
  const lastAt = m.last_event_at ? new Date(m.last_event_at) : null;
  return (
    <tr className="border-b border-line last:border-b-0 odd:bg-paper-2/40">
      <td className="px-3 py-2 align-top">
        <Link
          href={`/admin/registrations/${m.user_id}`}
          className="text-ink no-underline hover:text-accent"
        >
          <span className="block">{m.name || "(no name)"}</span>
          <span className="block font-mono text-[11px] text-ink-3">
            {m.email}
          </span>
        </Link>
      </td>
      <td className={"px-3 py-2 align-top font-mono text-xs " + statusCls}>
        {STATUS_LABEL[m.status] ?? m.status}
      </td>
      <td className="px-3 py-2 align-top text-right tabular text-ink">
        {m.total_sessions ?? 0}
      </td>
      <td className="px-3 py-2 align-top text-right tabular text-ink">
        {formatDuration(Number(m.total_active_secs ?? "0"))}
      </td>
      <td className="px-3 py-2 align-top text-right tabular text-ink">
        {m.ask_roger_count ?? 0}
      </td>
      <td className="px-3 py-2 align-top text-right tabular text-ink-2">
        {m.total_events ?? 0}
      </td>
      <td className="px-3 py-2 align-top text-ink-2">
        {lastAt ? (
          <>
            <span className="text-ink">
              {KIND_LABEL[m.last_kind ?? ""] ?? m.last_kind ?? "—"}
            </span>{" "}
            <span className="text-ink-3">·</span>{" "}
            <span className="font-mono text-[11px]">
              {relative(lastAt, nowMs)}
            </span>
          </>
        ) : (
          <span className="text-ink-3">never</span>
        )}
      </td>
    </tr>
  );
}

function getNowMs(): number {
  return Date.now();
}

function relative(d: Date, nowMs: number): string {
  const ms = nowMs - d.getTime();
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 48) return `${h}h ago`;
  const days = Math.round(h / 24);
  if (days < 30) return `${days}d ago`;
  if (days < 365) return `${Math.round(days / 30)}mo ago`;
  return `${Math.round(days / 365)}y ago`;
}

function formatDuration(secs: number): string {
  if (!secs || secs < 0) return "—";
  if (secs < 60) return `${secs}s`;
  const m = Math.round(secs / 60);
  if (m < 60) return `${m}m`;
  const h = m / 60;
  if (h < 48) return `${h.toFixed(1)}h`;
  const days = h / 24;
  return `${days.toFixed(1)}d`;
}
