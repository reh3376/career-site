// The optional "+pilot" convenience (FSD FR-PLT-18, owner 2026-11-09):
// someone who already has an ordinary account can hold a separate
// pilot identity as name+pilot@domain, and tick a box instead of typing
// the suffix. Optional on purpose: plus-addressing is a feature of
// some mail providers (Gmail, Fastmail), not of mail, and a company
// server that drops it would leave a participant unable to verify. The
// pilot flag, not the address, is what makes a participant.
export function withPlusPilot(email: string): string {
  const at = email.indexOf("@");
  if (at <= 0) return email;
  const local = email.slice(0, at);
  if (local.includes("+")) return email;
  return `${local}+pilot${email.slice(at)}`;
}
