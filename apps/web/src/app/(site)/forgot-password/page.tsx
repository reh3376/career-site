import type { Metadata } from "next";
import Link from "next/link";

import { ForgotForm } from "./form";

export const metadata: Metadata = {
  title: "Forgot password",
  description: "Request a 5-digit reset code for your career-site account.",
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
        Enter the email on your account. We will send a 5-digit code
        that is good for 15 minutes; on the next screen you choose a new
        password and type the code beside it.
      </p>
      <div className="mt-12">
        <ForgotForm />
      </div>
      <p className="mt-10 border-t border-line pt-6 text-sm text-ink-3">
        Already have a code?{" "}
        <Link
          href="/reset-password"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          Enter it here
        </Link>
        . Remembered your password?{" "}
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
