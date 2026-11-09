import Link from "next/link";

// What a verified person reads. An invited address is active the
// moment it verifies, so it is sent to sign in; everyone else is told
// the truth about the wait. Shared by the link path (server) and the
// code path (client), so the two never drift.
export function Verified({ name, status }: { name: string; status: string }) {
  const first = name ? name.split(" ")[0] : "";
  const active = status === "MEMBER_STATUS_ACTIVE";
  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">email verified</p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {active ? (first ? `You're in, ${first}.` : "You're in.") : first ? `Thanks, ${first}.` : "Thanks."}
      </h1>
      {active ? (
        <>
          <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
            Your email is verified and your access is ready. Sign in to continue.
          </p>
          <p className="mt-8">
            <Link
              href="/login?next=/home"
              className="inline-flex items-center rounded-md bg-accent px-6 py-3 text-sm font-medium text-white no-underline shadow-sm transition-colors hover:bg-accent-hover"
            >
              Sign in
            </Link>
          </p>
        </>
      ) : (
        <>
          <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
            Your email is verified. Roger has been notified and will review your request, usually within a day.
          </p>
          <p className="mt-3 text-sm text-ink-3">
            You&rsquo;ll receive an email either way. Nothing else you need to do right now.
          </p>
          <p className="mt-10">
            <Link
              href="/"
              className="text-sm text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              Back to home
            </Link>
          </p>
        </>
      )}
    </>
  );
}
