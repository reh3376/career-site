"use client";

import { useFormStatus } from "react-dom";

import { deleteMemberAction } from "../actions";
import { useReason } from "./action-button";

// Deletes the account outright (FSD FR-ADM-25). A native confirm on
// submit, as the whitelist's remove and the pilot's delete do, because
// there is no undo: the row and everything that cascades from it go.
export function DeleteMemberButton({ memberId, email }: { memberId: string; email: string }) {
  const reason = useReason();
  return (
    <form
      action={deleteMemberAction}
      onSubmit={(e) => {
        if (
          !window.confirm(
            `Delete the account ${email}? Its sessions, tokens and pilot sitting go with it, and history keeps a blank where it was. This cannot be undone.`,
          )
        ) {
          e.preventDefault();
        }
      }}
      className="m-0"
    >
      <input type="hidden" name="member_id" value={memberId} />
      <input type="hidden" name="reason" value={reason} />
      <Submit />
    </form>
  );
}

function Submit() {
  const { pending } = useFormStatus();
  return (
    <button
      type="submit"
      disabled={pending}
      className="inline-flex items-center border border-danger px-3 py-1.5 font-mono text-[11px] uppercase tracking-[0.14em] text-danger transition-colors hover:bg-danger hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Deleting…" : "Delete account"}
    </button>
  );
}
