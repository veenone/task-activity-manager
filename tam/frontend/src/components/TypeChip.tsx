import { ISSUE_TYPES } from "../api";
import { typeChipClass, typeChipLabel } from "../lib/typeChip";

// TypeChip is the coloured type pill the grid, the tree, and the panel share.
//
// subtaskLabel is the instance's own word for the sub-task level, passed in
// by whoever has the profile rather than looked up here, so the chip stays a
// pure render and can be dropped into a test without a provider. A reader who
// sees "Technical task" in Jira should see it here too: a chip reading "Sub"
// is a word their instance never uses.
//
// What it says and what colour it wears both come from lib/typeChip, because
// the column that holds the chip is measured from the same labels and a
// measurer reading a different string would size the wrong column.
export function TypeChip({ type, subtaskLabel }: { type: string; subtaskLabel?: string }) {
  const t = ISSUE_TYPES.find((x) => x.id === type);
  const label = typeChipLabel(type, subtaskLabel);
  return (
    // The title is the full name for the case the track is still too narrow.
    // Since #79 the tracks are measured from these labels, so it is a
    // fallback rather than the only way to read the column.
    <span className={`chip chip-type chip-type-${typeChipClass(type)}`} title={type === "subtask" ? label : (t?.label ?? label)}>
      {label}
    </span>
  );
}
