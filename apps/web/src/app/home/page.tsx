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
    <div className="mx-auto max-w-3xl px-6 py-16">
      <div className="mb-8 flex items-center justify-between">
        <p className="text-sm text-ink-3">
          Signed in as <span className="font-medium text-ink-2">{me.email}</span>
        </p>
        <form action={logoutAction}>
          <button
            type="submit"
            className="rounded-md border border-line px-3 py-1.5 text-sm text-ink-2 hover:border-accent hover:text-accent"
          >
            Sign out
          </button>
        </form>
      </div>

      <p className="mb-4 text-sm font-medium uppercase tracking-widest text-accent">
        Coming soon — late 2026
      </p>
      <h1 className="mb-6 text-4xl font-semibold leading-tight tracking-tight text-ink sm:text-5xl">
        Welcome, {firstName}.
      </h1>
      <p className="mb-8 text-lg leading-relaxed text-ink-2">
        You&rsquo;re in. The site is still being built &mdash; the personalized home, projects,
        writing, and Ask Roger all land ahead of the late-2026 launch. This page becomes your
        tailored dashboard when it does.
      </p>

      <section className="rounded-lg border border-line bg-paper-2 p-6 text-sm leading-relaxed text-ink-2">
        <h2 className="mb-3 text-sm font-semibold text-ink">What&rsquo;s live today</h2>
        <ul className="space-y-2">
          <li>• Approval-gated registration + one-click admin review (this page).</li>
          <li>• Member sign-in with 30-day sessions.</li>
          <li>
            • Backend health at{" "}
            <Link href="/version" className="text-accent underline underline-offset-2">
              /version
            </Link>
            .
          </li>
        </ul>
        <p className="mt-4 text-xs text-ink-3">
          Questions or feedback while it&rsquo;s under construction? Reach Roger at{" "}
          <a
            href="mailto:rogerhenley345@gmail.com"
            className="text-accent underline underline-offset-2"
          >
            rogerhenley345@gmail.com
          </a>
          .
        </p>
      </section>
    </div>
  );
}
