import Link from "next/link";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

type Row = {
  id: string;
  status?: string;
  match_score?: number;
  matchScore?: number;
  role_hint?: string;
  roleHint?: string;
  employer_hint?: string;
  employerHint?: string;
  created_at?: string;
  createdAt?: string;
  has_resume?: boolean;
  hasResume?: boolean;
};

const LABEL: Record<string, string> = {
  JD_STATUS_RECEIVED: "queued",
  JD_STATUS_SCORING: "scoring",
  JD_STATUS_GENERATING: "writing the résumé",
  JD_STATUS_BELOW_THRESHOLD: "below the gate",
  JD_STATUS_READY: "résumé ready",
  JD_STATUS_FAILED: "failed",
};

// The member's own submissions, newest first. Server-rendered; each
// row opens /jd-upload/<id>, which works from any signed-in session.
export async function MySubmissions() {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.JdService/ListMySubmissions",
    body: {},
    cookie,
  }).catch(() => null);
  if (!resp || !resp.ok) return null;
  const data = (await resp.json()) as { submissions?: Row[] };
  const rows = data.submissions ?? [];
  if (rows.length === 0) return null;

  return (
    <section aria-labelledby="my-submissions-heading" className="mt-14">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        your submissions
      </p>
      <h2
        id="my-submissions-heading"
        className="font-display mt-3 text-2xl leading-tight text-ink"
        style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
      >
        Reviews you can reopen any time.
      </h2>
      <ul className="mt-6 divide-y divide-line border-y border-line">
        {rows.map((r) => {
          const status = r.status ?? "";
          const score = r.match_score ?? r.matchScore;
          const role = r.role_hint ?? r.roleHint ?? "";
          const employer = r.employer_hint ?? r.employerHint ?? "";
          const created = (r.created_at ?? r.createdAt ?? "").slice(0, 10);
          const live =
            status === "JD_STATUS_RECEIVED" ||
            status === "JD_STATUS_SCORING" ||
            status === "JD_STATUS_GENERATING";
          return (
            <li key={r.id} className="py-4">
              <Link
                href={`/jd-upload/${r.id}`}
                className="grid gap-x-6 gap-y-1 no-underline sm:grid-cols-[4rem_1fr_10rem_5rem]"
              >
                <span className="font-mono text-xs text-ink-3">#{r.id}</span>
                <span className="text-sm text-ink">
                  {role || employer ? (
                    <>
                      {role || "Untitled role"}
                      {employer ? <span className="text-ink-3"> at {employer}</span> : null}
                    </>
                  ) : (
                    "Untitled posting"
                  )}
                  <span className="block text-xs text-ink-3">{created}</span>
                </span>
                <span
                  className={
                    "font-mono text-[11px] uppercase tracking-[0.14em] " +
                    (status === "JD_STATUS_READY"
                      ? "text-signal"
                      : status === "JD_STATUS_FAILED"
                        ? "text-danger"
                        : live
                          ? "text-accent"
                          : "text-ink-3")
                  }
                >
                  {LABEL[status] ?? status.toLowerCase()}
                </span>
                <span className="font-mono text-sm text-ink">
                  {typeof score === "number" ? score.toFixed(2) : ""}
                </span>
              </Link>
            </li>
          );
        })}
      </ul>
    </section>
  );
}
