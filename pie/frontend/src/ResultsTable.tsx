import type { query } from "../wailsjs/go/models";
import { projectLabel } from "./linked";
import { Pager, usePaged } from "./Pager";

// ResultsTable lists the issues the query matched, one row each, a page at a time.
export function ResultsTable({ issues }: { issues: query.Issue[] }) {
  const { rows, pager } = usePaged(issues);
  return (
    <>
      <div className="pie-results">
        <table className="pie-table" aria-label="Query results">
          <thead>
            <tr>
              <th scope="col">Key</th>
              <th scope="col">Project</th>
              <th scope="col">Type</th>
              <th scope="col">Summary</th>
              <th scope="col">Status</th>
              <th scope="col">Priority</th>
              <th scope="col">Updated</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((issue) => (
              <tr key={issue.key}>
                <td className="mono">{issue.key}</td>
                <td>{projectLabel(issue.project, issue.projectName)}</td>
                <td>{issue.type}</td>
                <td>{issue.summary}</td>
                <td>{issue.status}</td>
                <td>{issue.priority}</td>
                <td>{issue.updated.slice(0, 10)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
      <Pager label="Results" {...pager} />
    </>
  );
}
