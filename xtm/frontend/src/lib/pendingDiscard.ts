// The pending-change types that create something new in Jira. Discarding
// one throws away the whole draft, every field the user typed, and nothing
// else holds a copy, so the Pending changes dialog asks first (#535). A
// field edit discards at once: Jira still has the value it reverts to.
const CREATE_TYPES = new Set([
  "test_create",
  "bug_create",
  "precondition_add",
  "test_container_add",
  "folder_create",
]);

export function discardAsksFirst(entityType: string): boolean {
  return CREATE_TYPES.has(entityType);
}
