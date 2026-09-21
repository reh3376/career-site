import type { Metadata } from "next";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { DeleteGrantButton } from "./delete-button";
import { GrantForm } from "./grant-form";

export const metadata: Metadata = { title: "Admin · Access & whitelist" };
export const dynamic = "force-dynamic";

// Mirrors the AccessGrant proto in wire form (camelCase from the
// ConnectRPC JSON codec).
type AccessGrant = {
  id: string;
  email: string;
  default_ttl: string;
  notes?: string;
  entry_expires_at?: string;
  created_by?: string;
  created_at?: string;
  updated_at?: string;
};

type ListResp = {
  grants?: AccessGrant[];
  active_count?: number;
  expired_count?: number;
};

type FetchResult =
  | { ok: true; data: ListResp }
  | { ok: false; status: number; message: string };

async function fetchGrants(query: string): Promise<FetchResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, status: 401, message: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/ListAccessGrants",
    body: { query },
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

const TTL_LABEL: Record<string, string> = {
  GRANT_TTL_1D: "1 day",
  GRANT_TTL_3D: "3 days",
  GRANT_TTL_7D: "7 days",
  GRANT_TTL_30D: "30 days",
  GRANT_TTL_PERMANENT: "permanent",
};

export default async function AdminAccessPage({
  searchParams,
}: {
  searchParams: Promise<{ q?: string }>;
}) {
  const params = await searchParams;
  const q = params.q ?? "";
  const result = await fetchGrants(q);

  if (!result.ok) {
    return (
      <>
        <Header />
        <p className="mt-6 max-w-lg border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          Couldn&rsquo;t load the whitelist, {result.message}.
        </p>
      </>
    );
  }

  const { grants = [], active_count = 0, expired_count = 0 } = result.data;
  const nowMs = getNowMs();

  return (
    <>
      <Header />

      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        Emails on this list are auto-approved on signup. The TTL sets
        how long the resulting account stays active; the (optional)
        entry-expiry limits how long the whitelist row itself is
        honoured. Removing an entry only stops future auto-approvals,
        existing accounts are untouched.
      </p>

      <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-3">
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">active</dt>
          <dd className="m-0 tabular text-ink text-2xl">{active_count}</dd>
        </div>
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">lapsed</dt>
          <dd className="m-0 tabular text-ink text-2xl">{expired_count}</dd>
        </div>
        <div className="flex items-baseline gap-3">
          <dt className="text-ink-3">total</dt>
          <dd className="m-0 tabular text-ink text-2xl">
            {active_count + expired_count}
          </dd>
        </div>
      </dl>

      <section aria-label="Add or update entry" className="mt-8">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          add / update
        </p>
        <p className="mt-1 text-xs text-ink-3">
          Adding an email that&rsquo;s already on the list updates it in
          place.
        </p>
        <div className="mt-3">
          <GrantForm />
        </div>
      </section>

      <section aria-label="Whitelist entries" className="mt-12">
        <div className="flex items-baseline justify-between">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            entries
          </p>
          <form className="m-0" action="/admin/access" method="get">
            <input
              type="search"
              name="q"
              defaultValue={q}
              placeholder="filter email / notes"
              className="border border-line-strong bg-canvas px-2 py-1 font-mono text-xs text-ink outline-none focus:border-accent"
            />
          </form>
        </div>

        {grants.length === 0 ? (
          <p className="mt-6 text-sm text-ink-3">
            {q
              ? `No entries match "${q}".`
              : "No whitelist entries yet. Add one above."}
          </p>
        ) : (
          <ul className="mt-4 divide-y divide-line border-y border-line">
            {grants.map((g) => (
              <GrantRow key={g.id} g={g} nowMs={nowMs} />
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
        Access &amp; whitelist.
      </h1>
    </>
  );
}

function GrantRow({ g, nowMs }: { g: AccessGrant; nowMs: number }) {
  const created = g.created_at ? new Date(g.created_at) : null;
  const entryExp = g.entry_expires_at ? new Date(g.entry_expires_at) : null;
  const isLapsed = entryExp !== null && entryExp.getTime() <= nowMs;
  return (
    <li className="py-4">
      <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <span className="text-ink">{g.email}</span>
        <span>ttl · {TTL_LABEL[g.default_ttl] ?? g.default_ttl}</span>
        {created ? <span>added · {relativeDays(created, nowMs)}</span> : null}
        {entryExp ? (
          <span className={isLapsed ? "text-signal" : "text-ink-2"}>
            entry ·{" "}
            {isLapsed ? "lapsed" : "expires " + relativeDays(entryExp, nowMs)}
          </span>
        ) : (
          <span className="text-ink-2">entry · never lapses</span>
        )}
      </div>
      {g.notes ? (
        <p className="mt-1 text-sm text-ink-2">{g.notes}</p>
      ) : null}
      <div className="mt-3">
        <DeleteGrantButton id={g.id} email={g.email} />
      </div>
    </li>
  );
}

// Wrapped so the `Date.now` reference isn't lint-flagged as an impure
// call from a component body (other admin pages do the same).
function getNowMs(): number {
  return Date.now();
}

function relativeDays(d: Date, nowMs: number): string {
  const ms = d.getTime() - nowMs;
  const days = Math.round(ms / 86_400_000);
  if (days === 0) return "today";
  if (days > 0) {
    if (days === 1) return "in 1d";
    if (days < 30) return `in ${days}d`;
    if (days < 365) return `in ${Math.round(days / 30)}mo`;
    return `in ${Math.round(days / 365)}y`;
  }
  const ago = -days;
  if (ago === 1) return "1d ago";
  if (ago < 30) return `${ago}d ago`;
  if (ago < 365) return `${Math.round(ago / 30)}mo ago`;
  return `${Math.round(ago / 365)}y ago`;
}
