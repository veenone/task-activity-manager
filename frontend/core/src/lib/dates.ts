// Date wording shared by both apps (#539). Every timestamp renders in local
// time, day first, so a field and the status bar never disagree on whether
// 11/20 is a day or a month: XTM's status bar used the machine locale while
// its fields read dd/mm.
//
// Jira Data Center emits timestamps like "2026-06-09T15:18:59.928+0100", a
// numeric offset with no colon, which is not valid ISO 8601. WebView2's Date
// parser rejects it, so parseJiraDate puts the colon in first.

export function parseJiraDate(s: string): Date {
  return new Date(s.replace(/([+-]\d{2})(\d{2})$/, "$1:$2"));
}

const pad = (n: number) => String(n).padStart(2, "0");
const dayOf = (d: Date) => `${pad(d.getDate())}/${pad(d.getMonth() + 1)}/${d.getFullYear()}`;
const timeOf = (d: Date) => `${pad(d.getHours())}:${pad(d.getMinutes())}`;

// formatDateTime is dd/mm/yyyy HH:mm. "(none)" for an empty value, and the
// raw text when it is not a date at all.
export function formatDateTime(s?: string): string {
  if (!s) return "(none)";
  const d = parseJiraDate(s);
  return isNaN(d.getTime()) ? s : `${dayOf(d)} ${timeOf(d)}`;
}

// formatDate is dd/mm/yyyy, with the same empty and unreadable handling.
export function formatDate(s?: string): string {
  if (!s) return "(none)";
  const d = parseJiraDate(s);
  return isNaN(d.getTime()) ? s : dayOf(d);
}

// formatWhen is a sync or update stamp: "today HH:MM" for a same-day stamp,
// formatDateTime's wording otherwise, "" for none, and the raw text when it
// is not a date.
export function formatWhen(iso: string, now: Date = new Date()): string {
  if (!iso) return "";
  const d = parseJiraDate(iso);
  if (isNaN(d.getTime())) return iso;
  const sameDay =
    d.getFullYear() === now.getFullYear() && d.getMonth() === now.getMonth() && d.getDate() === now.getDate();
  return sameDay ? `today ${timeOf(d)}` : `${dayOf(d)} ${timeOf(d)}`;
}
