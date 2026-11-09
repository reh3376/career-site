import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { revokeAgentPasswordAction, revokeAgentSessionsAction } from "./actions";
import { CreatePanel } from "./create-panel";

export const metadata: Metadata = { title: "Admin · Agent access" };
export const dynamic = "force-dynamic";

// Mirrors AgentPassword in wire form (camelCase from the ConnectRPC
// JSON codec; the snake spellings are tolerated for older fixtures).
type AgentPassword = {
  id: string;
  createdAt?: string;
  created_at?: string;
  expiresAt?: string;
  expires_at?: string;
  usedAt?: string;
  used_at?: string;
  revokedAt?: string;
  revoked_at?: string;
  createdBy?: string;
  reason?: string;
  state?: string;
  sessionLive?: boolean;
  session_live?: boolean;
};

type ListResp = {
  passwords?: AgentPassword[];
  liveSessions?: number;
  live_sessions?: number;
  username?: string;
};

type FetchResult =
  | { ok: true; data: ListResp }
  | { ok: false; message: string };

async function fetchList(): Promise<FetchResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, message: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListAgentPasswords",
    body: {},
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
    return { ok: false, message };
  }
  return { ok: true, data: (await resp.json()) as ListResp };
}

const STATE_TONE: Record<string, string> = {
  live: "text-success",
  used: "text-ink-2",
  expired: "text-ink-3",
  revoked: "text-signal",
};

export default async function AdminAgentPage() {
  const result = await fetchList();

  if (!result.ok) {
    return (
      <>
        <Header />
        <p className="mt-6 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load the agent account, {result.message}.
        </p>
      </>
    );
  }

  const { passwords = [], username = "agent@345" } = result.data;
  const liveSessions = result.data.liveSessions ?? result.data.live_sessions ?? 0;
  const live = passwords.filter((p) => p.state === "live");

  return (
    <>
      <Header />

      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        An agent signs in with the username{" "}
        <code className="font-mono text-ink">{username}</code> and a
        password you create here. A password is good for one sign-in
        and for 15 minutes from creation; creating a new one revokes
        the old, so at most one works at a time. The session it mints
        lasts hours, not the member&rsquo;s thirty days, and you can end
        it below. Every password, its use and its session are recorded.
      </p>

      <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-3">
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">live password</dt>
          <dd className="m-0 tabular text-ink text-2xl">{live.length}</dd>
        </div>
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">live sessions</dt>
          <dd className="m-0 tabular text-ink text-2xl">{liveSessions}</dd>
        </div>
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">created</dt>
          <dd className="m-0 tabular text-ink text-2xl">{passwords.length}</dd>
        </div>
      </dl>

      <section aria-label="Create a one-time password" className="mt-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          create
        </p>
        <div className="mt-3">
          <CreatePanel username={username} />
        </div>
      </section>

      <section aria-label="Live sessions" className="mt-10">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          sessions
        </p>
        <div className="mt-3 flex flex-wrap items-center gap-4">
          <p className="text-sm text-ink-2">
            {liveSessions === 0
              ? "No agent session is live."
              : liveSessions === 1
                ? "One agent session is live."
                : `${liveSessions} agent sessions are live.`}
          </p>
          {liveSessions > 0 ? (
            <form action={revokeAgentSessionsAction} className="m-0 flex items-center gap-2">
              <input
                type="text"
                name="reason"
                maxLength={1000}
                placeholder="reason (optional)"
                className="border border-line-strong bg-canvas px-2 py-1 font-mono text-xs text-ink outline-none focus:border-accent"
              />
              <button
                type="submit"
                className="inline-flex h-[30px] items-center border border-signal px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-signal transition-colors hover:bg-signal hover:text-white"
              >
                End agent sessions
              </button>
            </form>
          ) : null}
        </div>
      </section>

      <section aria-label="Passwords" className="mt-12">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          passwords · newest first
        </p>
        {passwords.length === 0 ? (
          <p className="mt-6 text-sm text-ink-3">None created yet.</p>
        ) : (
          <ul className="mt-4 divide-y divide-line border-y border-line">
            {passwords.map((p) => (
              <PasswordRow key={p.id} p={p} />
            ))}
          </ul>
        )}
      </section>
    </>
  );
}

function Header() {
  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        admin surface
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Agent access.
      </h1>
    </>
  );
}

function fmt(iso?: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return "-";
  return d.toISOString().slice(0, 16).replace("T", " ") + " UTC";
}

function PasswordRow({ p }: { p: AgentPassword }) {
  const state = p.state ?? "";
  const tone = STATE_TONE[state] ?? "text-ink-2";
  const sessionLive = p.sessionLive ?? p.session_live ?? false;
  return (
    <li className="py-4">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <span className={tone}>{state || "unknown"}</span>
        <span>created · {fmt(p.createdAt ?? p.created_at)}</span>
        <span>expires · {fmt(p.expiresAt ?? p.expires_at)}</span>
        {p.usedAt ?? p.used_at ? (
          <span className="text-ink-2">
            used · {fmt(p.usedAt ?? p.used_at)}
            {sessionLive ? " · session live" : ""}
          </span>
        ) : null}
        {p.revokedAt ?? p.revoked_at ? (
          <span className="text-ink-2">revoked · {fmt(p.revokedAt ?? p.revoked_at)}</span>
        ) : null}
      </div>
      {p.reason ? <p className="mt-1 text-sm text-ink-2">{p.reason}</p> : null}
      {state === "live" ? (
        <form action={revokeAgentPasswordAction} className="m-0 mt-3 flex items-center gap-2">
          <input type="hidden" name="id" value={p.id} />
          <input
            type="text"
            name="reason"
            maxLength={1000}
            placeholder="reason (optional)"
            className="border border-line-strong bg-canvas px-2 py-1 font-mono text-xs text-ink outline-none focus:border-accent"
          />
          <button
            type="submit"
            className="inline-flex h-[30px] items-center border border-signal px-3 font-mono text-[11px] uppercase tracking-[0.14em] text-signal transition-colors hover:bg-signal hover:text-white"
          >
            Revoke
          </button>
        </form>
      ) : null}
    </li>
  );
}
