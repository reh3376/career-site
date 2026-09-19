import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "Request access",
  description: "Register to request access to Roger Henley's career portfolio.",
};

// Placeholder. The real registration form lands in Task 2 of Phase 1 —
// name, email, password, organization, stated role, consent, Turnstile,
// then the four-step approval-gated flow.
export default function RegisterPage() {
  return (
    <div className="mx-auto max-w-lg px-6 py-16">
      <h1 className="mb-6 text-3xl font-semibold text-ink">Request access</h1>
      <div className="space-y-4 leading-relaxed text-ink-2">
        <p>
          The registration form is being built. It&rsquo;ll ask for your name, email, organization,
          and role, then send a review request to Roger. Approval usually takes a day; you&rsquo;ll
          receive an email when your account is ready.
        </p>
        <p className="text-sm text-ink-3">
          Want a heads-up when it&rsquo;s ready? Email{" "}
          <a
            href="mailto:rogerhenley345@gmail.com"
            className="text-accent underline underline-offset-2"
          >
            rogerhenley345@gmail.com
          </a>
          .
        </p>
      </div>
      <Link
        href="/"
        className="mt-8 inline-flex text-sm text-accent underline underline-offset-2 hover:text-accent-hover"
      >
        ← Back to home
      </Link>
    </div>
  );
}
