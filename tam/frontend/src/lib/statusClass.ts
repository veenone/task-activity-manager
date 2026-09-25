// statusClass buckets a status into the three colours the mockup uses:
// done, in progress, and everything else. The category is Jira's own
// bucket for the status, the same three keys on every instance, so a
// status named in any language lands in the right colour.
//
// Without one it falls back to guessing from the name, which is what a row
// synced before schema 19 has, and what the store leaves behind when a
// board move writes a status nothing here knows the category of. The guess
// matches the backend's IsDone, so the chip, the tree's progress counts and
// the Show done filter never disagree about an English-named issue.
export function statusClass(status: string, category?: string): "done" | "active" | "todo" {
  switch (category) {
    case "done":
      return "done";
    case "indeterminate":
      return "active";
    case "new":
      return "todo";
  }
  const s = status.toLowerCase();
  if (s === "done" || s === "closed" || s === "resolved") return "done";
  if (s.includes("progress") || s === "in review") return "active";
  return "todo";
}
