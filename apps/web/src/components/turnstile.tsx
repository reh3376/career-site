"use client";

import Script from "next/script";
import { useEffect, useRef } from "react";

// Cloudflare Turnstile widget, rendered explicitly rather than by the
// `cf-turnstile` class scan.
//
// Explicit rendering is the whole reason this file exists: a token is
// single-use, so after a submission the form rejects (a duplicate
// email, a short password, anything) the token on the page is spent,
// and the retry fails the bot check even though the visitor did nothing
// wrong. Explicit rendering gives us a widget id to call reset() on,
// which the forms trigger through `resetOn`.
//
// The widget writes its own hidden input named TURNSTILE_FIELD into
// the enclosing <form>, which is what the server actions read.
//
// There is no React state here on purpose. Everything this component
// touches is an external system (a third-party script and the DOM node
// it owns), so it is all refs and effects; a `ready` state would just
// re-render the page to re-run an effect that can check the same
// condition itself.

const SCRIPT_SRC =
  "https://challenges.cloudflare.com/turnstile/v0/api.js?render=explicit";

// How often to look for the script's global, and for how long. The
// script may already be on the page from an earlier client-side
// navigation, in which case no load event fires for this mount, so
// polling is more reliable here than any one callback.
const POLL_MS = 150;
const POLL_LIMIT_MS = 15_000;

type TurnstileAPI = {
  render: (el: HTMLElement, opts: Record<string, unknown>) => string | undefined;
  reset: (id?: string) => void;
  remove: (id?: string) => void;
};

declare global {
  interface Window {
    turnstile?: TurnstileAPI;
  }
}

export function Turnstile({
  siteKey,
  action,
  resetOn,
  onToken,
}: {
  siteKey: string;
  // Labels the surface in Cloudflare's analytics, so a spike on
  // registration is distinguishable from a spike on contact.
  action: string;
  // Any value whose identity changes when the form needs a fresh
  // token. The forms pass their action state, which useActionState
  // replaces on every submission. The first render never resets.
  resetOn?: unknown;
  // Called with the token each time the widget solves, and with "" when
  // it expires.
  //
  // The hidden input the widget writes into the enclosing <form> covers
  // every caller that submits a form. The decision test does not: its
  // briefing calls decisionTestClient.startSession straight from the
  // browser, so it needs the value in hand rather than in the DOM.
  onToken?: (token: string) => void;
}) {
  const boxRef = useRef<HTMLDivElement | null>(null);
  const widgetId = useRef<string | undefined>(undefined);
  // Held in a ref so a caller passing an inline arrow does not
  // re-render the widget on every parent render. Assigned in an effect
  // rather than during render, because a render may be discarded and
  // writing a ref from one is not safe.
  const onTokenRef = useRef(onToken);
  useEffect(() => {
    onTokenRef.current = onToken;
  }, [onToken]);

  useEffect(() => {
    if (!siteKey) return;
    let cancelled = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    let waited = 0;

    const attempt = () => {
      if (cancelled || widgetId.current !== undefined) return;
      const api = window.turnstile;
      if (!api || !boxRef.current) {
        if (waited >= POLL_LIMIT_MS) return;
        waited += POLL_MS;
        timer = setTimeout(attempt, POLL_MS);
        return;
      }
      widgetId.current = api.render(boxRef.current, {
        sitekey: siteKey,
        action,
        theme: "auto",
        // A token expires after 300 seconds. Without this, someone who
        // fills a long form in another tab comes back to a dead widget
        // and a rejection they cannot explain.
        "refresh-expired": "auto",
        callback: (token: string) => onTokenRef.current?.(token),
        "expired-callback": () => onTokenRef.current?.(""),
        "error-callback": () => onTokenRef.current?.(""),
      });
    };
    attempt();

    return () => {
      cancelled = true;
      if (timer) clearTimeout(timer);
      if (widgetId.current !== undefined) {
        window.turnstile?.remove(widgetId.current);
        widgetId.current = undefined;
      }
    };
  }, [siteKey, action]);

  // Reset on every change of `resetOn` except the first render. The
  // effect's dependency array already does the change detection; the
  // ref only suppresses the mount run, when there is no widget and
  // nothing to reset.
  const mounted = useRef(false);
  useEffect(() => {
    if (!mounted.current) {
      mounted.current = true;
      return;
    }
    if (widgetId.current !== undefined) {
      window.turnstile?.reset(widgetId.current);
    }
  }, [resetOn]);

  // No site key configured: render nothing. Dev runs this way, and the
  // API is wired to turnstile.Noop in exactly the same case, so the two
  // halves agree. Production cannot reach it: the API refuses to boot
  // without the secret half.
  if (!siteKey) return null;

  return (
    <div>
      <Script id="cf-turnstile" src={SCRIPT_SRC} strategy="afterInteractive" />
      <div ref={boxRef} />
    </div>
  );
}
