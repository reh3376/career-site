import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Check your email",
  description: "Verify your email to finish registering.",
};

export default async function CheckEmailPage({
  searchParams,
}: {
  searchParams: Promise<{ email?: string }>;
}) {
  const { email } = await searchParams;

  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        next step
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Check your email.
      </h1>
      <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
        {email ? (
          <>
            I sent a verification link to{" "}
            <span className="font-mono text-ink">{email}</span>. Click it to
            move to the review step.
          </>
        ) : (
          <>
            Check your inbox for a verification link. Click it to move to the
            review step.
          </>
        )}
      </p>
      <p className="mt-4 text-sm text-ink-3">
        The link expires in 24 hours. Not seeing it? Check spam, or try
        registering again, the email is sent fresh every time.
      </p>
      <p className="mt-10">
        <Link
          href="/"
          className="text-sm text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          Back to home
        </Link>
      </p>
    </div>
  );
}
