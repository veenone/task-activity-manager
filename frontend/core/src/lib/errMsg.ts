// errMsg turns whatever a rejected promise carries into a readable string.
export function errMsg(e: unknown): string {
  if (e instanceof Error) return e.message;
  return typeof e === "string" ? e : String(e);
}
