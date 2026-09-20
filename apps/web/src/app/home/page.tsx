import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import { logoutAction } from "./actions";

export const metadata: Metadata = {
  title: "Member home",
};

// The gated member home. Today it's the "under construction" screen an
// approved member sees; Phases 2+ replace this with the tailored home
// (roles, projects, articles, Ask Roger). Server-side session check
// bounces anonymous visitors to /login.
export const dynamic = "force-dynamic";

type Me = {
  id: string;
  name: string;
  email: string;
  status: string;
  role: string;
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

export default async function HomePage() {
  const me = await fetchMe();
  if (!me) redirect("/login");

  const firstName = me.name?.split(" ")[0] ?? "there";

  return (
    <div className="mx-auto max-w-4xl px-6 py-16 sm:px-10 sm:py-20">
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

      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        under construction <span className="text-ink-4">·</span> ships late 2026
      </p>
      <h1
        className="font-display mt-4 text-5xl leading-[0.98] tracking-tight text-ink sm:text-6xl"
        style={{ fontVariationSettings: '"opsz" 144, "SOFT" 40' }}
      >
        Welcome, <span className="italic text-accent">{firstName}</span>.
      </h1>
      <p className="mt-8 max-w-2xl text-lg leading-relaxed text-ink-2">
        You&rsquo;re in. The personalized home, projects, writing, and Ask Roger
        all land ahead of the late-2026 launch. This page becomes your tailored
        dashboard when it does.
      </p>

      <section className="mt-16 grid gap-10 border-t border-line pt-10 md:grid-cols-[1fr_1.4fr]">
        <div>
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            currently live
          </p>
          <h2
            className="font-display mt-3 text-2xl leading-snug text-ink"
            style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
          >
            The scaffolding is real.
          </h2>
        </div>
        <ul className="space-y-4 text-base leading-relaxed text-ink-2">
          <li className="flex gap-3">
            <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" />
            <span>Approval-gated registration with one-click admin review.</span>
          </li>
          <li className="flex gap-3">
            <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" />
            <span>Member sign-in with 30-day sessions.</span>
          </li>
          <li className="flex gap-3">
            <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" />
            <span>
              Backend health at{" "}
              <Link
                href="/version"
                className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
              >
                /version
              </Link>
              .
            </span>
          </li>
        </ul>
      </section>

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
