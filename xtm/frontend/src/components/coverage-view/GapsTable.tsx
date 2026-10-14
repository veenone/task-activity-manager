import type { CoverageState } from "./useCoverageState";

// GapsTable lists the required values no test covers.
export function GapsTable({ s }: { s: CoverageState }) {
  const { gaps } = s;
  return (
    <>
      <table aria-label="Coverage gaps" className="cov-table cov-gaps">
        <thead>
          <tr>
            <th scope="col">Group</th>
            <th scope="col">Parameter</th>
            <th scope="col">Value</th>
            <th scope="col">Kind</th>
          </tr>
        </thead>
        <tbody>
          {gaps.length === 0 && (
            <tr>
              <td colSpan={4} className="cov-empty">No gaps. Every required value has a test. 🎉</td>
            </tr>
          )}
          {gaps.map((g) => (
            <tr key={g.valueId}>
              <td>{g.groupName}</td>
              <td>{g.paramName}</td>
              <td>{g.valueLabel}</td>
              <td>{g.errorCode || g.valueKind}</td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
