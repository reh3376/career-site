// The hidden input Turnstile writes into the enclosing <form>, read
// back by the server actions.
//
// It lives here, in a plain module, rather than beside the widget:
// components/turnstile.tsx is a "use client" module, and a server
// module importing a value from one of those gets a client reference,
// not the string. formData.get() then looks up a key that does not
// exist and the token silently reads as empty, which is a bot-check
// failure on every submission with no error anywhere to explain it.
export const TURNSTILE_FIELD = "cf-turnstile-response";

// Cloudflare Turnstile site key, read on the server and handed to the
// form as a prop.
//
// Deliberately NOT a NEXT_PUBLIC_* variable. Next inlines those at image
// build time, and the CI build has no Cloudflare keys, so a value set
// only in .env.prod would never appear in the bundle. That already
// happened once with the social links, which is why social-links.ts
// carries the same note. Reading it in a server component means the
// running container's environment is the source of truth and a key
// rotation is a restart, not a rebuild.
//
// The site key is public by design (it ships to every visitor in the
// widget markup). The secret half is TURNSTILE_SECRET_KEY and lives
// only in the api container.
export function getTurnstileSiteKey(): string {
  return (process.env.TURNSTILE_SITE_KEY ?? "").trim();
}
