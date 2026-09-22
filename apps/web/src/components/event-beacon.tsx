"use client";

import { usePathname } from "next/navigation";
import { useEffect, useRef } from "react";

import { flush, track } from "@/lib/events-client";

// Mounted once in the root layout. Emits page.view on every route,
// page.leave with dwell when the route changes or the page hides, and
// two click-derived events: landing.cta_click for internal links on
// the landing page (or any link carrying data-event) and link.external
// for links that leave the site. Nothing here identifies the visitor;
// the api attaches identity from cookies (docs/events/README.md).
export function EventBeacon() {
  const pathname = usePathname();
  const enteredAt = useRef<number>(0);
  const lastPath = useRef<string>("");

  useEffect(() => {
    const now = Date.now();
    if (lastPath.current && lastPath.current !== pathname) {
      track("page.leave", { dwell_ms: now - enteredAt.current });
    }
    lastPath.current = pathname;
    enteredAt.current = now;
    track("page.view", {
      title:
        typeof document !== "undefined" ? document.title.slice(0, 120) : "",
    });
  }, [pathname]);

  useEffect(() => {
    const leave = () => {
      if (!lastPath.current) return;
      track(
        "page.leave",
        { dwell_ms: Date.now() - enteredAt.current },
        { flush: true },
      );
      // The next visibilitychange back to visible should not double count.
      enteredAt.current = Date.now();
    };
    const onVisibility = () => {
      if (document.visibilityState === "hidden") leave();
    };
    const onClick = (ev: MouseEvent) => {
      const target = ev.target as Element | null;
      const a = target?.closest?.("a[href]") as HTMLAnchorElement | null;
      if (!a) return;
      const explicit = a.dataset.event;
      let url: URL;
      try {
        url = new URL(a.href, window.location.href);
      } catch {
        return;
      }
      const external = url.origin !== window.location.origin;
      if (external) {
        track("link.external", { host: url.host }, { flush: true });
        return;
      }
      if (explicit) {
        track(
          explicit,
          { cta: a.dataset.cta ?? url.pathname, href: url.pathname },
          { flush: true },
        );
        return;
      }
      if (window.location.pathname === "/") {
        track(
          "landing.cta_click",
          { cta: a.dataset.cta ?? url.pathname, href: url.pathname },
          { flush: true },
        );
      }
    };
    window.addEventListener("pagehide", leave);
    document.addEventListener("visibilitychange", onVisibility);
    document.addEventListener("click", onClick, true);
    return () => {
      window.removeEventListener("pagehide", leave);
      document.removeEventListener("visibilitychange", onVisibility);
      document.removeEventListener("click", onClick, true);
      flush();
    };
  }, []);

  return null;
}
