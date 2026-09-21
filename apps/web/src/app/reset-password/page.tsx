import type { Metadata } from "next";
import Link from "next/link";

import { ResetForm } from "./form";

export const metadata: Metadata = {
  title: "Reset password",
  description: "Set a new password for your career-site account.",
};

export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ token?: string }>;
}) {
  const { token } = await searchParams;

  if (!token) {
    return (
      <div className="mx-auto max-w-xl px-6 py-20 sm:px-10 sm:py-28">
        <h1
          className="font-display text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
          style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
        >
          Missing token.
        </h1>
        <p className="mt-6 max-w-md text-base leading-relaxed text-ink-2">
          This page needs a reset link from the email we sent. If you
          have one, open it directly. Otherwise{" "}
          <Link
            href="/forgot-password"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            request a new link
          </Link>
          .
        </p>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        account recovery
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Set a new password.
      </h1>
      <p className="mt-6 max-w-md text-base leading-relaxed text-ink-2">
        Once you save, every other browser signed into this account
        will be logged out, including any lingering session from
        whoever asked for the reset link.
      </p>
      <div className="mt-12">
        <ResetForm token={token} />
      </div>
    </div>
  );
}
