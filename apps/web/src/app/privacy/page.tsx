import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Privacy",
  description: "How this site collects, stores, and handles your information.",
};

export default function PrivacyPage() {
  return (
    <div className="mx-auto max-w-2xl px-6 py-16">
      <h1 className="mb-8 text-3xl font-semibold text-ink">Privacy</h1>
      <div className="space-y-6 leading-relaxed text-ink-2">
        <p>
          This site is under construction. The full privacy policy — covering what is collected,
          how long it&rsquo;s retained, how to export or delete your data, and how conversations
          with Ask Roger are stored — ships alongside the launch (late 2026).
        </p>
        <p className="text-sm text-ink-3">
          Until then, the only data collected from a request-access form is your name, email,
          organization, and stated role. It is used only to review your access request and, if
          approved, to send you a sign-in notification. Nothing is sold, shared with third parties,
          or used for advertising.
        </p>
        <p className="text-sm text-ink-3">
          Questions?{" "}
          <a
            href="mailto:rogerhenley345@gmail.com"
            className="text-accent underline underline-offset-2"
          >
            rogerhenley345@gmail.com
          </a>
        </p>
      </div>
    </div>
  );
}
