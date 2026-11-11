"use server";

import { revalidatePath } from "next/cache";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

// The requirement editor's two writes (FSD FR-HF-03, FR-HF-23): save an
// edit as a new draft version, and release the drafts for judging.
//
// The form posts every field. Only the ones that differ from the
// version being edited go into the patch, because the server derives
// thresholds, the evidence standard and the catalogue's equivalents
// again when the wording changes, and it only does that for a field the
// patch leaves alone. Sending back an untouched field would freeze it.

export type Thresholds = {
  minYears?: number;
  recencyYears?: number;
  minScope?: { unit?: string; value?: number };
  credentialLevel?: string;
  credentialField?: string;
  relationshipKind?: string;
};

export type Original = {
  text: string;
  type: string;
  requirementClass: string;
  weight: number;
  thresholds: Thresholds;
  equivalency: string;
  equivalenceNotes: string;
  evidenceStandard: string;
  alternatives: string[];
  namedParties: string[];
};

export type EditState = { saved?: boolean; error?: string };

function lines(raw: FormDataEntryValue | null): string[] {
  return String(raw ?? "")
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean);
}

function optionalNumber(raw: FormDataEntryValue | null): number | undefined {
  const s = String(raw ?? "").trim();
  if (!s) return undefined;
  const n = Number(s);
  return Number.isFinite(n) ? n : NaN;
}

// One canonical spelling of a set of thresholds, so "unchanged" can be
// decided by comparing strings.
function canonical(t: Thresholds): string {
  const scope =
    t.minScope && t.minScope.unit && (t.minScope.value ?? 0) > 0
      ? { unit: t.minScope.unit, value: t.minScope.value }
      : undefined;
  return JSON.stringify({
    minYears: t.minYears || undefined,
    recencyYears: t.recencyYears || undefined,
    minScope: scope,
    credentialLevel: t.credentialLevel || undefined,
    credentialField: (t.credentialField ?? "").trim() || undefined,
    relationshipKind: t.relationshipKind || undefined,
  });
}

async function errorText(resp: Response): Promise<string> {
  try {
    const j = (await resp.json()) as { message?: string };
    if (j.message) return j.message;
  } catch {
    /* fall through */
  }
  return `HTTP ${resp.status}`;
}

export async function updateRequirementAction(
  _prev: EditState,
  formData: FormData,
): Promise<EditState> {
  const requirementId = String(formData.get("requirement_id") ?? "").trim();
  const editReason = String(formData.get("edit_reason") ?? "").trim();
  if (!requirementId) return { error: "The requirement is missing." };
  if (!editReason) return { error: "Say why you are changing it." };

  let original: Original;
  try {
    original = JSON.parse(String(formData.get("original") ?? "")) as Original;
  } catch {
    return { error: "The form is out of date. Reload the page and try again." };
  }

  const patch: Record<string, unknown> = {};

  const text = String(formData.get("text") ?? "").trim();
  if (!text) return { error: "The requirement needs its wording." };
  if (text !== original.text) patch.text = text;

  const type = String(formData.get("type") ?? "").trim();
  if (type && type !== original.type) patch.type = type;

  const requirementClass = String(formData.get("requirement_class") ?? "").trim();
  if (requirementClass && requirementClass !== original.requirementClass)
    patch.requirementClass = requirementClass;

  const weight = Number(String(formData.get("weight") ?? "").trim());
  if (!Number.isInteger(weight) || weight < 1 || weight > 3)
    return { error: "Weight is 1, 2 or 3." };
  if (weight !== original.weight) patch.weight = weight;

  const minYears = optionalNumber(formData.get("min_years"));
  const recencyYears = optionalNumber(formData.get("recency_years"));
  const scopeValue = optionalNumber(formData.get("scope_value"));
  const scopeUnit = String(formData.get("scope_unit") ?? "").trim();
  for (const [name, v] of [
    ["Years asked for", minYears],
    ["The recency window", recencyYears],
    ["The scale", scopeValue],
  ] as const) {
    if (v !== undefined && (Number.isNaN(v) || v < 0))
      return { error: `${name} must be a number, or left empty.` };
  }
  if (Boolean(scopeUnit) !== Boolean(scopeValue))
    return { error: "A scale needs both a measure and an amount, or neither." };
  const thresholds: Thresholds = {
    minYears: minYears || undefined,
    recencyYears: recencyYears || undefined,
    minScope: scopeUnit && scopeValue ? { unit: scopeUnit, value: scopeValue } : undefined,
    credentialLevel: String(formData.get("credential_level") ?? "").trim() || undefined,
    credentialField: String(formData.get("credential_field") ?? "").trim() || undefined,
    relationshipKind: String(formData.get("relationship_kind") ?? "").trim() || undefined,
  };
  if (canonical(thresholds) !== canonical(original.thresholds)) patch.thresholds = thresholds;

  const equivalency = String(formData.get("equivalency") ?? "").trim();
  if (equivalency !== original.equivalency) patch.equivalency = equivalency;

  const equivalenceNotes = String(formData.get("equivalence_notes") ?? "").trim();
  if (equivalenceNotes !== original.equivalenceNotes) patch.equivalenceNotes = equivalenceNotes;

  const evidenceStandard = String(formData.get("evidence_standard") ?? "").trim();
  if (evidenceStandard !== original.evidenceStandard) patch.evidenceStandard = evidenceStandard;

  const alternatives = lines(formData.get("alternatives"));
  if (JSON.stringify(alternatives) !== JSON.stringify(original.alternatives))
    patch.alternatives = { values: alternatives };

  const namedParties = lines(formData.get("named_parties"));
  if (JSON.stringify(namedParties) !== JSON.stringify(original.namedParties))
    patch.namedParties = { values: namedParties };

  if (Object.keys(patch).length === 0) return { error: "Nothing was changed." };

  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.HiringService/UpdateRequirement",
    body: { requirementId, patch, editReason },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  revalidatePath("/admin/jd", "layout");
  return { saved: true };
}

export type ReleaseState = {
  done?: boolean;
  released?: number;
  runStarted?: boolean;
  error?: string;
};

export async function releaseRequirementsAction(
  _prev: ReleaseState,
  formData: FormData,
): Promise<ReleaseState> {
  const roleId = String(formData.get("role_id") ?? "").trim();
  if (!roleId) return { error: "The role is missing." };
  const cookie = await getSessionCookie();
  if (!cookie) return { error: "Not signed in." };
  const resp = await callApi({
    path: "/api/career.v1.HiringService/ReleaseRequirements",
    body: { roleId },
    cookie,
  });
  if (!resp.ok) return { error: await errorText(resp) };
  const j = (await resp.json()) as { released?: unknown[]; runStarted?: boolean };
  revalidatePath("/admin/jd", "layout");
  return { done: true, released: j.released?.length ?? 0, runStarted: Boolean(j.runStarted) };
}
