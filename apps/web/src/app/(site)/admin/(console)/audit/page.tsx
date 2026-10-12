import type { Metadata } from "next";
import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

export const metadata: Metadata = { title: "Admin · Audit log" };
export const dynamic = "force-dynamic";

// ConnectRPC's JSON codec emits camelCase. Both spellings are accepted
// here for the same reason the activity page accepts both: so a codec
// change does not blank the table silently.
type Entry = {
  id?: string;
  actor_id?: string;
  actorId?: string;
  actor_name?: string;
  actorName?: string;
  action?: string;
  target_type?: string;
  targetType?: string;
  target_id?: string;
  targetId?: string;
  payload_json?: string;
  payloadJson?: string;
  outcome?: string;
  occurred_at?: string;
  occurredAt?: string;
};

type Resp = {
  entries?: Entry[];
  page?: { totalCount?: number; total_count?: number };
};

type FetchResult = { ok: true; data: Resp } | { ok: false; error: string };

const PAGE_SIZE = 50;

async function fetchAudit(target: string, page: number): Promise<FetchResult> {
  const cookie = await getSessionCookie();
  if (!cookie) return { ok: false, error: "Not signed in" };
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetAudit",
    body: {
      actionPrefix: "",
      page: {
        pageSize: PAGE_SIZE,
        pageToken: page > 0 ? String(page * PAGE_SIZE) : "",
      },
      ...(target ? { actorId: target } : {}),
    },
    cookie,
  });
  if (!resp.ok) return { ok: false, error: `HTTP ${resp.status}` };
  return { ok: true, data: (await resp.json()) as Resp };
}

const pick = (a?: string, b?: string) => a ?? b ?? "";

// "/career.v1.AdminService/SetMemberStatus" reads as "SetMemberStatus"
// in a table. The whole procedure is kept in the row's title attribute,
// because it is the string that matches the contract and the one worth
// pasting into a search.
function shortAction(procedure: string): string {
  const i = procedure.lastIndexOf("/");
  return i >= 0 ? procedure.slice(i + 1) : procedure;
}

// Everything except `outcome`, which has its own column.
function detailsOf(payloadJson: string): Array<[string, string]> {
  if (!payloadJson) return [];
  try {
    const obj = JSON.parse(payloadJson) as Record<string, unknown>;
    return Object.entries(obj)
      .filter(([k]) => k !== "outcome")
      .map(([k, v]) => [k, typeof v === "string" ? v : JSON.stringify(v)]);
  } catch {
    return [["payload", payloadJson]];
  }
}

