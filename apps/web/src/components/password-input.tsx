"use client";

import { useState } from "react";

// A password field with a show/hide toggle, so the person can read what
// they typed before saving it (owner, live review 2026-11-09). The
// toggle is a button, not a link, never submits, and says which state
// it is in for a screen reader. Everything else about the input is the
// caller's: name, autocomplete, minimum length, the form's own class.
export function PasswordInput({
  className,
  ...props
}: Omit<React.InputHTMLAttributes<HTMLInputElement>, "type"> & { className: string }) {
  const [show, setShow] = useState(false);
  return (
    <div className="relative">
      <input {...props} type={show ? "text" : "password"} className={`${className} pr-16`} />
      <button
        type="button"
        onClick={() => setShow((s) => !s)}
        aria-pressed={show}
        aria-label={show ? "Hide password" : "Show password"}
        className="absolute right-0 top-1/2 -translate-y-1/2 px-2 py-1 font-mono text-[11px] uppercase tracking-[0.14em] text-ink-3 transition-colors hover:text-accent"
      >
        {show ? "hide" : "show"}
      </button>
    </div>
  );
}
