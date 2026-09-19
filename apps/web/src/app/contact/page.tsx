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
    <div className="mx-auto max-w-2xl px-6 py-16">
      <p className="mb-3 text-sm font-medium uppercase tracking-widest text-accent">
        Get in touch
      </p>
      <h1 className="mb-4 text-4xl font-semibold leading-tight tracking-tight text-ink sm:text-5xl">
        Reach me directly.
      </h1>
      <p className="mb-10 text-lg leading-relaxed text-ink-2">
        Bug, feature idea, press inquiry, or a question about my work — send it here. Replies come
        from my inbox, usually within a day.
      </p>

      <ContactForm signedIn={signedIn} />

      <p className="mt-8 text-xs text-ink-3">
        Reporting a security issue? Please use{" "}
        <a
          href="https://github.com/reh3376/career-site/security/advisories/new"
          className="text-accent underline underline-offset-2"
          rel="noopener noreferrer"
          target="_blank"
        >
          private GitHub Security Advisories
        </a>{" "}
        instead — do not include an exploit in a public form.
      </p>
    </div>
  );
}
