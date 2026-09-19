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
    <div className="mx-auto max-w-lg px-6 py-16">
      <h1 className="mb-4 text-3xl font-semibold text-ink">Check your email</h1>
      <p className="mb-6 leading-relaxed text-ink-2">
        {email ? (
          <>
            I sent a verification link to <span className="font-medium text-ink">{email}</span>.
            Click it (or enter the 6-digit code on this page) to move to the review step.
          </>
        ) : (
          <>
            Check your inbox for a verification link. Click it to move to the review step.
          </>
        )}
      </p>
      <p className="mb-8 text-sm text-ink-3">
        The link expires in 24 hours. Not seeing it? Look in spam, or try registering again — the
        email is sent fresh every time.
      </p>
      <Link
        href="/"
        className="text-sm text-accent underline underline-offset-2 hover:text-accent-hover"
      >
        ← Back to home
      </Link>
    </div>
  );
}
