import type { Metadata } from "next";

import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin — Registrations" };
export const dynamic = "force-dynamic";

// Read-only member list. Approve / decline still happen through the
// one-click email flow (/admin/decision) — that path is well-tested
// and sends the sign-in email as part of the transition. This surface
// is the durable record you can review outside of Gmail.

type Me = {
  id: string;
  name: string;
  email: string;
  status: string;
  role: string;
};

type MemberRecord = {
  me?: Me;
  first_seen_at?: string;
};

type ListResp = {
  members?: MemberRecord[];
  page?: { total_count?: number };
};

type FetchResult =
  | { ok: true; data: ListResp }
  | { ok: false; status: number; message: string };

const STATUS_MAP = {
  unverified: "MEMBER_STATUS_UNVERIFIED",
  pending: "MEMBER_STATUS_PENDING_APPROVAL",
  active: "MEMBER_STATUS_ACTIVE",
  declined: "MEMBER_STATUS_DECLINED",
  expired: "MEMBER_STATUS_EXPIRED",
  disabled: "MEMBER_STATUS_DISABLED",
} as const;

type StatusKey = keyof typeof STATUS_MAP;

async function fetchMembers(status?: string): Promise<FetchResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, status: 401, message: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListMembers",
    body: { status, page: { pageSize: 100 } },
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

const STATUS_LABEL: Record<string, string> = {
  MEMBER_STATUS_UNVERIFIED: "unverified",
  MEMBER_STATUS_PENDING_APPROVAL: "pending",
  MEMBER_STATUS_ACTIVE: "active",
  MEMBER_STATUS_DECLINED: "declined",
  MEMBER_STATUS_EXPIRED: "expired",
  MEMBER_STATUS_DISABLED: "disabled",
};

const STATUS_TONE: Record<string, string> = {
  MEMBER_STATUS_PENDING_APPROVAL: "text-signal",
  MEMBER_STATUS_ACTIVE: "text-success",
  MEMBER_STATUS_UNVERIFIED: "text-ink-3",
  MEMBER_STATUS_DECLINED: "text-ink-3",
  MEMBER_STATUS_EXPIRED: "text-ink-3",
  MEMBER_STATUS_DISABLED: "text-danger",
};

export default async function AdminRegistrationsPage({
  searchParams,
}: {
  searchParams: Promise<{ status?: string }>;
}) {
  const params = await searchParams;
  const view: StatusKey | "all" =
    (Object.keys(STATUS_MAP) as StatusKey[]).find((k) => k === params.status) ??
    (params.status === "all" ? "all" : "pending");
  const statusFilter = view === "all" ? undefined : STATUS_MAP[view];

  const result = await fetchMembers(statusFilter);

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
          Registrations.
        </h1>
        <p className="mt-6 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load members — {result.message}.
        </p>
      </>
    );
  }

  const members = result.data.members ?? [];
  const total = result.data.page?.total_count ?? members.length;

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        admin surface
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Registrations.
      </h1>
      <p className="mt-6 max-w-xl text-sm leading-relaxed text-ink-3">
        Every registration and its state. Click a row for the detail
        view where you can approve, decline, disable, or re-enable —
        state-appropriate actions live there so this list stays
        scannable.
      </p>

      <nav
        aria-label="Filter"
        className="mt-8 flex flex-wrap items-center gap-2 font-mono text-[11px] uppercase tracking-[0.14em]"
      >
        {(Object.keys(STATUS_MAP) as StatusKey[]).map((k) => (
          <FilterTab
            key={k}
            href={`/admin/registrations?status=${k}`}
            active={view === k}
          >
            {k}
          </FilterTab>
        ))}
        <FilterTab href="/admin/registrations?status=all" active={view === "all"}>
          all
        </FilterTab>
      </nav>

      <p className="mt-6 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        showing{" "}
        <span className="tabular text-ink">{members.length}</span>
        {" of "}
        <span className="tabular text-ink">{total}</span>
        {" · "}filter <span className="text-ink">{view}</span>
      </p>

      {members.length === 0 ? (
        <p className="mt-10 text-sm text-ink-3">
          Nothing to show in this view.
        </p>
      ) : (
        <ul className="mt-6 divide-y divide-line border-y border-line">
          {members.map((m, i) => (
            <MemberRow
              key={m.me?.id ?? m.me?.email ?? `row-${i}`}
              m={m}
            />
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

function MemberRow({ m }: { m: MemberRecord }) {
  const me = m.me;
  if (!me) return null;
  const status = me.status ?? "";
  const label = STATUS_LABEL[status] ?? status;
  const tone = STATUS_TONE[status] ?? "text-ink-2";
  const seen = m.first_seen_at ? new Date(m.first_seen_at) : null;
  const when = seen ? formatWhen(seen) : "—";
  return (
    <li>
      <Link
        href={`/admin/registrations/${me.id}`}
        className="grid gap-2 py-4 no-underline transition-colors hover:bg-accent-soft/40 sm:grid-cols-[minmax(0,1fr)_minmax(0,1fr)_120px_120px] sm:items-baseline sm:gap-6"
      >
        <div className="min-w-0">
          <p className="truncate text-sm text-ink">{me.name || "—"}</p>
          <p className="truncate text-xs text-ink-3">{me.email}</p>
        </div>
        <div className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          <span className="text-ink">id</span> {me.id}
          <span className="mx-2 text-ink-4">·</span>
          <span className="text-ink">role</span>{" "}
          {me.role === "MEMBER_ROLE_ADMIN" ? "admin" : "member"}
        </div>
        <div className={"font-mono text-[11px] uppercase tracking-[0.14em] " + tone}>
          <span className="pilot mr-2 align-middle" aria-hidden="true" />
          {label}
        </div>
        <div className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          {when}
        </div>
      </Link>
    </li>
  );
}

function formatWhen(d: Date): string {
  const days = Math.max(0, Math.round((Date.now() - d.getTime()) / 86_400_000));
  if (days < 1) return "today";
  if (days === 1) return "1d ago";
  if (days < 7) return `${days}d ago`;
  if (days < 30) return `${Math.round(days / 7)}w ago`;
  if (days < 365) return `${Math.round(days / 30)}mo ago`;
  return `${Math.round(days / 365)}y ago`;
}
