import { useEffect, useState } from "react";
import { CaseGrid as ReadGrid } from "../wailsjs/go/main/App";
import type { cases, query } from "../wailsjs/go/models";
import { errorText } from "./ProfileForm";
import { projectLabel, projectNames } from "./linked";
import { Pager, usePaged } from "./Pager";

// CaseGrid puts every case against every project on any case, with the
// analyst's verdict in each cell. It is read fresh from Go each time it
// opens, so it says what each case's own page says.
export function CaseGrid({
  profileId,
  result,
  onOpen,
}: {
  profileId: string;
  result: query.Result | null;
  onOpen: (id: number) => void;
}) {
  const [rows, setRows] = useState<cases.Detail[] | null>(null);
  const [error, setError] = useState("");
  const names = result ? projectNames(result) : new Map<string, string>();

  useEffect(() => {
    let current = true;
    ReadGrid(profileId).then(
      (g) => current && setRows(g),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId]);

  if (error) {
    return (
      <p role="alert" className="error-text">
        {error}
      </p>
    );
  }
  if (rows === null) return <p className="muted">Reading the cases...</p>;
  return <VerdictGrid rows={rows} names={names} onOpen={onOpen} />;
}

// VerdictGrid draws cases against projects. Without onOpen a case key is
// plain text, as in a cycle, whose cases may since have changed or gone.
export function VerdictGrid({ rows, names, onOpen }: { rows: cases.Detail[]; names: Map<string, string>; onOpen?: (id: number) => void }) {
  const projects = [...new Set(rows.flatMap((c) => c.projects.map((p) => p.key)))].sort();
  const verdict = (c: cases.Detail, key: string) => c.projects.find((p) => p.key === key)?.verdict ?? "-";
  const paged = usePaged(rows);

  if (projects.length === 0) return <p className="muted">No case has projects to assess yet.</p>;
  return (
    <>
      <div className="pie-results">
        <table className="pie-table pie-map" aria-label="Cases by project">
          <thead>
            <tr>
              <th scope="col">Case</th>
              {projects.map((p) => (
                <th key={p} scope="col">
                  {projectLabel(p, names.get(p))}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {paged.rows.map((c) => (
              <tr key={c.id}>
                <th scope="row">
                  {onOpen ? (
                    <button type="button" className="pie-map-key mono" onClick={() => onOpen(c.id)}>
                      {c.key}
                    </button>
                  ) : (
                    <span className="mono">{c.key}</span>
                  )}
                </th>
                {projects.map((p) => (
                  <td key={p} className={verdict(c, p) === "affected" ? "pie-out-of-step" : undefined}>
                    {verdict(c, p)}
                  </td>
                ))}
              </tr>
            ))}
          </tbody>
          <tfoot>
            <tr>
              <th scope="row">Affected</th>
              {projects.map((p) => (
                <td key={p}>{rows.filter((c) => verdict(c, p) === "affected").length}</td>
              ))}
            </tr>
          </tfoot>
        </table>
      </div>
      <Pager label="Cases by project" {...paged.pager} />
    </>
  );
}
