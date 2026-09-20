import type { Metadata } from "next";
import Link from "next/link";
import { redirect } from "next/navigation";

import { OtPanel } from "@/components/ot-panel";
import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";
import { getUiMode } from "@/lib/ui-mode";

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
  const [me, mode] = await Promise.all([fetchMe(), getUiMode()]);
  if (!me) redirect("/login");

  const firstName = me.name?.split(" ")[0] ?? "there";

  if (mode === "ot") {
    return (
      <OtPanel
        tag="MBR-01"
        title={`SESSION.ACTIVE · ${me.email}`}
        note={me.role === "MEMBER_ROLE_ADMIN" ? "role admin" : "role member"}
      >
        <div className="flex flex-wrap items-baseline justify-between gap-4">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
            under construction · ships late 2026
          </p>
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
        <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
          You&rsquo;re in. The personalized dashboard lands ahead of the
          late-2026 launch.
        </p>

        <dl className="mt-10 grid gap-3 border-t border-line pt-6 font-mono text-[12px] text-ink-2 sm:grid-cols-[10rem_1fr]">
          <dt className="text-ink-3">STATUS</dt>
          <dd className="m-0 text-success">
            <span className="pilot mr-2 align-middle text-success" aria-hidden="true" />
            ACTIVE
          </dd>
          <dt className="text-ink-3">EMAIL</dt>
          <dd className="m-0 text-ink">{me.email}</dd>
          <dt className="text-ink-3">ROLE</dt>
          <dd className="m-0 text-ink">
            {me.role === "MEMBER_ROLE_ADMIN" ? "admin" : "member"}
          </dd>
          <dt className="text-ink-3">BACKEND</dt>
          <dd className="m-0">
            <Link
              href="/version"
              className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              /version
            </Link>
          </dd>
        </dl>

        <p className="mt-8 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
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
