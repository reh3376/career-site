import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Terms",
  description: "Terms of use for career-site.",
};

export default function TermsPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        the other small print
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Terms of use.
      </h1>
      <div className="mt-10 space-y-6 text-base leading-relaxed text-ink-2">
        <p>
          This site is under construction. Full terms &mdash; covering
          acceptable use of the site and Ask Roger, content licensing, and
          account management &mdash; ship alongside the launch (late 2026).
        </p>
        <p className="text-sm text-ink-3">
          In the meantime, this site is a personal, non-commercial portfolio.
          All content is &copy; the respective owner; source code is
          MIT-licensed (see{" "}
          <a
            href="https://github.com/reh3376/career-site"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            rel="noopener noreferrer"
            target="_blank"
          >
            the repository
          </a>
          ).
        </p>
      </div>
    </div>
  );
}
