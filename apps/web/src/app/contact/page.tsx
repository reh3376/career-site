import type { Metadata } from "next";

import { getSessionCookie } from "@/lib/session";

import { ContactForm } from "./form";

export const metadata: Metadata = {
  title: "Contact",
  description: "Reach Roger Henley about the site, a bug, a feature idea, or an interview.",
};

export const dynamic = "force-dynamic";

export default async function ContactPage() {
  const cookie = await getSessionCookie();
  const signedIn = Boolean(cookie);

  return (
    <div className="mx-auto max-w-5xl px-6 py-20 sm:px-10 sm:py-28">
      <div className="grid gap-16 md:grid-cols-[1fr_1.4fr] md:gap-20">
        {/* Left column — invitation, deliberately quiet */}
        <div className="max-w-sm">
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
            get in touch
          </p>
          <h1
            className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
            style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
          >
            Reach me directly.
          </h1>
          <p className="mt-6 text-base leading-relaxed text-ink-2">
            A bug, a feature idea, press, an interview, a question about the
            work &mdash; send it here. Replies come from my inbox, usually
            within a day.
          </p>
          <dl className="mt-10 space-y-4 font-mono text-sm text-ink-2">
            <div className="flex gap-3">
              <dt className="text-ink-3">for</dt>
              <dd className="m-0 text-ink">product feedback · press · hiring</dd>
            </div>
            <div className="flex gap-3">
              <dt className="text-ink-3">from</dt>
              <dd className="m-0 text-ink">rogerhenley.dev</dd>
            </div>
            <div className="flex gap-3">
              <dt className="text-ink-3">reply</dt>
              <dd className="m-0 text-ink">usually within 24h</dd>
            </div>
          </dl>
        </div>

        {/* Right column — the form itself, sitting on paper without a
            surrounding card. The inputs carry the visual weight. */}
        <div>
          <ContactForm signedIn={signedIn} />

          <p className="mt-10 border-t border-line pt-6 text-xs leading-relaxed text-ink-3">
            <span className="font-mono uppercase tracking-[0.14em] text-signal">
              security
            </span>
            <span className="mx-2 text-ink-4">·</span>
            If you&rsquo;re reporting a vulnerability, please use{" "}
            <a
              href="https://github.com/reh3376/career-site/security/advisories/new"
              className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
              rel="noopener noreferrer"
              target="_blank"
            >
              private GitHub Security Advisories
            </a>{" "}
            instead. Do not include a working exploit in a public form.
          </p>
        </div>
      </div>
    </div>
  );
}
