export const KINDS: Array<{ value: string; label: string }> = [
  { value: "testset", label: "Test Set" },
  { value: "testplan", label: "Test Plan" },
  { value: "testexec", label: "Test Execution" },
];

// Standard Xray Test Run result vocabulary (mirrors testrepo.RunStatuses).
export const RUN_STATUSES = ["TODO", "EXECUTING", "PASS", "FAIL", "ABORTED", "BLOCKED"];

// Page sizes the member table offers. An execution may hold hundreds of tests.
export const PAGE_SIZE_OPTIONS = [10, 15, 25, 50, 100, 200];
