// The stored components value is newline-bounded ("\nA\nB\n"); see
// internal/fieldcodec. These helpers only read and check names; encoding
// stays in Go.

export function formatComponentsValue(stored: string): string {
  return stored
    .split("\n")
    .map((s) => s.trim())
    .filter(Boolean)
    .join(", ");
}

export function validateComponentName(v: string): string | null {
  if (!v.trim()) return "A component needs a name.";
  if (/[\r\n]/.test(v)) return "A component name cannot contain a line break.";
  if ([...v].length > 255) return "A component name can be at most 255 characters.";
  return null;
}
