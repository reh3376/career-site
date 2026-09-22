// The JD match gate, read at request time from the same variable the
// api uses (JD_MATCH_THRESHOLD), so the copy on the JD pages always
// quotes the gate that is actually enforced. The value is model
// dependent; docs/llm-tuning-log.md records it per judge model.
export function getJdThreshold(): string {
  const raw = process.env.JD_MATCH_THRESHOLD?.trim();
  const n = raw ? Number(raw) : NaN;
  if (!Number.isFinite(n) || n <= 0 || n > 1) return "0.55";
  return n.toFixed(2);
}
