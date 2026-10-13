import fs from "node:fs/promises";
import path from "node:path";

import { parseFrontMatter } from "./frontmatter";

// Awards, education, service, certifications and eligibility
// (FR-CNT-10).
//
// One kind rather than five directories, because they are read
// together on one page and the only real difference between them is
// which fields apply. An award has a year and no status; a
// certification has a status and often no year; an eligibility
// statement has neither.
//
// The row says "as the owner chooses to publish them", which is the
// part that matters: nothing here is derived or inferred, every entry
// is a file somebody wrote on purpose, and removing one is deleting a
// file rather than finding the flag that hides it.

export type CredentialCategory =
  | "award"
  | "education"
  | "certification"
  | "service"
  | "eligibility";

export type CredentialStatus = "current" | "lapsed" | "completed";

export type Credential = {
  id: string;
  title: string;
  category: CredentialCategory;
  issuer: string;
  awarded: number;
  status: CredentialStatus | "";
  note: string;
};

const DIR = path.join(process.cwd(), "content", "credentials");

const CATEGORIES = new Set<CredentialCategory>([
  "award",
  "education",
  "certification",
  "service",
  "eligibility",
]);
const STATUSES = new Set<CredentialStatus>(["current", "lapsed", "completed"]);

// Section headings, and the order the page reads in. Awards first
// because they are the part a reader came for; eligibility last
// because it is an answer to a question rather than an achievement.
export const SECTIONS: { category: CredentialCategory; heading: string }[] = [
  { category: "award", heading: "Recognition" },
  { category: "education", heading: "Education" },
  { category: "certification", heading: "Certifications" },
  { category: "service", heading: "Service" },
  { category: "eligibility", heading: "Availability" },
];

// Only "lapsed" is shown. "completed" on a degree would be noise, and
// a certification that is current needs no qualifier. A lapsed one
// does, and saying so is the point of publishing it at all.
export const STATUS_LABEL: Partial<Record<CredentialStatus, string>> = {
  lapsed: "lapsed",
};

function toInt(v: string | undefined): number {
  const n = Number.parseInt((v ?? "").trim(), 10);
  return Number.isFinite(n) ? n : 0;
}

export async function listCredentials(): Promise<Credential[]> {
  let names: string[];
  try {
    names = await fs.readdir(DIR);
  } catch {
    return [];
  }

  const rows: { order: number; credential: Credential }[] = [];
  for (const name of names.filter((n) => n.endsWith(".md"))) {
    const raw = await fs.readFile(path.join(DIR, name), "utf8");
    const { meta, body } = parseFrontMatter(raw);
    if ((meta.visibility ?? "").trim() !== "public") continue;

    const category = (meta.category ?? "").trim() as CredentialCategory;
    if (!CATEGORIES.has(category)) continue;

    const status = (meta.status ?? "").trim() as CredentialStatus;

    rows.push({
      order: toInt(meta.order),
      credential: {
        id: (meta.id ?? name).trim(),
        title: (meta.title ?? "").trim(),
        category,
        issuer: (meta.issuer ?? "").trim(),
        awarded: toInt(meta.awarded),
        status: STATUSES.has(status) ? status : "",
        note: body.trim(),
      },
    });
  }

  rows.sort((a, b) => a.order - b.order);
  return rows.map((r) => r.credential);
}

export function inSection(
  credentials: Credential[],
  category: CredentialCategory,
): Credential[] {
  return credentials.filter((c) => c.category === category);
}
