import Link from "next/link";

// Site header. Wordmark set in the display face, a live "system · nominal"
// indicator that pulses once every 2.4s (the one motion moment on the
// page — see globals.css), and the two nav CTAs. Deliberately thin: the
// header sits on paper without a card or shadow, separated only by a
// hairline and the whitespace of the layout below it.
export function SiteHeader() {
  return (
    <header className="border-b border-line bg-paper">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-5 sm:px-10">
        {/* Wordmark. Fraunces at a display size but held in check so it
            doesn't overpower a page whose real headline is the hero. */}
        <Link
          href="/"
          aria-label="Roger Henley — home"
          className="group flex items-baseline gap-3 no-underline"
        >
          <span className="font-display text-[22px] font-semibold leading-none tracking-tight text-ink transition-colors group-hover:text-accent">
            Roger Henley
          </span>
          <span
            aria-hidden="true"
            className="hidden font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 sm:inline"
          >
            practice notes
          </span>
        </Link>

        {/* Right cluster — system indicator + nav */}
        <div className="flex items-center gap-6">
          {/* Live status indicator. Renders on all pages so the same
              motion doesn't have to be re-earned every route change. */}
          <div
            className="hidden items-center gap-2 md:flex"
            aria-label="Site status: nominal"
            title="system.nominal — the site itself is a working plant"
          >
            <span className="relative inline-flex h-2.5 w-2.5">
              <span className="pulse-signal absolute inline-flex h-full w-full rounded-full bg-signal" />
              <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-signal" />
            </span>
            <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              system<span className="text-ink-4"> · </span>nominal
            </span>
          </div>

          <nav aria-label="Primary" className="flex items-center gap-5 text-sm">
            <Link
              href="/login"
              className="text-ink-2 no-underline transition-colors hover:text-accent"
            >
              Sign in
            </Link>
            <Link
              href="/register"
              className="rounded-md bg-accent px-4 py-2 text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
            >
              Request access
            </Link>
          </nav>
        </div>
      </div>
    </header>
  );
}
