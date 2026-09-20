import type { Metadata } from "next";
import Link from "next/link";
import { notFound } from "next/navigation";

import { callApi } from "@/lib/api-fetch";
import { getSessionCookie } from "@/lib/session";

import {
  approveRegistrationAction,
  declineRegistrationAction,
  extendAccessAction,
  setMemberStatusAction,
} from "../actions";

import { ActionButton } from "./action-button";

export const metadata: Metadata = { title: "Admin — Member detail" };
export const dynamic = "force-dynamic";

type Me = {
  id: string;
  name: string;
  email: string;
  organization?: string;
  stated_role?: string;
  status: string;
  role: string;
  created_at?: string;
  expires_at?: string;
  last_notification_kind?: string;
  last_notification_at?: string;
  last_notification_error?: string;
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
  const created = me.created_at ? new Date(me.created_at) : null;
  const expires = me.expires_at ? new Date(me.expires_at) : null;

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

      <section
        aria-labelledby="account-heading"
        className="mt-10 border-t border-line pt-6"
      >
        <p
          id="account-heading"
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
        >
          account
        </p>
        <dl className="mt-4 grid gap-3 font-mono text-sm text-ink-2 sm:grid-cols-[11rem_1fr]">
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
          <dt className="text-ink-3">organization</dt>
          <dd className="m-0 text-ink">{me.organization || "—"}</dd>
          <dt className="text-ink-3">stated role</dt>
          <dd className="m-0 text-ink">{me.stated_role || "—"}</dd>
          <dt className="text-ink-3">status</dt>
          <dd className={"m-0 " + tone}>
            <span className="pilot mr-2 align-middle" aria-hidden="true" />
            {label}
          </dd>
          <dt className="text-ink-3">role</dt>
          <dd className="m-0 text-ink">{isAdmin ? "admin" : "member"}</dd>
        </dl>
      </section>

      <section
        aria-labelledby="access-heading"
        className="mt-10 border-t border-line pt-6"
      >
        <p
          id="access-heading"
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
        >
          access window
        </p>
        <dl className="mt-4 grid gap-3 font-mono text-sm text-ink-2 sm:grid-cols-[11rem_1fr]">
          <dt className="text-ink-3">registered</dt>
          <dd className="m-0 text-ink">
            {created ? created.toISOString().slice(0, 10) : "—"}
            {created ? (
              <span className="ml-3 text-ink-3">({relative(created)})</span>
            ) : null}
          </dd>
          <dt className="text-ink-3">expires</dt>
          <dd className="m-0 text-ink">
            {expires ? (
              <>
                {expires.toISOString().slice(0, 10)}
                {" · "}
                <span
                  className={
                    daysUntil(expires) <= 7
                      ? "text-signal"
                      : daysUntil(expires) < 0
                        ? "text-danger"
                        : "text-ink-2"
                  }
                >
                  {formatDaysUntil(expires)}
                </span>
              </>
            ) : status === "MEMBER_STATUS_ACTIVE" ? (
              <span className="text-success">permanent</span>
            ) : (
              "—"
            )}
          </dd>
        </dl>
      </section>

      <section
        aria-labelledby="notif-heading"
        className="mt-10 border-t border-line pt-6"
      >
        <p
          id="notif-heading"
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
        >
          last notification email
        </p>
        <NotificationPill me={me} />
      </section>

      <section
        aria-labelledby="activity-heading"
        className="mt-10 border-t border-line pt-6"
      >
        <p
          id="activity-heading"
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
        >
          activity
        </p>
        <p className="mt-4 text-sm leading-relaxed text-ink-3">
          Activity events, Ask Roger conversation history, and saved
          items surface here when the activity ingest lands (Phase 3+).
          The <code className="font-mono text-ink">MemberCounts</code>{" "}
          field on the RPC is stubbed at zero for now.
        </p>
      </section>

      <section
        aria-labelledby="actions-heading"
        className="mt-10 border-t border-line pt-6"
      >
        <p
          id="actions-heading"
          className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3"
        >
          actions
        </p>
        <Actions me={me} isAdmin={isAdmin} />
        <ContactAction email={me.email} name={me.name} />
      </section>
    </>
  );
}

