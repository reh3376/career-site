import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Terms",
  description: "Terms of use for career-site.",
};

export default function TermsPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-16">
      <h1 className="mb-8 text-3xl font-semibold text-ink">Terms of use</h1>
      <div className="space-y-6 leading-relaxed text-ink-2">
        <p>
          This site is under construction. Full terms — covering acceptable use of the site and Ask
          Roger, content licensing, and account management — ship alongside the launch (late 2026).
        </p>
        <p className="text-sm text-ink-3">
          In the meantime, this site is a personal, non-commercial portfolio. All content is © the
          respective owner; source code is MIT-licensed (see{" "}
          <a
            href="https://github.com/reh3376/career-site"
            className="text-accent underline underline-offset-2"
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
