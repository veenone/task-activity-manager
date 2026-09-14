interface Props {
  issueKey: string;
  count: number;
  expanded: boolean;
  onToggle: () => void;
}

export function SubtaskToggle({ issueKey, count, expanded, onToggle }: Props) {
  if (count === 0) return null;
  return <button type="button" className="subtask-toggle" tabIndex={-1}
    aria-label={`${expanded ? "Collapse" : "Expand"} subtasks of ${issueKey}`}
    aria-expanded={expanded}
    title={`${count} ${count === 1 ? "subtask" : "subtasks"}`}
    onClick={(e) => { e.stopPropagation(); onToggle(); }}
    onKeyDown={(e) => { if (e.key === "Enter" || e.key === " ") e.stopPropagation(); }}>
    <span aria-hidden="true">{expanded ? "▾" : "▸"} {count}</span>
  </button>;
}
