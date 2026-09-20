import type { Metadata } from "next";
import Link from "next/link";

import { OtPanel } from "@/components/ot-panel";
import { getUiMode } from "@/lib/ui-mode";

import { LoginForm } from "./form";

export const metadata: Metadata = {
  title: "Sign in",
  description: "Sign in to Roger Henley's career portfolio.",
};

export default async function LoginPage({
  searchParams,
}: {
  searchParams: Promise<{ next?: string }>;
}) {
  const [{ next }, mode] = await Promise.all([searchParams, getUiMode()]);

  if (mode === "ot") {
    return (
      <OtPanel tag="AUTH-01" title="SESSION.NEW · SIGN IN" note="30-day cookie">
        <LoginForm next={next} />
        <p className="mt-8 border-t border-line pt-4 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
          no account?{" "}
          <Link
            href="/register"
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            request access
          </Link>
        </p>
      </OtPanel>
    );
  }

  return (
    <div className="mx-auto max-w-xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
        welcome back
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        Sign in.
      </h1>
      <p className="mt-6 max-w-md text-base leading-relaxed text-ink-2">
        If you haven&rsquo;t requested access yet,{" "}
        <Link
          href="/register"
          className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
        >
          register first
        </Link>
        .
      </p>
      <div className="mt-12">
        <LoginForm next={next} />
      </div>
    </div>
  );
}