export default async function AdminAuditPage({
  searchParams,
}: {
  searchParams: Promise<{ actor?: string; page?: string }>;
}) {
  const params = await searchParams;
  const actor = (params.actor ?? "").trim();
  const page = Math.max(0, Number(params.page ?? "0") || 0);
  const result = await fetchAudit(actor, page);

  const entries = result.ok ? (result.data.entries ?? []) : [];
  const total = result.ok
    ? (result.data.page?.totalCount ?? result.data.page?.total_count ?? 0)
    : 0;

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        admin surface
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Audit log.
      </h1>
      <p className="mt-6 max-w-2xl text-sm leading-relaxed text-ink-3">
        One row per admin action that changes something or moves member data,
        written by the server rather than by each handler, so a new admin
        endpoint cannot ship without appearing here. Refused attempts are
        recorded too: the outcome column says which.
      </p>

      {!result.ok ? (
        <p
          role="alert"
          className="mt-8 border-l-2 border-danger bg-danger-soft/40 px-4 py-3 text-sm text-ink"
        >
          Could not load the audit log: {result.error}
        </p>
      ) : entries.length === 0 ? (
        <p className="mt-8 text-sm text-ink-3">
          {actor
            ? "No actions recorded for that actor."
            : "No actions recorded yet."}
        </p>
      ) : (
        <>
          <p className="mt-8 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            {total} action{total === 1 ? "" : "s"} recorded
            {actor ? ` by actor ${actor}` : ""}
          </p>

          <div className="mt-4 overflow-x-auto">
            <table className="w-full min-w-[56rem] border-collapse text-sm">
              <thead>
                <tr className="border-b border-line text-left font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  <th className="py-2 pr-4 font-normal">When</th>
                  <th className="py-2 pr-4 font-normal">Actor</th>
                  <th className="py-2 pr-4 font-normal">Action</th>
                  <th className="py-2 pr-4 font-normal">Target</th>
                  <th className="py-2 pr-4 font-normal">Outcome</th>
                  <th className="py-2 font-normal">Details</th>
                </tr>
              </thead>
              <tbody>
                {entries.map((e) => {
                  const occurred = pick(e.occurred_at, e.occurredAt);
                  const actorId = pick(e.actor_id, e.actorId);
                  const actorName = pick(e.actor_name, e.actorName);
                  const action = pick(e.action, e.action);
                  const targetType = pick(e.target_type, e.targetType);
                  const targetId = pick(e.target_id, e.targetId);
                  const details = detailsOf(
                    pick(e.payload_json, e.payloadJson),
                  );
                  const outcome = e.outcome ?? "";
                  return (
                    <tr
                      key={e.id}
                      className="border-b border-line/60 align-top"
                    >
                      <td className="py-3 pr-4 whitespace-nowrap font-mono text-xs text-ink-3">
                        {occurred
                          ? new Date(occurred)
                              .toISOString()
                              .replace("T", " ")
                              .slice(0, 19)
                          : ""}
                      </td>
                      <td className="py-3 pr-4 whitespace-nowrap">
                        {actorName || (actorId ? `#${actorId}` : "system")}
                      </td>
                      <td
                        className="py-3 pr-4 font-mono text-xs"
                        title={action}
                      >
                        {shortAction(action)}
                      </td>
                      <td className="py-3 pr-4 whitespace-nowrap text-ink-2">
                        {targetId ? `${targetType || "id"} ${targetId}` : "·"}
                      </td>
                      <td
                        className={
                          "py-3 pr-4 font-mono text-xs " +
                          (outcome === "ok" ? "text-success" : "text-danger")
                        }
                      >
                        {outcome || "·"}
                      </td>
                      <td className="py-3 text-xs text-ink-2">
                        {details.length === 0 ? (
                          "·"
                        ) : (
                          <dl className="space-y-1">
                            {details.map(([k, v]) => (
                              <div key={k} className="flex gap-2">
                                <dt className="shrink-0 font-mono text-ink-3">
                                  {k}
                                </dt>
                                <dd className="break-all">{v}</dd>
                              </div>
                            ))}
                          </dl>
                        )}
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>

          <nav
            aria-label="Pages"
            className="mt-6 flex items-center gap-4 font-mono text-[11px] uppercase tracking-[0.14em]"
          >
            {page > 0 ? (
              <Link
                href={`/admin/audit?page=${page - 1}${actor ? `&actor=${actor}` : ""}`}
                className="text-accent underline decoration-accent/40 underline-offset-4"
              >
                newer
              </Link>
            ) : (
              <span className="text-ink-4">newer</span>
            )}
            <span className="text-ink-3">
              {page * PAGE_SIZE + 1}
              {entries.length > 1
                ? ` to ${page * PAGE_SIZE + entries.length}`
                : ""}{" "}
              of {total}
            </span>
            {(page + 1) * PAGE_SIZE < total ? (
              <Link
                href={`/admin/audit?page=${page + 1}${actor ? `&actor=${actor}` : ""}`}
                className="text-accent underline decoration-accent/40 underline-offset-4"
              >
                older
              </Link>
            ) : (
              <span className="text-ink-4">older</span>
            )}
          </nav>
        </>
      )}
    </>
  );
}
