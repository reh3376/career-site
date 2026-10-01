import { describe, expect, it } from "vitest";

import {
  CRAWLER_DISALLOW,
  INDEXABLE_PATHS,
  isMemberPath,
  isPublicPath,
  PUBLIC_PATHS,
} from "./public-routes";

// The access policy, which is the one piece of front-end logic where a
// mistake is not cosmetic: too loose and a member page is served to
// anyone, too tight and a real page 404s for the person it was built
// for.
//
// `apps/web` had no tests at all when these were written. These are
// first because the proxy had just changed: it used to redirect every
// unknown path to sign-in, so a stale link took a visitor's password
// and then showed them a 404.

describe("isPublicPath", () => {
  it("admits the pages a hiring manager needs before deciding to register", () => {
    for (const p of [
      "/",
      "/articles",
      "/gallery",
      "/how-ask-roger-works",
      "/contact",
      "/privacy",
      "/terms",
    ]) {
      expect(isPublicPath(p), `${p} should be public`).toBe(true);
    }
  });

  it("admits article slugs, which is the whole point of publishing them", () => {
    expect(isPublicPath("/articles/better-business-decisions-part-1")).toBe(
      true,
    );
  });

  it("refuses the things an account buys", () => {
    for (const p of ["/jd-upload", "/ask", "/home", "/settings", "/meetings"]) {
      expect(isPublicPath(p), `${p} must not be public`).toBe(false);
    }
  });

  it("refuses the admin console, including nested pages", () => {
    for (const p of ["/admin", "/admin/decisions", "/admin/db", "/admin/qa"]) {
      expect(isPublicPath(p), `${p} must not be public`).toBe(false);
    }
  });

  // /admin/decision is the one-click Accept/Decline link out of an
  // email. It is public because the owner opens it from his phone
  // without a session, and it carries its own token. The neighbouring
  // /admin/decisions, plural, is the review console and is not.
  it("separates the token-bearing decision link from the review console", () => {
    expect(isPublicPath("/admin/decision")).toBe(true);
    expect(isPublicPath("/admin/decision/abc123")).toBe(true);
    expect(isPublicPath("/admin/decisions")).toBe(false);
  });
});

describe("isMemberPath", () => {
  it("claims the gated pages that exist", () => {
    for (const p of [
      "/home",
      "/ask",
      "/jd-upload",
      "/jd-upload/42",
      "/meetings",
      "/settings",
      "/admin",
      "/admin/corpus",
    ]) {
      expect(isMemberPath(p), `${p} should be gated`).toBe(true);
    }
  });

  // The reason this function exists. These three were named in the
  // roadmap as paths that have never existed and still demanded a
  // sign-in before showing a 404.
  it("disclaims paths that have never existed", () => {
    for (const p of ["/timeline", "/projects", "/downloads", "/nonsense"]) {
      expect(isMemberPath(p), `${p} should 404, not redirect`).toBe(false);
    }
  });
});

describe("the three lists that have to agree", () => {
  it("never invites a crawler to a page that is not public", () => {
    for (const p of INDEXABLE_PATHS) {
      expect(isPublicPath(p), `${p} is in the sitemap but not public`).toBe(
        true,
      );
    }
  });

  it("never disallows a crawler from a page it also invites", () => {
    for (const p of INDEXABLE_PATHS) {
      // Widened to string: the literal unions cannot overlap today,
      // which is the property under test, and tsc rejects a comparison
      // it can already prove false.
      const path: string = p;
      const clash = (CRAWLER_DISALLOW as readonly string[]).find(
        (d) => path === d || (d !== "/" && path.startsWith(d)),
      );
      expect(clash, `${p} is both indexable and disallowed`).toBeUndefined();
    }
  });

  // A page cannot be both sides of the gate. This would have caught a
  // member path accidentally pasted into PUBLIC_PATHS.
  it("never calls the same path public and gated", () => {
    for (const p of PUBLIC_PATHS) {
      expect(isMemberPath(p), `${p} is both public and gated`).toBe(false);
    }
  });
});
