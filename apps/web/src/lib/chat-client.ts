import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { ChatService } from "@/gen/career/v1/chat_pb";

// Ask Roger's browser client.
//
// Separate from src/lib/api.ts, and the difference is the base URL. That
// transport resolves an absolute URL from NEXT_PUBLIC_API_URL / API_URL,
// which is right for code that may run on the server and wrong in a
// browser: API_URL is a server-only variable naming the container
// (http://api:8080), so a client component that picked it up would ask
// the visitor's browser to resolve a Docker hostname.
//
// Caddy serves /api/* from the same origin as the site, so a relative
// base is both correct and the reason session cookies are sent at all:
// they are first-party here and would not be cross-origin.
const transport = createConnectTransport({ baseUrl: "/api" });

export const chatClient = createClient(ChatService, transport);
