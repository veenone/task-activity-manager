import type { ValueCoverage } from "../../api";

// How a parameter value's coverage reads in the matrix.
export const STATUS_LABEL: Record<string, string> = {
  PASSED: "Passed",
  FAILED: "Failed",
  NOTRUN: "Not run",
  UNCOVERED: "Uncovered",
};

export function statusClass(vc: ValueCoverage | undefined): string {
  if (!vc || !vc.tested) return "cov-pill cov-gap";
  switch (vc.runStatus) {
    case "PASSED":
      return "cov-pill cov-pass";
    case "FAILED":
      return "cov-pill cov-fail";
    default:
      return "cov-pill cov-notrun";
  }
}

export function statusText(vc: ValueCoverage | undefined): string {
  if (!vc || !vc.tested) return "Gap";
  return STATUS_LABEL[vc.runStatus] ?? vc.runStatus;
}
