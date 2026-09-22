// Browser side of the product event stream (docs/events/README.md).
//
// Events queue in memory and flush in small batches to the api's public
// EventService.Record, through the same-origin /api path Caddy routes
// to the api in every environment. Identity (member, anonymous id) and
// the address hash are attached by the api from cookies and headers;
// nothing here reads them. A flush on pagehide uses sendBeacon so the
// last page.leave survives navigation.

export type EventProps = Record<
  string,
  string | number | boolean | null | undefined
>;

type Queued = {
  eventId: string;
  name: string;
  clientTsMs: number;
  path: string;
  referrer: string;
  uiMode: string;
  propsJson: string;
};

const ENDPOINT = "/api/career.v1.EventService/Record";
const FLUSH_AFTER_MS = 2500;
const MAX_BATCH = 50;

const queue: Queued[] = [];
let timer: ReturnType<typeof setTimeout> | null = null;

function uiMode(): string {
  try {
    const m = document.documentElement.getAttribute("data-mode");
    return m === "ot" ? "ot" : "it";
  } catch {
    return "";
  }
}

function newEventId(): string {
  try {
    if (typeof crypto !== "undefined" && "randomUUID" in crypto)
      return crypto.randomUUID();
  } catch {
    /* fall through */
  }
  return (
    "ev-" +
    Date.now().toString(36) +
    "-" +
    Math.random().toString(36).slice(2, 10)
  );
}

// track queues one event for the current page. Props are limited to
// what the registry allows for the name; the api drops the rest.
export function track(
  name: string,
  props: EventProps = {},
  opts: { flush?: boolean } = {},
): void {
  if (typeof window === "undefined") return;
  const clean: EventProps = {};
  for (const [k, v] of Object.entries(props)) {
    if (v !== undefined && v !== null) clean[k] = v;
  }
  queue.push({
    eventId: newEventId(),
    name,
    clientTsMs: Date.now(),
    path: window.location.pathname + window.location.search,
    referrer: document.referrer || "",
    uiMode: uiMode(),
    propsJson: JSON.stringify(clean),
  });
  if (opts.flush || queue.length >= MAX_BATCH) {
    flush();
    return;
  }
  if (!timer) timer = setTimeout(flush, FLUSH_AFTER_MS);
}

// flush sends whatever is queued. keepalive/sendBeacon let the request
// outlive the page on unload.
export function flush(): void {
  if (timer) {
    clearTimeout(timer);
    timer = null;
  }
  if (queue.length === 0) return;
  const batch = queue.splice(0, MAX_BATCH);
  const body = JSON.stringify({ events: batch });
  try {
    if (
      typeof navigator !== "undefined" &&
      typeof navigator.sendBeacon === "function"
    ) {
      const ok = navigator.sendBeacon(
        ENDPOINT,
        new Blob([body], { type: "application/json" }),
      );
      if (ok) {
        if (queue.length > 0) flush();
        return;
      }
    }
    void fetch(ENDPOINT, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body,
      keepalive: true,
      credentials: "same-origin",
    }).catch(() => {
      /* analytics never surfaces errors to the visitor */
    });
  } catch {
    /* ignore */
  }
  if (queue.length > 0) flush();
}
