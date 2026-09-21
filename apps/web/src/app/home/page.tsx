import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";

import { OtPanel } from "@/components/ot-panel";
import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";
import { getUiMode } from "@/lib/ui-mode";

import { ActivityBeacon } from "./activity-beacon";
import { logoutAction } from "./actions";

export const metadata: Metadata = { title: "Member home" };
export const dynamic = "force-dynamic";

type Me = {
  id: string;
  name: string;
  email: string;
  status: string;
  role: string;
  organization?: string;
  stated_role?: string;
  statedRole?: string;
  created_at?: string;
  createdAt?: string;
  expires_at?: string;
  expiresAt?: string;
};

type ActivityEvent = {
  kind: string;
  content_id?: string;
  contentId?: string;
  occurred_at?: string;
  occurredAt?: string;
};

async function fetchMe(): Promise<Me | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.MemberService/GetMe",
    body: {},
    cookie,
  });
  if (!resp.ok) return null;
  const j = (await resp.json()) as { me?: Me };
  return j.me ?? null;
}

async function fetchHistory(): Promise<ActivityEvent[]> {
  const cookie = await getSessionCookie();
  if (!cookie) return [];
  const resp = await callApi({
    path: "/api/career.v1.MemberService/GetHistory",
    body: { page: { pageSize: 10 } },
    cookie,
  });
  if (!resp.ok) return [];
  const j = (await resp.json()) as { events?: ActivityEvent[] };
  return j.events ?? [];
}

const KIND_LABEL: Record<string, string> = {
  KIND_LOGIN: "signed in",
  KIND_LOGOUT: "signed out",
  KIND_VIEW: "viewed",
  KIND_DOWNLOAD: "downloaded",
  KIND_SEARCH: "searched",
  KIND_SAVE: "saved",
  KIND_CHAT: "asked Roger",
  KIND_ESCALATE: "escalated",
};

