import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { AuthService } from "@/gen/career/v1/auth_pb";
import { SystemService } from "@/gen/career/v1/system_pb";

// The Go API mounts every RPC under /api. In development that resolves to
// the API container on port 8080 (or wherever API_URL points); in production
// it resolves through the reverse proxy on the same origin, so relative
// "/api" is enough and no cross-origin cookies are involved.
const baseUrl =
  process.env.NEXT_PUBLIC_API_URL ?? process.env.API_URL ?? "http://localhost:8080";

const transport = createConnectTransport({
  baseUrl: `${baseUrl}/api`,
});

export const systemClient = createClient(SystemService, transport);
export const authClient = createClient(AuthService, transport);
