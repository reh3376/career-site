import type { Metadata } from "next";
import Link from "next/link";

import { RegisterForm } from "./form";

export const metadata: Metadata = {
  title: "Request access",
  description: "Register to request access to Roger Henley's career portfolio.",
};

export default function RegisterPage() {
  return (
    <div className="mx-auto max-w-lg px-6 py-16">
      <h1 className="mb-2 text-3xl font-semibold text-ink">Request access</h1>
      <p className="mb-8 leading-relaxed text-ink-2">
        Tell me a bit about yourself. I&rsquo;ll get an email and reply within a day or so.
      </p>
      <RegisterForm />
      <p className="mt-8 text-sm text-ink-3">
        Already registered?{" "}
        <Link href="/login" className="text-accent underline underline-offset-2">
          Sign in
        </Link>
        .
      </p>
    </div>
  );
}
