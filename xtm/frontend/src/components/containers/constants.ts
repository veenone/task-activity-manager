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

// Xray's Test Run result vocabulary, and the fallback for a backend that
// declares none. Mirrors backend.DefaultRunStatuses in internal/backend.
export const RUN_STATUSES = ["TODO", "EXECUTING", "PASS", "FAIL", "ABORTED", "BLOCKED"];

// offeredRunStatuses is the result vocabulary to offer for this profile.
//
// The vocabulary is not shared: Zephyr configures its statuses per instance
// and reads them at connect, and Kiwi has its own names. Offering Xray's six
// everywhere let a user pick a result their backend has no id for, which only
// failed at commit (#371 Phase 5).
//
// A backend that declares none is offered Xray's, which is also what shows
// while a profile's capabilities are still loading, so nothing disappears and
// comes back. Kiwi still declares none: reading a Kiwi instance's configured
// TestExecutionStatus list is its own piece of work.
export function offeredRunStatuses(runStatuses: string[]): string[] {
  return runStatuses.length > 0 ? runStatuses : RUN_STATUSES;
}

// Page sizes the member table offers. An execution may hold hundreds of tests.
export const PAGE_SIZE_OPTIONS = [10, 15, 25, 50, 100, 200];
