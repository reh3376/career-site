import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import {
  approveRegistrationAction,
  declineRegistrationAction,
  setMemberStatusAction,
} from "../actions";

import { ActionButton } from "./action-button";

export const metadata: Metadata = { title: "Admin — Member detail" };
export const dynamic = "force-dynamic";

type Me = {
  id: string;
  name: string;
  email: string;
  status: string;
  role: string;
};

type GetMemberResp = {
  member?: {
    me?: Me;
    first_seen_at?: string;
  };
};

const STATUS_LABEL: Record<string, string> = {
  MEMBER_STATUS_UNVERIFIED: "unverified",
  MEMBER_STATUS_PENDING_APPROVAL: "pending",
  MEMBER_STATUS_ACTIVE: "active",
  MEMBER_STATUS_DECLINED: "declined",
  MEMBER_STATUS_EXPIRED: "expired",
  MEMBER_STATUS_DISABLED: "disabled",
};

const STATUS_TONE: Record<string, string> = {
  MEMBER_STATUS_PENDING_APPROVAL: "text-signal",
  MEMBER_STATUS_ACTIVE: "text-success",
  MEMBER_STATUS_UNVERIFIED: "text-ink-3",
  MEMBER_STATUS_DECLINED: "text-ink-3",
  MEMBER_STATUS_EXPIRED: "text-ink-3",
  MEMBER_STATUS_DISABLED: "text-danger",
};

async function fetchMember(id: string): Promise<GetMemberResp | null> {
  const cookie = await getSessionCookie();
  if (!cookie) return null;
  const resp = await callApi({
    path: "/api/career.v1.AdminService/GetMember",
    body: { memberId: id },
    cookie,
  });
  if (!resp.ok) return null;
  return (await resp.json()) as GetMemberResp;
}

export default async function AdminMemberDetailPage({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const data = await fetchMember(id);
  const me = data?.member?.me;
  if (!me) notFound();

  const status = me.status ?? "";
  const label = STATUS_LABEL[status] ?? status;
  const tone = STATUS_TONE[status] ?? "text-ink-2";
  const isAdmin = me.role === "MEMBER_ROLE_ADMIN";

  return (
    <>
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        <Link
          href="/admin/registrations"
          className="text-ink-2 no-underline hover:text-accent"
        >
          ← registrations
        </Link>
      </p>
      <h1
        className="font-display mt-3 text-4xl leading-[1.05] tracking-tight text-ink sm:text-5xl"
        style={{ fontVariationSettings: '"opsz" 120, "SOFT" 40' }}
      >
        {me.name || me.email}
      </h1>

      <dl className="mt-8 grid gap-3 border-y border-line py-4 font-mono text-sm text-ink-2 sm:grid-cols-[10rem_1fr]">
        <dt className="text-ink-3">id</dt>
        <dd className="m-0 text-ink">{me.id}</dd>
        <dt className="text-ink-3">name</dt>
        <dd className="m-0 text-ink">{me.name || "—"}</dd>
        <dt className="text-ink-3">email</dt>
        <dd className="m-0">
          <a
            href={`mailto:${me.email}`}
            className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
          >
            {me.email}
          </a>
        </dd>
        <dt className="text-ink-3">status</dt>
        <dd className={"m-0 " + tone}>
          <span className="pilot mr-2 align-middle" aria-hidden="true" />
          {label}
        </dd>
        <dt className="text-ink-3">role</dt>
        <dd className="m-0 text-ink">
          {isAdmin ? "admin" : "member"}
        </dd>
        <dt className="text-ink-3">first seen</dt>
        <dd className="m-0 text-ink">
          {data?.member?.first_seen_at
            ? new Date(data.member.first_seen_at).toISOString().slice(0, 10)
            : "—"}
        </dd>
      </dl>

      <section
        aria-labelledby="actions-heading"
        className="mt-12 border-t border-line pt-8"
      >
        <p
          id="actions-heading"
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
        >
          actions
        </p>
        <Actions memberId={me.id} status={status} isAdmin={isAdmin} />
      </section>

      <p className="mt-10 text-xs leading-relaxed text-ink-3">
        Activity events, conversations, and admin notes surface here
        when the activity ingest lands (Phase 3+). Status transitions
        write to the <code className="font-mono text-ink">approval_decisions</code>
        {" "}table with <code className="font-mono text-ink">decided_via=console</code>
        {" "}so the audit trail distinguishes them from the email flow.
      </p>
    </>
  );
}

function Actions({
  memberId,
  status,
  isAdmin,
}: {
  memberId: string;
  status: string;
  isAdmin: boolean;
}) {
  // Guard: don't let an admin accidentally disable themselves or
  // another admin from this surface. Admin-role changes need a
  // separate, more deliberate control.
  if (isAdmin) {
    return (
      <p className="mt-4 text-sm text-ink-3">
        This member has the admin role. Status changes on admin users
        aren&rsquo;t available from this surface.
      </p>
    );
  }
  switch (status) {
    case "MEMBER_STATUS_PENDING_APPROVAL":
      return (
        <div className="mt-4 flex flex-wrap gap-3">
          <ActionButton
            action={approveRegistrationAction}
            memberId={memberId}
            tone="accent"
            label="Approve"
          />
          <ActionButton
            action={declineRegistrationAction}
            memberId={memberId}
            tone="signal"
            label="Decline"
          />
        </div>
      );
    case "MEMBER_STATUS_ACTIVE":
      return (
        <div className="mt-4 flex flex-wrap gap-3">
          <ActionButton
            action={setMemberStatusAction}
            memberId={memberId}
            target="MEMBER_STATUS_DISABLED"
            tone="signal"
            label="Disable account"
          />
        </div>
      );
    case "MEMBER_STATUS_DISABLED":
      return (
        <div className="mt-4 flex flex-wrap gap-3">
          <ActionButton
            action={setMemberStatusAction}
            memberId={memberId}
            target="MEMBER_STATUS_ACTIVE"
            tone="accent"
            label="Re-enable"
          />
        </div>
      );
    case "MEMBER_STATUS_EXPIRED":
      return (
        <div className="mt-4">
          <p className="text-sm text-ink-3">
            Access expired. Re-approval / TTL extension lands with the
            access-grants console (next surface).
          </p>
        </div>
      );
    case "MEMBER_STATUS_DECLINED":
      return (
        <div className="mt-4">
          <p className="text-sm text-ink-3">
            Applicant was declined. If they should be given access,
            they can register again from{" "}
            <a
              href="/register"
              className="text-accent underline decoration-accent/40 decoration-1 underline-offset-4 hover:decoration-accent"
            >
              /register
            </a>
            .
          </p>
        </div>
      );
    case "MEMBER_STATUS_UNVERIFIED":
      return (
        <div className="mt-4">
          <p className="text-sm text-ink-3">
            Applicant hasn&rsquo;t verified their email yet. No admin
            action needed — the account moves to pending_approval
            automatically once they click the verification link.
          </p>
        </div>
      );
    default:
      return (
        <p className="mt-4 text-sm text-ink-3">No actions for this status.</p>
      );
  }
}

