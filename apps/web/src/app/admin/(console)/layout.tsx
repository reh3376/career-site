import Link from "next/link";
import { redirect } from "next/navigation";
import type { ReactNode } from "react";

import { getSessionUser, isAdmin } from "@/lib/session-user";

import { StaleBuild } from "./stale-build";

// Admin layout. Server-side role gate for the whole /admin subtree so
// unauthenticated visitors go to /login and authenticated non-admins
// bounce back to their member home. Renders a two-column layout: a
// left sidebar carrying the admin nav, main content on the right.
export const dynamic = "force-dynamic";

const NAV = [
  { href: "/admin", label: "Overview" },
  { href: "/admin/contacts", label: "Contact messages" },
  { href: "/admin/registrations", label: "Registrations" },
  { href: "/admin/access", label: "Access & whitelist" },
  { href: "/admin/activity", label: "Activity" },
  { href: "/admin/corpus", label: "Corpus" },
  { href: "/admin/jd", label: "JD submissions" },
  { href: "/admin/decisions", label: "Decision review" },
  { href: "/admin/evals", label: "Evaluations" },
  { href: "/admin/analytics", label: "Analytics" },
  { href: "/admin/db", label: "DB query" },
];

// buildVersion reads the version the API reports. Web and api roll out
// under one image tag, so this identifies the deploy that rendered the
// page, which is what StaleBuild compares against later from the
// browser. A failure here is not worth breaking the console over: an
// empty string simply turns the check off.
async function buildVersion(): Promise<string> {
  try {
    const base = process.env.API_URL ?? "http://localhost:8080";
    const resp = await fetch(`${base}/api/readyz`, { cache: "no-store" });
    if (!resp.ok) return "";
    const body = (await resp.json()) as { version?: string };
    return body.version ?? "";
  } catch {
    return "";
  }
}

export default async function AdminLayout({ children }: { children: ReactNode }) {
  const me = await getSessionUser();
  if (!me) redirect("/login?next=/admin");
  if (!isAdmin(me)) redirect("/home");
  const version = await buildVersion();

  return (
    <div className="mx-auto grid w-full max-w-6xl gap-10 px-6 py-14 sm:px-10 sm:py-20 md:grid-cols-[220px_1fr] md:gap-16">
      <StaleBuild renderedVersion={version} />
      <aside className="md:sticky md:top-24 md:self-start">
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
          admin console
        </p>
        <p
          className="font-display mt-2 text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          Ops surface.
        </p>
        <nav aria-label="Admin sections" className="mt-8 border-l border-line">
          <ul className="space-y-1">
            {NAV.map((n) => (
              <li key={n.href}>
                <Link
                  href={n.href}
                  className="-ml-px block border-l-2 border-transparent px-4 py-1.5 text-sm text-ink-2 no-underline transition-colors hover:border-accent hover:text-accent"
                >
                  {n.label}
                </Link>
              </li>
            ))}
          </ul>
        </nav>
        <p className="mt-10 text-xs leading-relaxed text-ink-3">
          Signed in as{" "}
          <span className="font-mono text-ink">{me.email}</span>
          {", "}admin role. Everything on this surface is auth-gated
          server-side.
        </p>
      </aside>

      <div>{children}</div>
    </div>
  );
}
