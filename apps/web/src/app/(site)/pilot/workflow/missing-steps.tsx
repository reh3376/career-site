"use client";

import { useState } from "react";

import { CommentBox } from "../comment-box";

// The repeatable "a step that is missing" field (FSD FR-PLT-09). Each
// entry is a comment of kind workflow_missing with a sequence number
// for its target; a new empty box appears once the last one is saved,
// so a participant with three missing steps writes three and never
// has to say in advance how many.
export function MissingSteps({ initial }: { initial: { id: string; body: string }[] }) {
  const nextId = () => String(Math.max(0, ...initial.map((m) => Number(m.id) || 0)) + 1);
  const [slots, setSlots] = useState<{ id: string; body: string }[]>(() => [...initial, { id: nextId(), body: "" }]);

  return (
    <ol className="mt-2 space-y-2">
      {slots.map((s, i) => (
        <li key={s.id}>
          <CommentBox
            targetKind="workflow_missing"
            targetId={s.id}
            initialBody={s.body}
            label={`missing step ${s.id}`}
            placeholder="What happens, where in the sequence, and who owns it"
            onSaved={(body) => {
              // Saving the last, empty slot opens another. Removing a
              // saved one leaves the slot; the row is gone server-side.
              if (i === slots.length - 1 && body !== "") {
                setSlots((prev) => [...prev, { id: String(Number(prev[prev.length - 1].id) + 1), body: "" }]);
              }
            }}
          />
        </li>
      ))}
    </ol>
  );
}
