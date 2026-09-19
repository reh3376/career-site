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
    <main style={{ padding: "3rem 1.5rem", maxWidth: "40rem", margin: "0 auto" }}>
      <h1 style={{ fontSize: "1.25rem" }}>API version</h1>
      {result.ok ? (
        <dl style={{ fontFamily: "monospace", lineHeight: 1.8 }}>
          <dt style={{ fontWeight: "bold" }}>version</dt>
          <dd style={{ margin: 0 }}>{result.version || "(empty)"}</dd>
          <dt style={{ fontWeight: "bold", marginTop: "0.75rem" }}>commit</dt>
          <dd style={{ margin: 0 }}>{result.commit || "(empty)"}</dd>
          <dt style={{ fontWeight: "bold", marginTop: "0.75rem" }}>go</dt>
          <dd style={{ margin: 0 }}>{result.goVersion || "(empty)"}</dd>
        </dl>
      ) : (
        <p style={{ color: "#c00" }}>API unreachable: {result.error}</p>
      )}
    </main>
  );
}
