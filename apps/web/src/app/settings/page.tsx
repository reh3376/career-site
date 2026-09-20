import type { Metadata } from "next";
import Link from "next/link";

import { signOutAction } from "@/app/actions/session";
import { ModeToggle } from "@/components/mode-toggle";
import { getSessionUser, isAdmin } from "@/lib/session-user";
import { getUiMode } from "@/lib/ui-mode";

export const metadata: Metadata = { title: "Settings" };
export const dynamic = "force-dynamic";

// User- and site-preference surface reached from the hamburger menu.
// Right now it holds account state for signed-in visitors and a
// placeholder for site display preferences (the IT/OT mode toggle
// ships in the next PR). Anon visitors see the display-preferences
// block only, so the surface exists for everyone without leaking
// anything private.
export default async function SettingsPage() {
  const [me, mode] = await Promise.all([getSessionUser(), getUiMode()]);
  const admin = isAdmin(me);

  return (
    <div className="mx-auto max-w-3xl px-6 py-20 sm:px-10 sm:py-24">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        preferences
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Settings.
      </h1>
      <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
        Site and account preferences. More lands here as features come
        online &mdash; the IT / OT display mode is next.
      </p>

      {/* Account block — signed-in only. Kept intentionally spartan;
          the real account-edit surface (change email, change password,
          delete account) ships alongside the admin console rewrite. */}
      {me ? (
        <section
          aria-labelledby="account-heading"
          className="mt-14 border-t border-line pt-10"
        >
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            your account
          </p>
          <h2
            id="account-heading"
            className="font-display mt-3 text-2xl leading-snug text-ink"
            style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
          >
            Signed in.
          </h2>
          <dl className="mt-6 grid gap-4 font-mono text-sm text-ink-2 sm:grid-cols-[8rem_1fr]">
            <dt className="text-ink-3">name</dt>
            <dd className="m-0 text-ink">{me.name || "—"}</dd>
            <dt className="text-ink-3">email</dt>
            <dd className="m-0 text-ink">{me.email}</dd>
            <dt className="text-ink-3">role</dt>
            <dd className="m-0 text-ink">
              {admin ? "admin" : "member"}
            </dd>
          </dl>
          <div className="mt-8 flex flex-wrap gap-x-6 gap-y-3">
            {admin ? (
              <Link
                href="/admin"
                className="text-sm text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
              >
                Admin console
              </Link>
            ) : null}
            <Link
              href="/home"
              className="text-sm text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              Member home
            </Link>
            <form action={signOutAction} className="m-0">
              <button
                type="submit"
                className="text-sm text-ink-2 underline decoration-line decoration-1 underline-offset-4 hover:text-signal hover:decoration-signal"
              >
                Sign out
              </button>
            </form>
          </div>
        </section>
      ) : (
        <section
          aria-labelledby="signin-heading"
          className="mt-14 border-t border-line pt-10"
        >
          <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            your account
          </p>
          <h2
            id="signin-heading"
            className="font-display mt-3 text-2xl leading-snug text-ink"
            style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
          >
            Not signed in.
          </h2>
          <p className="mt-4 max-w-md text-sm leading-relaxed text-ink-2">
            Account preferences are available once you&rsquo;re signed
            in.{" "}
            <Link
              href="/login?next=/settings"
              className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              Sign in
            </Link>
            {" "}or{" "}
            <Link
              href="/register"
              className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              request access
            </Link>
            .
          </p>
        </section>
      )}

      {/* Display block — always shown. The mode toggle is a client
          component that flips <html data-mode> optimistically and then
          persists via a server action; every page across the site
          re-tokens off the same attribute. */}
      <section
        aria-labelledby="display-heading"
        className="mt-14 border-t border-line pt-10"
      >
        <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          display
        </p>
        <h2
          id="display-heading"
          className="font-display mt-3 text-2xl leading-snug text-ink"
          style={{ fontVariationSettings: '"opsz" 60, "SOFT" 50' }}
        >
          Mode.
        </h2>
        <p className="mt-4 max-w-lg text-sm leading-relaxed text-ink-2">
          Two visual modes are available. <strong className="text-ink">IT</strong>
          {" "}is the standard editorial presentation. <strong className="text-ink">OT</strong>
          {" "}re-renders the front-end to feel like a plant HMI /
          SCADA screen &mdash; dark panels, mono type, tag names,
          status chips &mdash; the kind of surface Roger builds for
          control rooms.
        </p>
        <div className="mt-6 flex items-center gap-4">
          <ModeToggle current={mode} />
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
            currently <span className="text-ink">{mode}</span>
          </span>
        </div>
        <p className="mt-4 max-w-lg text-xs leading-relaxed text-ink-3">
          Selection persists in a first-party cookie for a year. Toggling
          on any page takes effect immediately across the whole site.
        </p>
      </section>
    </div>
  );
}
