import Link from "next/link";

export function SiteHeader() {
  return (
    <header className="border-b border-line bg-paper">
      <div className="mx-auto flex max-w-6xl items-center justify-between px-6 py-4">
        <Link
          href="/"
          className="text-sm font-semibold tracking-tight text-ink hover:text-accent"
        >
          Roger Henley
        </Link>
        <nav className="flex items-center gap-4 text-sm text-ink-3" aria-label="Primary">
          <Link href="/login" className="text-ink-2 hover:text-accent">
            Sign in
          </Link>
          <Link
            href="/register"
            className="rounded-md bg-accent px-3 py-1.5 text-white transition-colors hover:bg-accent-hover"
          >
            Request access
          </Link>
        </nav>
      </div>
    </header>
  );
}
