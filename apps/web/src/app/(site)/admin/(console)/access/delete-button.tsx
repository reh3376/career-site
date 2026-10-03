"use client";

import { useFormStatus } from "react-dom";

import { deleteAccessGrantAction } from "./actions";

// Delete button on each row. Wrapped in a form so we can use the
// server action + pending state; the confirm is a native prompt on
// submit so an accidental click doesn't nuke the row silently.
export function DeleteGrantButton({
  id,
  email,
}: {
  id: string;
  email: string;
}) {
  return (
    <form
      action={deleteAccessGrantAction}
      onSubmit={(e) => {
        if (
          !window.confirm(
            `Remove ${email} from the whitelist? Existing accounts already granted access are unaffected.`,
          )
        ) {
          e.preventDefault();
        }
      }}
      className="m-0"
    >
      <input type="hidden" name="id" value={id} />
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
      className="inline-flex items-center border border-signal px-2 py-1 font-mono text-[10px] uppercase tracking-[0.14em] text-signal transition-colors hover:bg-signal hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
    >
      {pending ? "Removing…" : "Remove"}
    </button>
  );
}
