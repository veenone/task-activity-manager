export const KINDS: Array<{ value: string; label: string }> = [
  { value: "testset", label: "Test Set" },
  { value: "testplan", label: "Test Plan" },
  { value: "testexec", label: "Test Execution" },
];

// offeredKinds narrows the type picker to the kinds the backend has, keeping
// the order above.
//
// Only Xray has all three. Kiwi and Zephyr Essential have executions alone, so
// offering Test Set and Test Plan there leads to a view that can never hold
// anything, which reads as a failed load rather than as a kind this backend
// does not have (#371 Phase 2).
//
// A backend reporting none is offered all three. That is the state while a
// profile's capabilities are still loading, and it matches the permissive
// default every other consumer uses, so nothing disappears and comes back.
export function offeredKinds(containerKinds: string[]): Array<{ value: string; label: string }> {
  const offered = KINDS.filter((k) => containerKinds.includes(k.value));
  return offered.length > 0 ? offered : KINDS;
}

// Standard Xray Test Run result vocabulary (mirrors testrepo.RunStatuses).
export const RUN_STATUSES = ["TODO", "EXECUTING", "PASS", "FAIL", "ABORTED", "BLOCKED"];

// Page sizes the member table offers. An execution may hold hundreds of tests.
export const PAGE_SIZE_OPTIONS = [10, 15, 25, 50, 100, 200];
