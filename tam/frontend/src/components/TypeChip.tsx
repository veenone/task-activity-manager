import { ISSUE_TYPES } from "../api";

// TypeChip is the coloured type pill the grid, the tree, and the panel share.
// A type TAM has no palette for shows its raw name on the neutral chip. That
// is the ordinary case since #68: the sync fetches every type the project
// has, so a type TAM does not model reaches every one of these views.
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
    // The palette class only for a type that has one. A row, or a draft, can
    // carry the project's own name for a type TAM has no concept of (#65
    // item 2, #68), and chip-type-Improvement is a class no stylesheet
    // defines, which renders as unstyled text and reports nothing.
    <span className={`chip chip-type chip-type-${t ? type : "none"}`} title={instanceName || t?.label || type}>
      {instanceName || t?.short || type}
    </span>
  );
}