export default async function HomePage() {
  const [me, mode, history] = await Promise.all([
    fetchMe(),
    getUiMode(),
    fetchHistory(),
  ]);
  if (!me) redirect("/login");

  const firstName = me.name?.split(" ")[0] ?? "there";
  const expiresAtIso = me.expires_at ?? me.expiresAt;
  const expiresAt = expiresAtIso ? new Date(expiresAtIso) : null;
  const createdIso = me.created_at ?? me.createdAt;
  const memberSince = createdIso ? new Date(createdIso) : null;
  const statedRole = me.stated_role ?? me.statedRole;
  const nowMs = getNowMs();

  if (mode === "ot") {
    return (
      <OtPanel
        tag="MBR-01"
        title={`SESSION.ACTIVE · ${me.email}`}
        note={me.role === "MEMBER_ROLE_ADMIN" ? "role admin" : "role member"}
      >
        <ActivityBeacon contentId="/home" />
        <div className="flex flex-wrap items-baseline justify-between gap-4">
          <AccessPill expiresAt={expiresAt} nowMs={nowMs} tone="ot" />
          <form action={logoutAction} className="m-0">
            <button
              type="submit"
              className="border border-line-strong px-3 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-signal hover:text-signal"
            >
              [ SIGN OUT ]
            </button>
          </form>
        </div>
        <h1
          className="font-display mt-6 text-4xl leading-tight tracking-tight text-ink sm:text-5xl"
          style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
        >
          Welcome, <span className="italic text-accent">{firstName}</span>.
        </h1>

        <dl className="mt-10 grid gap-3 border-t border-line pt-6 font-mono text-[12px] text-ink-2 sm:grid-cols-[10rem_1fr]">
          <dt className="text-ink-3">STATUS</dt>
          <dd className="m-0 text-success">
            <span className="pilot mr-2 align-middle text-success" aria-hidden="true" />
            {me.status === "MEMBER_STATUS_ACTIVE" ? "ACTIVE" : me.status.replace("MEMBER_STATUS_", "")}
          </dd>
          <dt className="text-ink-3">EMAIL</dt>
          <dd className="m-0 text-ink">{me.email}</dd>
          {me.organization ? (
            <>
              <dt className="text-ink-3">ORG</dt>
              <dd className="m-0 text-ink">{me.organization}</dd>
            </>
          ) : null}
          {statedRole ? (
            <>
              <dt className="text-ink-3">ROLE</dt>
              <dd className="m-0 text-ink">{statedRole}</dd>
            </>
          ) : null}
          {memberSince ? (
            <>
              <dt className="text-ink-3">MEMBER SINCE</dt>
              <dd className="m-0 text-ink">{memberSince.toISOString().slice(0, 10)}</dd>
            </>
          ) : null}
          {expiresAt ? (
            <>
              <dt className="text-ink-3">EXPIRES</dt>
              <dd className="m-0 text-ink">
                {expiresAt.toISOString().slice(0, 10)}{" "}
                <span className="text-ink-3">· {formatDaysUntil(expiresAt, nowMs)}</span>
              </dd>
            </>
          ) : null}
        </dl>

        {history.length > 0 ? (
          <>
            <p className="mt-10 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              RECENT ACTIVITY
            </p>
            <ul className="mt-3 space-y-2 border-l border-line pl-3 font-mono text-[12px]">
              {history.slice(0, 5).map((e, i) => (
                <HistoryLine key={i} e={e} nowMs={nowMs} />
              ))}
            </ul>
          </>
        ) : null}

        <p className="mt-10 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          SURFACES
        </p>
        <ul className="mt-3 space-y-1 border-l border-line pl-3 font-mono text-[12px]">
          <li>
            <span className="text-signal">JD.INPUT</span>{" "}
            <span className="text-ink-3">·</span>{" "}
            <Link
              href="/jd-upload"
              className="text-accent no-underline hover:text-accent-hover"
            >
              /jd-upload
            </Link>{" "}
            <span className="text-ink-3">— paste a role, get a scored résumé</span>
          </li>
          <li>
            <span className="text-signal">ARTICLES</span>{" "}
            <span className="text-ink-3">·</span>{" "}
            <Link
              href="/articles"
              className="text-accent no-underline hover:text-accent-hover"
            >
              /articles
            </Link>{" "}
            <span className="text-ink-3">— five pieces from the corpus</span>
          </li>
          <li>
            <span className="text-ink-3">ASK.ROGER</span>{" "}
            <span className="text-ink-3">·</span>{" "}
            <span className="text-ink-3">available Q4 2026</span>
          </li>
        </ul>

        <p className="mt-10 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          questions?{" "}
          <a
            href="mailto:rogerhenley345@gmail.com"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            rogerhenley345@gmail.com
          </a>
        </p>
      </OtPanel>
    );
  }

  return (
    <div className="mx-auto max-w-4xl px-6 py-16 sm:px-10 sm:py-20">
      <ActivityBeacon contentId="/home" />
      <div className="mb-14 flex flex-wrap items-center justify-between gap-4 border-b border-line pb-6">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          signed in as <span className="text-ink">{me.email}</span>
        </p>
        <form action={logoutAction}>
          <button
            type="submit"
            className="rounded-md border border-line px-3 py-1.5 text-xs font-mono uppercase tracking-[0.14em] text-ink-2 transition-colors hover:border-accent hover:text-accent"
          >
            sign out
          </button>
        </form>
      </div>

      <AccessPill expiresAt={expiresAt} nowMs={nowMs} tone="it" />

      <h1
        className="font-display mt-4 text-5xl leading-[0.98] tracking-tight text-ink sm:text-6xl"
        style={{ fontVariationSettings: '"opsz" 144, "SOFT" 40' }}
      >
        Welcome, <span className="italic text-accent">{firstName}</span>.
      </h1>
      <p className="mt-8 max-w-2xl text-lg leading-relaxed text-ink-2">
        You&rsquo;re in. Two surfaces are live today; the third
        (Ask Roger) lands with Phase 4.
      </p>

      <section className="mt-14 grid gap-6 md:grid-cols-3">
        <LiveCard
          label="jd upload · live"
          title="Have a role in mind?"
          body="Paste a JD and Roger&rsquo;s pipeline scores fit against thirty years of manufacturing and applied-AI work. If the score clears the threshold, a two-page résumé tailored to that posting is generated."
          href="/jd-upload"
          cta="Upload a JD →"
        />
        <LiveCard
          label="articles · live"
          title="Roger's writing"
          body="Pieces on digital transformation, control-room adoption, plant-historian architecture, and why manufacturing AI programs die in the pilot."
          href="/articles"
          cta="Read the archive →"
        />
        <LiveCard
          label="gallery · live"
          title="On the floor"
          body="Control rooms, columns, tank farms, greenfield builds, and the desk the writing comes from. Every photo carries its context and the year."
          href="/gallery"
          cta="Open the gallery →"
        />
        <ComingSoonCard
          label="ask roger"
          title="Ask Roger anything"
          body="A retrieval-grounded assistant answering questions about Roger's career, projects, and how he thinks, with citations back to primary sources. Available Q4 2026."
        />
      </section>

      <section className="mt-14 grid gap-10 border-t border-line pt-10 md:grid-cols-[1fr_1.4fr]">
        <div>
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            your account
          </p>
          <h2
            className="font-display mt-3 text-2xl leading-snug text-ink"
            style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
          >
            Details on file.
          </h2>
        </div>
        <dl className="grid gap-x-6 gap-y-3 text-sm sm:grid-cols-[10rem_1fr]">
          <dt className="text-ink-3">Email</dt>
          <dd className="m-0 text-ink">{me.email}</dd>
          {me.organization ? (
            <>
              <dt className="text-ink-3">Organization</dt>
              <dd className="m-0 text-ink">{me.organization}</dd>
            </>
          ) : null}
          {statedRole ? (
            <>
              <dt className="text-ink-3">Role</dt>
              <dd className="m-0 text-ink">{statedRole}</dd>
            </>
          ) : null}
          {memberSince ? (
            <>
              <dt className="text-ink-3">Member since</dt>
              <dd className="m-0 text-ink">
                {memberSince.toISOString().slice(0, 10)}
              </dd>
            </>
          ) : null}
          {expiresAt ? (
            <>
              <dt className="text-ink-3">Access expires</dt>
              <dd className="m-0 text-ink">
                {expiresAt.toISOString().slice(0, 10)}
                <span className="ml-2 text-ink-3">
                  ({formatDaysUntil(expiresAt, nowMs)})
                </span>
              </dd>
            </>
          ) : null}
        </dl>
      </section>

      {history.length > 0 ? (
        <section className="mt-14 border-t border-line pt-10">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            recent activity
          </p>
          <ul className="mt-4 space-y-2 border-l border-line pl-4 text-sm">
            {history.slice(0, 8).map((e, i) => (
              <HistoryLine key={i} e={e} nowMs={nowMs} />
            ))}
          </ul>
        </section>
      ) : null}

      <p className="mt-14 text-sm text-ink-3">
        Questions or feedback? Reach me at{" "}
        <a
          href="mailto:rogerhenley345@gmail.com"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          rogerhenley345@gmail.com
        </a>
        .
      </p>
    </div>
  );
}

