import type { CoverageState } from "./useCoverageState";

// ReuseTable lists the member requirements that reuse this one.
export function ReuseTable({ s }: { s: CoverageState }) {
  const { reuse } = s;
  return (
    <>
      <table aria-label="Reusable tests" className="cov-table cov-reuse">
        <thead>
          <tr>
            <th scope="col">Project</th>
            <th scope="col">Requirement</th>
            <th scope="col">Summary</th>
            <th scope="col">Status</th>
          </tr>
        </thead>
        <tbody>
          {reuse.length === 0 && (
            <tr>
              <td colSpan={4} className="cov-empty">No member requirements. Use "Members" to link the customer requirements that reuse this.</td>
            </tr>
          )}
          {reuse.map((r) => (
            <tr key={r.requirementKey}>
              <td>{r.projectKey || "—"}</td>
              <td>{r.requirementKey}</td>
              <td>{r.summary}</td>
              <td>{r.status}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
