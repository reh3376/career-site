import Link from "next/link";

import { HamburgerMenu } from "@/components/hamburger-menu";
import { getSessionUser, isAdmin } from "@/lib/session-user";
import { getSocialLinks } from "@/lib/social-links";
import { getUiMode } from "@/lib/ui-mode";

// Site header. Server component: it resolves the caller's session on
// the server so the initial paint carries the right nav (no client
// flash between anon and signed-in). The hamburger client component is
// mounted inside for open/close state and the drawer contents; it also
// receives the same role state so its menu is composed on the server.
export async function SiteHeader() {
  const [me, mode] = await Promise.all([getSessionUser(), getUiMode()]);
  const signedIn = me != null;
  const admin = isAdmin(me);
  const social = getSocialLinks();

  return (
    <header className="relative border-b border-line bg-paper">
      <div className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-4 py-5 sm:px-10">
        {/* Wordmark. Fraunces at a display size, held in check so it
            doesn't overpower a page whose real headline is the hero. */}
        <Link
          href="/"
          aria-label="Roger Henley · home"
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

        {/* Right cluster, system indicator + primary CTA + hamburger.
            The hamburger is always the last item so it lands under the
            same thumb on every viewport. */}
        <div className="flex items-center gap-3 sm:gap-5">
          {/* Live status indicator. The one motion moment on the page,
              see globals.css `pulse-signal`. Hidden below md so the
              header stays a two-thing header on phones. */}
          <div
            className="hidden items-center gap-2 md:flex"
            aria-label="Site status: nominal"
            title="system.nominal, the site itself is a working plant"
          >
            <span className="relative inline-flex h-2.5 w-2.5">
              <span className="pulse-signal absolute inline-flex h-full w-full rounded-full bg-signal" />
              <span className="relative inline-flex h-2.5 w-2.5 rounded-full bg-signal" />
            </span>
            <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              system<span className="text-ink-4"> · </span>nominal
            </span>
          </div>

          {/* Primary CTA sits in the header for anon visitors so the
              conversion path is one click on any viewport. Signed-in
              members already have Sign out inside the hamburger; we
              don't repeat it out here. On mobile we hide the anon CTA
              too, the hamburger carries it. */}
          {signedIn ? null : (
            <Link
              href="/register"
              className="hidden rounded-md bg-accent px-4 py-2 text-sm text-white no-underline shadow-sm transition-colors hover:bg-accent-hover sm:inline-block"
            >
              Request access
            </Link>
          )}

          <HamburgerMenu
            signedIn={signedIn}
            isAdmin={admin}
            memberName={me?.name}
            memberEmail={me?.email}
            mode={mode}
            social={{
              linkedinUrl: social.linkedin,
              linkedinHandle: social.linkedinHandle,
              githubUrl: social.github,
              githubHandle: social.githubHandle,
            }}
          />
        </div>
      </div>
    </header>
  );
}
