import type { Metadata } from "next";
import Link from "next/link";

import { LoginForm } from "./form";

export const metadata: Metadata = {
  title: "Sign in",
  description: "Sign in to Roger Henley's career portfolio.",
};

export default function LoginPage() {
  return (
    <div className="mx-auto max-w-lg px-6 py-16">
      <h1 className="mb-2 text-3xl font-semibold text-ink">Sign in</h1>
      <p className="mb-8 leading-relaxed text-ink-2">
        Welcome back. If you haven&rsquo;t requested access yet,{" "}
        <Link href="/register" className="text-accent underline underline-offset-2">
          register first
        </Link>
        .
      </p>
      <LoginForm />
    </div>
  );
}