function AccessPill({
  expiresAt,
  nowMs,
  tone,
}: {
  expiresAt: Date | null;
  nowMs: number;
  tone: "it" | "ot";
}) {
  const base =
    tone === "ot"
      ? "font-mono text-[11px] uppercase tracking-[0.14em]"
      : "font-mono text-[11px] uppercase tracking-[0.14em]";
  if (!expiresAt) {
    return (
      <p className={base + " text-success"}>
        access · permanent
      </p>
    );
  }
  const days = daysUntil(expiresAt, nowMs);
  const tone2 =
    days < 0 ? "text-signal" : days <= 7 ? "text-signal" : "text-success";
  return (
    <p className={base + " " + tone2}>
      access · {formatDaysUntil(expiresAt, nowMs)}
    </p>
  );
}

function ComingSoonCard({
  label,
  title,
  body,
}: {
  label: string;
  title: string;
  body: string;
}) {
  return (
    <div className="border border-line p-5">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        {label} · coming soon
      </p>
      <p
        className="font-display mt-2 text-xl leading-snug text-ink"
        style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
      >
        {title}
      </p>
      <p className="mt-2 text-sm leading-relaxed text-ink-2">{body}</p>
    </div>
  );
}

function LiveCard({
  label,
  title,
  body,
  href,
  cta,
}: {
  label: string;
  title: string;
  body: string;
  href: string;
  cta: string;
}) {
  return (
    <Link
      href={href}
      className="group block border border-line p-5 no-underline transition-colors hover:border-accent"
    >
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        {label}
      </p>
      <p
        className="font-display mt-2 text-xl leading-snug text-ink transition-colors group-hover:text-accent"
        style={{ fontVariationSettings: '"opsz" 40, "SOFT" 50' }}
      >
        {title}
      </p>
      <p className="mt-2 text-sm leading-relaxed text-ink-2">{body}</p>
      <p className="mt-4 font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        {cta}
      </p>
    </Link>
  );
}

function HistoryLine({
  e,
  nowMs,
}: {
  e: ActivityEvent;
  nowMs: number;
}) {
  const at = e.occurred_at ?? e.occurredAt;
  const contentId = e.content_id ?? e.contentId;
  const when = at ? relative(new Date(at), nowMs) : "-";
  return (
    <li className="text-ink-2">
      <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        {when}
      </span>{" "}
      <span className="text-ink">
        {KIND_LABEL[e.kind] ?? e.kind.toLowerCase()}
      </span>
      {contentId ? (
        <>
          {" "}
          <span className="text-ink-3">·</span>{" "}
          <span className="font-mono text-xs">{contentId}</span>
        </>
      ) : null}
    </li>
  );
}

function getNowMs(): number {
  return Date.now();
}

function daysUntil(d: Date, nowMs: number): number {
  return Math.round((d.getTime() - nowMs) / 86_400_000);
}

function formatDaysUntil(d: Date, nowMs: number): string {
  const n = daysUntil(d, nowMs);
  if (n === 0) return "expires today";
  if (n > 0) {
    if (n === 1) return "1d remaining";
    if (n < 30) return `${n}d remaining`;
    if (n < 365) return `${Math.round(n / 30)}mo remaining`;
    return `${Math.round(n / 365)}y remaining`;
  }
  const ago = -n;
  if (ago === 1) return "expired 1d ago";
  if (ago < 30) return `expired ${ago}d ago`;
  return `expired ${Math.round(ago / 30)}mo ago`;
}

function relative(d: Date, nowMs: number): string {
  const ms = nowMs - d.getTime();
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return "just now";
  const m = Math.round(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.round(m / 60);
  if (h < 48) return `${h}h ago`;
  const days = Math.round(h / 24);
  if (days < 30) return `${days}d ago`;
  if (days < 365) return `${Math.round(days / 30)}mo ago`;
  return `${Math.round(days / 365)}y ago`;
}
