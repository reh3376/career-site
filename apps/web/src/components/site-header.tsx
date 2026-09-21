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
        <div className="flex items-center gap-4">
          <Link
            href="/"
            aria-label="Roger Henley · home"
            className="group no-underline"
          >
            <span className="font-display text-[22px] font-semibold leading-none tracking-tight text-ink transition-colors group-hover:text-accent">
              Roger Henley
            </span>
          </Link>
          {/* Profile links beside the wordmark: LinkedIn and GitHub logos,
              rendered only when the URLs are configured. */}
          <span className="flex items-center gap-2.5">
            {social.linkedin ? (
              <a
                href={social.linkedin}
                rel="noopener noreferrer"
                target="_blank"
                aria-label="LinkedIn profile"
                title="LinkedIn profile"
                className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-line text-ink-2 transition-colors hover:border-accent hover:text-accent"
              >
                <svg viewBox="0 0 24 24" width="15" height="15" fill="currentColor" aria-hidden="true">
                  <path d="M20.45 20.45h-3.55v-5.57c0-1.33-.03-3.04-1.85-3.04-1.86 0-2.14 1.45-2.14 2.94v5.67H9.36V9h3.41v1.56h.05c.47-.9 1.63-1.85 3.36-1.85 3.6 0 4.27 2.37 4.27 5.45v6.29zM5.34 7.43a2.06 2.06 0 1 1 0-4.12 2.06 2.06 0 0 1 0 4.12zM7.12 20.45H3.56V9h3.56v11.45zM22.22 0H1.77C.79 0 0 .77 0 1.73v20.54C0 23.23.79 24 1.77 24h20.45c.98 0 1.78-.77 1.78-1.73V1.73C24 .77 23.2 0 22.22 0z" />
                </svg>
              </a>
            ) : null}
            {social.github ? (
              <a
                href={social.github}
                rel="noopener noreferrer"
                target="_blank"
                aria-label="GitHub repositories"
                title="GitHub repositories"
                className="inline-flex h-7 w-7 items-center justify-center rounded-md border border-line text-ink-2 transition-colors hover:border-accent hover:text-accent"
              >
                <svg viewBox="0 0 24 24" width="15" height="15" fill="currentColor" aria-hidden="true">
                  <path d="M12 .5C5.65.5.5 5.65.5 12c0 5.08 3.29 9.39 7.86 10.91.58.1.79-.25.79-.56v-2.17c-3.2.7-3.87-1.36-3.87-1.36-.52-1.33-1.28-1.68-1.28-1.68-1.04-.71.08-.7.08-.7 1.15.08 1.76 1.18 1.76 1.18 1.03 1.76 2.7 1.25 3.35.96.1-.75.4-1.25.73-1.54-2.55-.29-5.24-1.28-5.24-5.68 0-1.26.45-2.28 1.18-3.09-.12-.29-.51-1.46.11-3.04 0 0 .97-.31 3.17 1.18a11 11 0 0 1 5.78 0c2.2-1.49 3.17-1.18 3.17-1.18.62 1.58.23 2.75.11 3.04.74.81 1.18 1.83 1.18 3.09 0 4.41-2.69 5.38-5.25 5.67.41.36.78 1.06.78 2.14v3.17c0 .31.21.67.8.56A11.5 11.5 0 0 0 23.5 12C23.5 5.65 18.35.5 12 .5z" />
                </svg>
              </a>
            ) : null}
          </span>
        </div>

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
