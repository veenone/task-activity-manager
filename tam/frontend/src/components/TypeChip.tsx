import { ISSUE_TYPES } from "../api";

// TypeChip is the coloured type pill the grid, the tree, and the panel share.
// Unknown types (none should reach the UI) show their raw name.
//
// subtaskLabel is the instance's own word for the sub-task level, passed in
// by whoever has the profile rather than looked up here, so the chip stays a
// pure render and can be dropped into a test without a provider. A reader who
// sees "Technical task" in Jira should see it here too: a chip reading "Sub"
// is a word their instance never uses. The column is narrow, so the chip
// clips and carries the full name as its tooltip.
export function TypeChip({ type, subtaskLabel }: { type: string; subtaskLabel?: string }) {
  const t = ISSUE_TYPES.find((x) => x.id === type);
  const instanceName = type === "subtask" ? (subtaskLabel ?? "") : "";
  return (
    <span className={`chip chip-type chip-type-${type || "none"}`} title={instanceName || t?.label || type}>
      {instanceName || t?.short || type}
    </span>
  );
}
