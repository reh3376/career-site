import type { Metadata } from "next";

export const metadata: Metadata = { title: "Admin — Registrations" };

export default function AdminRegistrationsPage() {
  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        surface scaffolded
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Registrations.
      </h1>
      <p className="mt-6 max-w-xl text-base leading-relaxed text-ink-2">
        Pending applicants and recent decisions in one view. Currently
        every request routes through the one-click Accept / Decline
        email link (see{" "}
        <code className="font-mono text-ink">/admin/decision</code>);
        this surface adds a durable console so approvals aren&rsquo;t
        only in Gmail. Filter by status, sort by age, override an
        expired grant. Backed by{" "}
        <code className="font-mono text-ink">ListMembers</code>
        {" — "}already declared in{" "}
        <code className="font-mono text-ink">admin.proto</code>, needs
        the handler.
      </p>
    </>
  );
}
