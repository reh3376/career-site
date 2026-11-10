"use client";

import { useState } from "react";

import { CommentBox } from "../comment-box";

// The repeatable "a step that is missing" field, one set per stage
// (FSD FR-PLT-09). Each entry is a comment of kind workflow_missing
// whose target is the stage and a sequence number (A-1, A-2); a new
// empty box appears once the last one is saved, so a participant with
// three missing steps writes three and never has to say in advance
// how many.
export function MissingSteps({ phase, initial }: { phase: string; initial: { id: string; body: string }[] }) {
  const num = (id: string) => Number(id.split("-")[1]) || 0;
  const nextId = (last: number) => `${phase}-${last + 1}`;
  const [slots, setSlots] = useState<{ id: string; body: string }[]>(() => [
    ...initial,
    { id: nextId(Math.max(0, ...initial.map((m) => num(m.id)))), body: "" },
  ]);

  return (
    <ol className="mt-2 space-y-2">
      {slots.map((s, i) => (
        <li key={s.id}>
          <CommentBox
            targetKind="workflow_missing"
            targetId={s.id}
            initialBody={s.body}
            label={`missing step ${num(s.id)}`}
            placeholder="What happens, where in the sequence, and who does it"
            onSaved={(body) => {
              // Saving the last, empty slot opens another. Removing a
              // saved one leaves the slot; the row is gone server-side.
              if (i === slots.length - 1 && body !== "") {
                setSlots((prev) => [...prev, { id: nextId(num(prev[prev.length - 1].id)), body: "" }]);
              }
            }}
          />
        </li>
      ))}
    </ol>
  );
}
