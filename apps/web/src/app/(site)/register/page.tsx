import type { Metadata } from "next";
import Link from "next/link";

import { OtPanel } from "@/components/ot-panel";
import { getUiMode } from "@/lib/ui-mode";

import { RegisterForm } from "./form";

export const metadata: Metadata = {
  title: "Request access",
  description: "Register to request access to Roger Henley's career portfolio.",
};

export default async function RegisterPage() {
  const mode = await getUiMode();

  if (mode === "ot") {
    return (
      <OtPanel
        tag="ACC-01"
        title="ACCESS.REQ · REGISTER"
        note="reviewed by admin"
      >
        <RegisterForm />
        <p className="mt-8 border-t border-line pt-4 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          already registered?{" "}
          <Link
            href="/login"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            sign in
          </Link>
        </p>
      </OtPanel>
    );
  }

  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        access is reviewed
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Request access.
      </h1>
      <p className="mt-6 max-w-lg text-base leading-relaxed text-ink-2">
        A short intake so I know who&rsquo;s reaching out. I get an email when
        you submit; I&rsquo;ll usually reply within a day.
      </p>
      <div className="mt-12">
        <RegisterForm />
      </div>
      <p className="mt-10 border-t border-line pt-6 text-sm text-ink-3">
        Already registered?{" "}
        <Link
          href="/login"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          Sign in
        </Link>
        .
      </p>
    </div>
  );
}
