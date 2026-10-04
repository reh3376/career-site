import Link from "next/link";

import { SiteFooter } from "@/components/site-footer";
import { SiteHeader } from "@/components/site-header";

// A 404 for a URL that matched no route at all.
//
// This has to draw its own header and footer. An unmatched URL belongs
// to no route group, so Next renders it against the bare root layout,
// and without this it would be the framework's default: a line of text,
// no navigation, and no way back for someone who mistyped or followed a
// stale link. That is the same dead end lib/public-routes.ts was written
// to avoid when it stopped sending /timeline and /projects to a sign-in
// form.
//
// It names the real destinations rather than offering only "go home",
// because the usual way to arrive here is a link to a page that never
// existed, and the useful reply is what does.
export default function NotFound() {
  return (
    <>
      <SiteHeader />
      <main className="flex-1">
        <div className="mx-auto max-w-2xl px-6 py-24 sm:px-10 sm:py-32">
          <p className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
            404
          </p>
          <h1
            className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
            style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
          >
            That page is not here.
          </h1>
          <p className="mt-5 text-base leading-relaxed text-ink-2">
            Either the address has a typo in it, or it points at something
            that was never built. Both happen. Here is what does exist:
          </p>
          <ul className="mt-8 divide-y divide-line border-y border-line">
            {[
              { href: "/", label: "The front page", note: "who I am and what I do" },
              { href: "/articles", label: "Writing", note: "the decision-quality series and more" },
              { href: "/how-ask-roger-works", label: "How the reviewer works", note: "the model, the method, the live results" },
              { href: "/contact", label: "Contact", note: "including asking for access" },
            ].map((l) => (
              <li key={l.href}>
                <Link
                  href={l.href}
                  className="group flex flex-wrap items-baseline justify-between gap-x-6 gap-y-1 py-5 no-underline"
                >
                  <span className="text-lg leading-snug text-ink transition-colors group-hover:text-accent">
                    {l.label}
                  </span>
                  <span className="font-mono text-[11px] tracking-[0.14em] text-ink-3 uppercase">
                    {l.note}
                  </span>
                </Link>
              </li>
            ))}
          </ul>
        </div>
      </main>
      <SiteFooter />
    </>
  );
}
