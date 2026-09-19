import Link from "next/link";

export function SiteFooter() {
  const year = new Date().getFullYear();
  return (
    <footer className="border-t border-line bg-paper-2">
      <div className="mx-auto flex max-w-6xl flex-col gap-4 px-6 py-8 text-sm text-ink-3 sm:flex-row sm:items-center sm:justify-between">
        <p className="m-0">© {year} Roger Henley</p>
        <nav aria-label="Footer" className="flex flex-wrap gap-x-6 gap-y-2">
          <Link href="/privacy" className="hover:text-accent">
            Privacy
          </Link>
          <Link href="/terms" className="hover:text-accent">
            Terms
          </Link>
          <Link href="/how-ask-roger-works" className="hover:text-accent">
            How Ask Roger works
          </Link>
        </nav>
      </div>
    </footer>
  );
}
