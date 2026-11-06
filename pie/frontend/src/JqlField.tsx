// JqlField folds the JQL away: most runs reuse a saved query or a Jira filter,
// so the box is shut and the query shows on one line. Empty JQL keeps it open,
// since there is nothing to show and something to type.
export function JqlField({
  jql,
  onChange,
  open,
  onOpenChange,
}: {
  jql: string;
  onChange: (jql: string) => void;
  open: boolean;
  onOpenChange: (open: boolean) => void;
}) {
  return (
    <details
      className="pie-jql"
      aria-label="JQL query"
      open={open || jql.trim() === ""}
      onToggle={(e) => onOpenChange(e.currentTarget.open)}
    >
      <summary>
        <span className="field-label">JQL</span>
        <span className="pie-jql-line mono">{jql}</span>
      </summary>
      <label className="sr-only" htmlFor="pie-jql">
        JQL
      </label>
      <textarea
        id="pie-jql"
        className="detail-input mono"
        rows={4}
        spellCheck={false}
        value={jql}
        onChange={(e) => onChange(e.target.value)}
      />
    </details>
  );
}
