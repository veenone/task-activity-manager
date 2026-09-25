import { ISSUE_TYPES } from "../api";

// What the type chip says and what colour it wears, in one place, because
// the chip is no longer the only thing that needs to know: the column that
// holds it is now measured from the same labels (lib/typeColumn.ts), and a
// measurer reading a different string from the renderer sizes the wrong
// column.

// TYPE_CHIP_ALT_CLASSES are the palette entries a type TAM does not model
// draws from. Before #79 every one of them fell to chip-type-none, which
// since #68 is most of a project's vocabulary: the sync fetches every type
// the project has, so Improvement, Change Request and the instance's own
// words all shared one grey chip and the column stopped carrying meaning.
//
// Three colours is not one per name and cannot be. It is enough for the
// handful of extra types a project actually uses to be told apart, and the
// chip carries its name, so the colour is a help rather than the message.
export const TYPE_CHIP_ALT_CLASSES = ["alt-teal", "alt-pink", "alt-orange"] as const;

// typeChipLabel is the text on the chip: the instance's own word for the
// sub-task level when it is known, TAM's short label for a type it models,
// and otherwise the raw name, which is what a type TAM has no concept of
// arrives as. A reader who sees "Technical task" in Jira should see it here.
export function typeChipLabel(type: string, subtaskLabel?: string): string {
  if (type === "subtask" && subtaskLabel) return subtaskLabel;
  return ISSUE_TYPES.find((t) => t.id === type)?.short ?? type;
}

// typeChipClass is the palette suffix, so `chip-type-${typeChipClass(t)}`.
// A row with no type at all keeps the neutral chip: there is nothing to tell
// apart, and colouring "unknown" would say the opposite.
export function typeChipClass(type: string): string {
  if (ISSUE_TYPES.some((t) => t.id === type)) return type;
  if (type.trim() === "") return "none";
  return TYPE_CHIP_ALT_CLASSES[hash(type) % TYPE_CHIP_ALT_CLASSES.length];
}

// djb2, for a stable colour per name. Not a security hash and not trying to
// be: it only has to give the same answer on every render and spread a few
// dozen names over three buckets. >>> 0 keeps it unsigned, since a negative
// remainder would index off the front of the palette.
function hash(s: string): number {
  let h = 5381;
  for (let i = 0; i < s.length; i++) h = (h * 33) ^ s.charCodeAt(i);
  return h >>> 0;
}
