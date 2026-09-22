"use client";

import Link from "next/link";
import { useEffect, useRef, useState } from "react";

import { signOutAction } from "@/app/actions/session";
import { ModeToggle } from "@/components/mode-toggle";
import type { UiMode } from "@/lib/ui-mode";

// Menu shown to the visitor. Grouped by section so the drawer stays
// legible even when it grows. `admin` items only render when isAdmin is
// true; `member` items only when signedIn is true.
type MenuItem =
  | { kind: "link"; label: string; href: string; external?: boolean }
  | { kind: "action"; label: string; action: () => Promise<void> };

type MenuGroup = { label: string; items: MenuItem[] };

// Outbound profile links, resolved on the server (lib/social-links)
// and passed in because this is a client component.
export type SocialProps = {
  linkedinUrl?: string;
  linkedinHandle?: string;
  githubUrl?: string;
  githubHandle?: string;
};

export function HamburgerMenu({
  signedIn,
  isAdmin,
  memberName,
  memberEmail,
  mode,
  social,
}: {
  signedIn: boolean;
  isAdmin: boolean;
  memberName?: string;
  memberEmail?: string;
  mode: UiMode;
  social?: SocialProps;
}) {
  const [open, setOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);

  // Route-change close is handled on each menu item's click (see
  // renderItem below) rather than via a usePathname effect. This
  // satisfies react-hooks/set-state-in-effect and, more importantly,
  // avoids one render cycle where the drawer stays open while the
  // new route is streaming in.

  // Escape and click-outside close.
  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        setOpen(false);
        buttonRef.current?.focus();
      }
    };
    const onClick = (e: MouseEvent) => {
      if (
        panelRef.current &&
        !panelRef.current.contains(e.target as Node) &&
        !buttonRef.current?.contains(e.target as Node)
      ) {
        setOpen(false);
      }
    };
    document.addEventListener("keydown", onKey);
    document.addEventListener("mousedown", onClick);
    return () => {
      document.removeEventListener("keydown", onKey);
      document.removeEventListener("mousedown", onClick);
    };
  }, [open]);

  const groups: MenuGroup[] = buildGroups({ signedIn, isAdmin, social });

  return (
    <>
      <button
        ref={buttonRef}
        type="button"
        aria-label={open ? "Close menu" : "Open menu"}
        aria-expanded={open}
        aria-controls="site-menu-panel"
        onClick={() => setOpen((v) => !v)}
        className="relative inline-flex h-10 w-10 items-center justify-center rounded-md border border-line bg-paper text-ink transition-colors hover:border-accent hover:text-accent"
      >
        {/* Three-stroke hamburger. The middle bar fades and the outer
            two slide together into an X when open, subtle motion cue
            that reinforces the toggle. */}
        <span aria-hidden="true" className="relative block h-4 w-5">
          <span
            className={`absolute left-0 h-[1.5px] w-full bg-current transition-all duration-200 ${
              open ? "top-1/2 -translate-y-1/2 rotate-45" : "top-0"
            }`}
          />
          <span
            className={`absolute left-0 top-1/2 h-[1.5px] w-full -translate-y-1/2 bg-current transition-opacity duration-200 ${
              open ? "opacity-0" : "opacity-100"
            }`}
          />
          <span
            className={`absolute left-0 h-[1.5px] w-full bg-current transition-all duration-200 ${
              open ? "top-1/2 -translate-y-1/2 -rotate-45" : "top-full -translate-y-full"
            }`}
          />
        </span>
      </button>

      {/* Drawer. Panel is anchored to the top-right below the header so
          it never covers the wordmark. On mobile it spans the full width
          minus a gutter; on desktop it caps at ~22rem. */}
      {open ? (
        <div
          id="site-menu-panel"
          ref={panelRef}
          role="menu"
          className="absolute right-4 top-full z-50 mt-2 w-[calc(100vw-2rem)] max-w-sm border border-line bg-paper shadow-lg sm:right-10 sm:mt-3"
        >
          {signedIn && memberEmail ? (
            <div className="border-b border-line px-5 py-4">
              <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                signed in {isAdmin ? "as admin" : "as member"}
              </p>
              <p className="mt-2 truncate text-sm text-ink">
                {memberName || memberEmail}
              </p>
              {memberName ? (
                <p className="truncate text-xs text-ink-3">{memberEmail}</p>
              ) : null}
            </div>
          ) : null}

          <nav aria-label="Site menu" className="py-2">
            {/* Display-mode toggle. Sits at the top of the drawer so
                it's discoverable from every page; also linked from
                /settings for the same effect. */}
            <div className="border-b border-line px-5 pb-3 pt-2">
              <p className="pb-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                display mode
              </p>
              <ModeToggle current={mode} />
            </div>
            {groups.map((group, gi) => (
              <div
                key={group.label}
                className={gi > 0 ? "mt-1 border-t border-line pt-2" : "mt-1"}
              >
                <p className="px-5 pb-1 pt-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  {group.label}
                </p>
                <ul>
                  {group.items.map((item) => (
                    <li key={item.label}>
                      {renderItem(item, () => setOpen(false))}
                    </li>
                  ))}
                </ul>
              </div>
            ))}
          </nav>
        </div>
      ) : null}
    </>
  );
}

