import type { Metadata } from "next";

export const metadata: Metadata = { title: "Admin — Access & whitelist" };

export default function AdminAccessPage() {
  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        surface scaffolded
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Access &amp; whitelist.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        Two related surfaces on one page:
      </p>
      <ul className="mt-5 max-w-xl space-y-3 text-base leading-relaxed text-ink-2">
        <li className="flex gap-3">
          <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" />
          <span>
            <strong className="font-medium text-ink">
              Email whitelist
            </strong>
            {" — "}addresses that auto-approve on registration (D-20,
            ADR-0020). Add, remove, note the reason. Backed by the{" "}
            <code className="font-mono text-ink">access_whitelist</code>
            {" "}table.
          </span>
        </li>
        <li className="flex gap-3">
          <span className="mt-1.5 h-1.5 w-1.5 shrink-0 rounded-full bg-accent" />
          <span>
            <strong className="font-medium text-ink">
              Active access grants
            </strong>
            {" — "}who has access today, when it expires, one-click
            bump the TTL. Backed by{" "}
            <code className="font-mono text-ink">access_grants</code>.
          </span>
        </li>
      </ul>
      <p className="mt-8 max-w-xl text-base leading-relaxed text-ink-3">
        Handlers land next PR alongside the contact-messages surface.
      </p>
    </>
  );
}
