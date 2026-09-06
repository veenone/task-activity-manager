// formatWhen renders a sync timestamp for the status bar: "today HH:MM" for
// a same-day stamp, the local date and time otherwise, "" for none, and the
// raw text when it is not a date at all.
export function formatWhen(iso: string, now: Date = new Date()): string {
  if (!iso) return "";
  const d = new Date(iso);
  if (Number.isNaN(d.getTime())) return iso;
  const time = d.toLocaleTimeString([], { hour: "2-digit", minute: "2-digit" });
  const sameDay =
    d.getFullYear() === now.getFullYear() &&
    d.getMonth() === now.getMonth() &&
    d.getDate() === now.getDate();
  return sameDay ? `today ${time}` : `${d.toLocaleDateString()} ${time}`;
}

// plural picks the singular or plural word for a count and prefixes it with
// the count itself, e.g. plural(1, "row", "rows") -> "1 row".
export function plural(n: number, one: string, many: string): string {
  return `${n} ${n === 1 ? one : many}`;
}
