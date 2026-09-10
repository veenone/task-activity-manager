import type { SprintOption } from "../api";

// duplicateNameIds is which sprint ids share a name with another sprint in
// the same list, compared case-insensitively. Only those get their board
// name beside them, both of them and not just the second; a sprint whose
// name is unique is shown plain.
//
// A shared name here means two genuinely different sprints: OpenSprints
// folds a sprint id to one row no matter how many boards it reaches, so two
// rows never share an id, and a repeated name is two boards' sprints that
// happen to be called the same thing.
export function duplicateNameIds(sprints: SprintOption[]): Set<number> {
  const byName = new Map<string, number[]>();
  for (const s of sprints) {
    const key = s.name.toLowerCase();
    const ids = byName.get(key);
    if (ids) {
      ids.push(s.id);
    } else {
      byName.set(key, [s.id]);
    }
  }
  const dups = new Set<number>();
  for (const ids of byName.values()) {
    if (ids.length > 1) ids.forEach((id) => dups.add(id));
  }
  return dups;
}

// sprintOptionLabel is the text an option shows: the sprint's own name,
// with its board name beside it only when another sprint in the same list
// answers to the same name.
export function sprintOptionLabel(s: SprintOption, dupIds: Set<number>): string {
  return dupIds.has(s.id) && s.boardName ? `${s.name} (${s.boardName})` : s.name;
}
