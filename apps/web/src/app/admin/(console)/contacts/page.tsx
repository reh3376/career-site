import type { Metadata } from "next";

export const metadata: Metadata = { title: "Admin — Contact messages" };

// Placeholder shell for the contact-messages surface. Real data lands
// as soon as the AdminService.ListContactMessages + MarkResolved RPCs
// are wired (next PR).
export default function AdminContactsPage() {
  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        surface scaffolded
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Contact messages.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        The contact form is live at{" "}
        <code className="font-mono text-ink">/contact</code> and every
        submission already lands in Postgres (see the
        <code className="ml-1 font-mono text-ink">support_messages</code>
        table). This surface will list them, filter by category, show
        the resolved / unresolved status, and one-click open a mail
        reply. Ships next PR alongside the{" "}
        <code className="font-mono text-ink">ListContactMessages</code>
        {" "}and{" "}
        <code className="font-mono text-ink">MarkResolved</code>
        {" "}RPCs.
      </p>
    </>
  );
}
