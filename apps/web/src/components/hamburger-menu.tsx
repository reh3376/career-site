"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";

import { signOutAction } from "@/app/actions/session";

// Menu shown to the visitor. Grouped by section so the drawer stays
// legible even when it grows. `admin` items only render when isAdmin is
// true; `member` items only when signedIn is true.
type MenuItem =
  | { kind: "link"; label: string; href: string; external?: boolean }
  | { kind: "action"; label: string; action: () => Promise<void> };

type MenuGroup = { label: string; items: MenuItem[] };

export function HamburgerMenu({
  signedIn,
  isAdmin,
  memberName,
  memberEmail,
}: {
  signedIn: boolean;
  isAdmin: boolean;
  memberName?: string;
  memberEmail?: string;
}) {
  const [open, setOpen] = useState(false);
  const panelRef = useRef<HTMLDivElement>(null);
  const buttonRef = useRef<HTMLButtonElement>(null);
  const pathname = usePathname();

  // Close on route change (Next.js keeps the layout mounted across
  // navigation, so we clear the drawer ourselves).
  useEffect(() => {
    setOpen(false);
  }, [pathname]);

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

  const groups: MenuGroup[] = buildGroups({ signedIn, isAdmin });

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
            two slide together into an X when open — subtle motion cue
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
            {groups.map((group, gi) => (
              <div
                key={group.label}
                className={gi > 0 ? "mt-1 border-t border-line pt-2" : ""}
              >
                <p className="px-5 pb-1 pt-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
                  {group.label}
                </p>
                <ul>
                  {group.items.map((item) => (
                    <li key={item.label}>{renderItem(item)}</li>
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

function renderItem(item: MenuItem) {
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
        >
          {item.label}
        </a>
      );
    }
    return (
      <Link href={item.href} className={base} role="menuitem">
        {item.label}
      </Link>
    );
  }
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
}: {
  signedIn: boolean;
  isAdmin: boolean;
}): MenuGroup[] {
  const groups: MenuGroup[] = [
    {
      label: "browse",
      items: [
        { kind: "link", label: "Home", href: "/" },
        { kind: "link", label: "Contact", href: "/contact" },
        { kind: "link", label: "How Ask Roger works", href: "/how-ask-roger-works" },
      ],
    },
  ];

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
      {
        kind: "link",
        label: "GitHub — reh3376",
        href: "https://github.com/reh3376",
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