function renderItem(item: MenuItem, close: () => void) {
  const base =
    "block w-full px-5 py-2 text-left text-sm text-ink-2 no-underline transition-colors hover:bg-accent-soft hover:text-accent";
  if (item.kind === "link") {
    if (item.external) {
      return (
        <a
          href={item.href}
          className={base}
          rel="noopener noreferrer"
          target="_blank"
          role="menuitem"
          onClick={close}
        >
          {item.label}
        </a>
      );
    }
    return (
      <Link href={item.href} className={base} role="menuitem" onClick={close}>
        {item.label}
      </Link>
    );
  }
  // NOTE: do NOT call close() from the submit button's onClick —
  // setOpen(false) unmounts the <form> synchronously in the same
  // click, which cancels the browser's submit before the server
  // action can fire. signOutAction ends with redirect("/"), so the
  // subsequent client navigation unmounts the drawer naturally.
  return (
    <form action={item.action} className="m-0">
      <button type="submit" className={base} role="menuitem">
        {item.label}
      </button>
    </form>
  );
}

// Compose the menu from the caller's role. Kept as a pure function so
// the shape is testable and easy to reason about; nothing here reads
// state, all inputs come from the caller.
function buildGroups({
  signedIn,
  isAdmin,
  social,
}: {
  signedIn: boolean;
  isAdmin: boolean;
  social?: SocialProps;
}): MenuGroup[] {
  // Anonymous visitors see the evidence: the landing, the writing, the
  // work photos, and the way in. The gallery serves them a subset from
  // the same URL. The reviewer and the member surfaces need an account.
  const browse: MenuItem[] = [
    { kind: "link", label: "Home", href: "/" },
    { kind: "link", label: "Articles", href: "/articles" },
    { kind: "link", label: "Gallery", href: "/gallery" },
    { kind: "link", label: "Contact", href: "/contact" },
  ];
  if (signedIn) {
    browse.push(
      { kind: "link", label: "JD upload", href: "/jd-upload" },
      { kind: "link", label: "How Ask Roger works", href: "/how-ask-roger-works" },
    );
  }
  const groups: MenuGroup[] = [{ label: "browse", items: browse }];

  if (isAdmin) {
    groups.push({
      label: "admin",
      items: [
        { kind: "link", label: "Admin dashboard", href: "/admin" },
        { kind: "link", label: "Contact messages", href: "/admin/contacts" },
        { kind: "link", label: "Registrations", href: "/admin/registrations" },
        { kind: "link", label: "Access & whitelist", href: "/admin/access" },
      ],
    });
  }

  if (signedIn) {
    groups.push({
      label: "your account",
      items: [
        { kind: "link", label: "Member home", href: "/home" },
        { kind: "link", label: "Settings", href: "/settings" },
        { kind: "action", label: "Sign out", action: signOutAction },
      ],
    });
  } else {
    groups.push({
      label: "access",
      items: [
        { kind: "link", label: "Sign in", href: "/login" },
        { kind: "link", label: "Request access", href: "/register" },
      ],
    });
  }

  groups.push({
    label: "elsewhere",
    items: [
      ...(social?.linkedinUrl
        ? [
            {
              kind: "link" as const,
              label: `LinkedIn, ${social.linkedinHandle?.replace(/^linkedin\.com\/in\//, "") ?? "profile"}`,
              href: social.linkedinUrl,
              external: true,
            },
          ]
        : []),
      {
        kind: "link",
        label: `GitHub, ${social?.githubHandle?.replace(/^github\.com\//, "") ?? "reh3376"}`,
        href: social?.githubUrl ?? "https://github.com/reh3376",
        external: true,
      },
      {
        kind: "link",
        label: "This site's repo",
        href: "https://github.com/reh3376/career-site",
        external: true,
      },
    ],
  });

  return groups;
}
