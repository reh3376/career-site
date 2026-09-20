import Link from "next/link";

// Multi-column footer. Colophon on the left (what this site IS), site
// nav in the middle, community links on the right (GitHub repos +
// contributor request — public repo, so make that surface deliberate).
export function SiteFooter() {
  const year = new Date().getFullYear();
  return (
    <footer className="border-t border-line bg-paper-2">
      <div className="mx-auto max-w-6xl px-6 py-14 sm:px-10">
        <div className="grid gap-10 md:grid-cols-[1.4fr_1fr_1fr]">
          {/* Colophon — the site is a work sample; say so plainly */}
          <div className="max-w-md">
            <p className="font-display text-lg leading-snug text-ink">
              A practice, not a portfolio.
            </p>
            <p className="mt-3 text-sm leading-relaxed text-ink-2">
              This site is itself a work sample: Go &amp; Python behind a
              Next.js front-end, deployed on Docker with everything in the open.
              The spec, the decisions, and the diffs all live on GitHub.
            </p>
            <p className="mt-4 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              {year} <span className="text-ink-4">·</span> Roger Henley
            </p>
          </div>

          {/* Site nav */}
          <nav aria-label="Footer" className="text-sm">
            <p className="mb-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              This site
            </p>
            <ul className="space-y-2">
              <li>
                <Link href="/contact" className="text-ink-2 no-underline hover:text-accent">
                  Contact
                </Link>
              </li>
              <li>
                <Link href="/how-ask-roger-works" className="text-ink-2 no-underline hover:text-accent">
                  How Ask Roger works
                </Link>
              </li>
              <li>
                <Link href="/privacy" className="text-ink-2 no-underline hover:text-accent">
                  Privacy
                </Link>
              </li>
              <li>
                <Link href="/terms" className="text-ink-2 no-underline hover:text-accent">
                  Terms
                </Link>
              </li>
            </ul>
          </nav>

          {/* Community */}
          <div className="text-sm">
            <p className="mb-3 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              Public code
            </p>
            <ul className="space-y-2">
              <li>
                <a
                  href="https://github.com/reh3376"
                  rel="noopener noreferrer"
                  target="_blank"
                  className="text-ink-2 no-underline hover:text-accent"
                >
                  github.com/reh3376
                </a>
              </li>
              <li>
                <a
                  href="https://github.com/reh3376/career-site"
                  rel="noopener noreferrer"
                  target="_blank"
                  className="text-ink-2 no-underline hover:text-accent"
                >
                  This site&rsquo;s repo
                </a>
              </li>
              <li>
                <a
                  href="https://github.com/reh3376/career-site/blob/main/CONTRIBUTING.md"
                  rel="noopener noreferrer"
                  target="_blank"
                  className="text-ink-2 no-underline hover:text-accent"
                >
                  Become a contributor
                </a>
              </li>
            </ul>
          </div>
        </div>
      </div>
    </footer>
  );
}
