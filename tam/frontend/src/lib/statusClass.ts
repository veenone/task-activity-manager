// statusClass buckets a Jira status name into the three colours the
// mockup uses: done, in progress, and everything else. The done bucket
// matches the backend's IsDone so the chip, the tree's progress counts,
// and the Show done filter never disagree about one issue.
export function statusClass(status: string): "done" | "active" | "todo" {
  const s = status.toLowerCase();
  if (s === "done" || s === "closed" || s === "resolved") return "done";
  if (s.includes("progress") || s === "in review") return "active";
  return "todo";
}