function Actions({ me, isAdmin }: { me: Me; isAdmin: boolean }) {
  const status = me.status;
  const memberId = me.id;

  if (isAdmin) {
    return (
      <p className="mt-4 text-sm text-ink-3">
        This member has the admin role. Status transitions on admin
        users aren&rsquo;t available from this surface.
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
        <div className="mt-4 space-y-4">
          <div>
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              extend access
            </p>
            <div className="mt-2 flex flex-wrap gap-2">
              <ActionButton
                action={extendAccessAction}
                memberId={memberId}
                extra={{ mode: "days", days: "30" }}
                tone="accent"
                label="+30 days"
              />
              <ActionButton
                action={extendAccessAction}
                memberId={memberId}
                extra={{ mode: "days", days: "90" }}
                tone="accent"
                label="+90 days"
              />
              <ActionButton
                action={extendAccessAction}
                memberId={memberId}
                extra={{ mode: "days", days: "365" }}
                tone="accent"
                label="+1 year"
              />
              <ActionButton
                action={extendAccessAction}
                memberId={memberId}
                extra={{ mode: "permanent" }}
                tone="accent"
                label="Make permanent"
              />
            </div>
          </div>
          <div>
            <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
              suspend / disable
            </p>
            <div className="mt-2">
              <ActionButton
                action={setMemberStatusAction}
                memberId={memberId}
                extra={{ status: "MEMBER_STATUS_DISABLED" }}
                tone="signal"
                label="Disable account"
              />
            </div>
          </div>
        </div>
      );
    case "MEMBER_STATUS_DISABLED":
      return (
        <div className="mt-4 flex flex-wrap gap-3">
          <ActionButton
            action={setMemberStatusAction}
            memberId={memberId}
            extra={{ status: "MEMBER_STATUS_ACTIVE" }}
            tone="accent"
            label="Re-enable"
          />
        </div>
      );
    case "MEMBER_STATUS_EXPIRED":
      return (
        <div className="mt-4 space-y-4">
          <p className="text-sm text-ink-3">
            Access expired. Extend below to reactivate — a new
            expires_at in the future flips the account back to active
            automatically.
          </p>
          <div className="flex flex-wrap gap-2">
            <ActionButton
              action={extendAccessAction}
              memberId={memberId}
              extra={{ mode: "days", days: "30" }}
              tone="accent"
              label="+30 days"
            />
            <ActionButton
              action={extendAccessAction}
              memberId={memberId}
              extra={{ mode: "days", days: "90" }}
              tone="accent"
              label="+90 days"
            />
            <ActionButton
              action={extendAccessAction}
              memberId={memberId}
              extra={{ mode: "days", days: "365" }}
              tone="accent"
              label="+1 year"
            />
            <ActionButton
              action={extendAccessAction}
              memberId={memberId}
              extra={{ mode: "permanent" }}
              tone="accent"
              label="Make permanent"
            />
          </div>
        </div>
      );
    case "MEMBER_STATUS_DECLINED":
      return (
        <div className="mt-4">
          <p className="text-sm text-ink-3">
            Applicant was declined. If they should be given access,
            ask them to register again at{" "}
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
            action needed — the account moves to{" "}
            <span className="font-mono text-ink">pending_approval</span>{" "}
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

function ContactAction({ email, name }: { email: string; name?: string }) {
  const subject = `Re: your career-site access`;
  const body = `Hi ${name?.split(" ")[0] || "there"},\n\n`;
  const href = `mailto:${email}?subject=${encodeURIComponent(subject)}&body=${encodeURIComponent(body)}`;
  return (
    <div className="mt-6 border-t border-line pt-6">
      <p className="font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3">
        contact
      </p>
      <div className="mt-2 flex flex-wrap gap-3">
        <a
          href={href}
          className="inline-flex items-center border border-line-strong px-4 py-2 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-2 no-underline transition-colors hover:border-accent hover:text-accent"
        >
          Email {name?.split(" ")[0] || "member"}
        </a>
      </div>
    </div>
  );
}

// Human labels for the notification kinds the API stamps.
const NOTIF_LABEL: Record<string, string> = {
  user_approved: "approval",
  user_declined: "decline",
  user_auto_declined: "auto-decline",
  expiry_warn: "expiry warning",
  expired: "expiry notice",
};

// Small pill showing whether the last outbound email actually
// landed with the provider. Green (success), red (failure), or
// muted (never sent).
function NotificationPill({ me }: { me: Me }) {
  const kind = me.last_notification_kind ?? "";
  const at = me.last_notification_at ? new Date(me.last_notification_at) : null;
  const failed = Boolean(me.last_notification_error);

  if (!kind || !at) {
    return (
      <p className="mt-3 text-sm text-ink-3">
        No notification email has been sent to this member yet. Approve
        / decline / extend actions dispatch one automatically.
      </p>
    );
  }
  const label = NOTIF_LABEL[kind] ?? kind;
  const when = relative(at);
  if (failed) {
    return (
      <div className="mt-3 border-l-2 border-signal bg-signal-soft/50 px-4 py-3 text-sm text-ink">
        <p>
          <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-signal">
            send failed
          </span>{" "}
          <span className="text-ink-2">
            — {label} email, {when}
          </span>
        </p>
        <p className="mt-2 whitespace-pre-wrap font-mono text-[11px] text-ink-3">
          {me.last_notification_error}
        </p>
      </div>
    );
  }
  return (
    <p className="mt-3 text-sm text-ink-2">
      <span className="font-mono text-[11px] uppercase tracking-[0.14em] text-success">
        sent
      </span>{" "}
      — {label} email, {when}. If the member reports not receiving it,
      check their spam folder and the provider dashboard.
    </p>
  );
}

// ---------------------------------------------------------------
// Date helpers
// ---------------------------------------------------------------

function relative(d: Date): string {
  const days = Math.max(0, Math.round((Date.now() - d.getTime()) / 86_400_000));
  if (days < 1) return "today";
  if (days === 1) return "1d ago";
  if (days < 7) return `${days}d ago`;
  if (days < 30) return `${Math.round(days / 7)}w ago`;
  if (days < 365) return `${Math.round(days / 30)}mo ago`;
  return `${Math.round(days / 365)}y ago`;
}

function daysUntil(d: Date): number {
  return Math.round((d.getTime() - Date.now()) / 86_400_000);
}

function formatDaysUntil(d: Date): string {
  const n = daysUntil(d);
  if (n === 0) return "expires today";
  if (n < 0) return `expired ${-n}d ago`;
  if (n === 1) return "1 day left";
  if (n < 30) return `${n} days left`;
  if (n < 365) return `${Math.round(n / 30)}mo left`;
  return `${Math.round(n / 365)}y left`;
}
