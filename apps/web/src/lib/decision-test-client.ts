import { createClient } from "@connectrpc/connect";
import { createConnectTransport } from "@connectrpc/connect-web";

import { DecisionTestService } from "@/gen/career/v1/decision_test_pb";

// The decision test's browser client.
//
// Relative base, like chat-client.ts and for the same reason: Caddy
// serves /api/* from this origin, so the request is first-party and the
// anonymous id cookie is sent. An absolute URL from API_URL would name
// a Docker hostname the visitor's browser cannot resolve.
const transport = createConnectTransport({ baseUrl: "/api" });

export const decisionTestClient = createClient(DecisionTestService, transport);
