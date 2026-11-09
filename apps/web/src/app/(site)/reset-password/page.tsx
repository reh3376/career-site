import type { Metadata } from "next";
import Link from "next/link";

import { ResetForm } from "./form";

export const metadata: Metadata = {
  title: "Reset password",
  description: "Set a new password for your career-site account with the emailed code.",
};

export default async function ResetPasswordPage({
  searchParams,
}: {
  searchParams: Promise<{ email?: string; sent?: string }>;
}) {
  const { email = "", sent } = await searchParams;

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
      {sent ? (
        <p className="mt-6 border-l-2 border-accent bg-accent/10 px-4 py-3 text-sm text-ink">
          If an account exists for that email, a 5-digit code is on its
          way. Choose your new password below and type the code beside
          it within 15 minutes.
        </p>
      ) : null}
      <p className="mt-6 max-w-md text-base leading-relaxed text-ink-2">
        Once you save, every other browser signed into this account is
        signed out, including any session left by whoever asked for the
        code.
      </p>
      <div className="mt-12">
        <ResetForm email={email} />
      </div>
      <p className="mt-10 border-t border-line pt-6 text-sm text-ink-3">
        No code arrived after a couple of minutes? Check your spam
        folder, or{" "}
        <Link
          href="/forgot-password"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          request a new one
        </Link>
        .
      </p>
    </div>
  );
}
