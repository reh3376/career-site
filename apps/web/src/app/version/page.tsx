import { create } from "@bufbuild/protobuf";

import { GetVersionRequestSchema } from "@/gen/career/v1/system_pb";
import { systemClient } from "@/lib/api";

// Server-rendered proof of end-to-end wiring: this page calls the Go API's
// SystemService.GetVersion via Connect from a React Server Component and
// prints what it got back. Phase 0 smoke; will move under /admin later.
export const dynamic = "force-dynamic";

async function fetchVersion(): Promise<
  { ok: true; version: string; commit: string; goVersion: string } | { ok: false; error: string }
> {
  try {
    const resp = await systemClient.getVersion(create(GetVersionRequestSchema, {}));
    return {
      ok: true,
      version: resp.version,
      commit: resp.commit,
      goVersion: resp.goVersion,
    };
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : String(err) };
  }
}

export default async function VersionPage() {
  const result = await fetchVersion();

  return (
    <div className="mx-auto max-w-2xl px-6 py-20 sm:px-10 sm:py-28">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-accent">
        health check
      </p>
      <h1
        className="font-display mt-4 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        API version.
      </h1>
      {result.ok ? (
        <dl className="mt-10 grid gap-4 border-t border-line pt-6 font-mono text-sm text-ink-2">
          <div className="flex gap-4">
            <dt className="min-w-[6rem] text-ink-3">version</dt>
            <dd className="m-0 text-ink">{result.version || "(empty)"}</dd>
          </div>
          <div className="flex gap-4">
            <dt className="min-w-[6rem] text-ink-3">commit</dt>
            <dd className="m-0 text-ink">{result.commit || "(empty)"}</dd>
          </div>
          <div className="flex gap-4">
            <dt className="min-w-[6rem] text-ink-3">go</dt>
            <dd className="m-0 text-ink">{result.goVersion || "(empty)"}</dd>
          </div>
        </dl>
      ) : (
        <p className="mt-10 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
          API unreachable: <span className="font-mono">{result.error}</span>
        </p>
      )}
    </div>
  );
}
