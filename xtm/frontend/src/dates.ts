// Date formatting for XTM. The shared wording (parseJiraDate, formatDateTime,
// formatDate, formatWhen) lives in core so TAM and XTM print one date shape,
// day first in local time (#539); this file re-exports it for the existing
// imports.
import { formatDateTime, parseJiraDate } from "@agile-suite/core";

export { formatDate, formatDateTime, formatWhen, parseJiraDate } from "@agile-suite/core";

// formatDateTimeLong is formatDateTime with seconds, for logs and history.
export function formatDateTimeLong(s?: string): string {
  const base = formatDateTime(s);
  const d = s ? parseJiraDate(s) : null;
  return d && !isNaN(d.getTime()) ? `${base}:${String(d.getSeconds()).padStart(2, "0")}` : base;
}
