import type { Metadata } from "next";
import Link from "next/link";

import { ForgotForm } from "./form";

export const metadata: Metadata = {
  title: "Forgot password",
  description: "Request a password-reset link for your career-site account.",
};

export default function ForgotPasswordPage() {
  return (
    <div className="mx-auto max-w-xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        account recovery
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Forgot password.
      </h1>
      <p className="mt-6 max-w-md text-base leading-relaxed text-ink-2">
        Enter the email on your account and we&rsquo;ll send a
        single-use link that&rsquo;s good for 60 minutes.
      </p>
      <div className="mt-12">
        <ForgotForm />
      </div>
      <p className="mt-10 border-t border-line pt-6 text-sm text-ink-3">
        Remembered it?{" "}
        <Link
          href="/login"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          Back to sign in
        </Link>
        .
      </p>
    </div>
  );
}
