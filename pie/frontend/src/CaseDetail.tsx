import { useEffect, useState } from "react";
import { useConfirm } from "@agile-suite/core";
import { GetCase, RemoveCaseProject, SetCaseNotes, SetVerdict } from "../wailsjs/go/main/App";
import type { cases, query } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";
import { projectLabel, projectNames } from "./linked";
import { ProjectPicker } from "./ProjectPicker";

// The verdicts Go accepts, in the order an assessment usually moves through.
const VERDICTS = ["not assessed", "investigating", "affected", "not affected"];

// when gives a stored time in the analyst's own time zone, as runAtText does.
function when(iso: string): string {
  if (!iso) return "-";
  const d = new Date(iso);
  const two = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${two(d.getMonth() + 1)}-${two(d.getDate())} ${two(d.getHours())}:${two(d.getMinutes())}`;
}

// CaseDetail is one case: its notes and the projects to assess, each with the
// analyst's verdict and reason. Every change is stored at once; nothing here
// asks Jira except the project picker's list.
export function CaseDetail({
  profileId,
  id,
  issue,
  result,
  onChanged,
}: {
  profileId: string;
  id: number;
  issue?: query.Issue;
  result: query.Result | null;
  onChanged: () => void;
}) {
  const { confirm } = useConfirm();
  const [detail, setDetail] = useState<cases.Detail | null>(null);
  const [notes, setNotes] = useState("");
  const [reasons, setReasons] = useState<Record<string, string>>({});
  const [picking, setPicking] = useState(false);
  const [error, setError] = useState("");
  const names = result ? projectNames(result) : new Map<string, string>();

  function show(d: cases.Detail) {
    setDetail(d);
    setReasons(Object.fromEntries(d.projects.map((p) => [p.key, p.reason])));
  }

  useEffect(() => {
    let current = true;
    GetCase(profileId, id).then(
      (d) => {
        if (!current) return;
        show(d);
        setNotes(d.notes);
      },
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId, id]);

  // change stores one edit, shows the case Go returns, and tells the list.
  async function change(edit: () => Promise<cases.Detail | void>) {
    setError("");
    try {
      const d = await edit();
      if (d) show(d);
      onChanged();
    } catch (e) {
      setError(errorText(e));
    }
  }

  async function remove(key: string) {
    const yes = await confirm({
      title: `Remove ${key} from the case?`,
      message: "Its verdict and reason go with it.",
      confirmLabel: "Remove",
      danger: true,
    });
    if (yes) await change(() => RemoveCaseProject(profileId, id, key));
  }

  if (!detail) return error ? <p role="alert" className="error-text">{error}</p> : <p className="muted">Reading the case...</p>;
  return (
    <section className="pie-trace-detail" aria-label={`Case ${detail.key}`}>
      <h2 className="mono">{detail.key}</h2>
      <p>{issue?.summary ?? "This issue is not in the run on screen."}</p>
      <label className="field-label" htmlFor="pie-case-notes">
        Notes
      </label>
      <textarea id="pie-case-notes" className="detail-input" rows={3} value={notes} onChange={(e) => setNotes(e.target.value)} />
      <div className="form-actions">
        <button type="button" className="btn" disabled={notes === detail.notes} onClick={() => void change(async () => {
          await SetCaseNotes(profileId, id, notes);
          return { ...detail, notes } as cases.Detail;
        })}>
          Save notes
        </button>
      </div>
      {error && <p role="alert" className="error-text">{error}</p>}

      <div className="pie-detail-head">
        <h3>Projects to assess</h3>
        <button type="button" className="btn" onClick={() => setPicking(true)}>
          Add projects
        </button>
      </div>
      {detail.projects.length === 0 ? (
        <p className="muted">No projects yet. Add the projects this issue may reach.</p>
      ) : (
        <table className="pie-table" aria-label="Verdicts">
          <thead>
            <tr>
              <th scope="col">Project</th>
              <th scope="col">Verdict</th>
              <th scope="col">Reason</th>
              <th scope="col">Last set</th>
              <th scope="col">
                <span className="sr-only">Remove</span>
              </th>
            </tr>
          </thead>
          <tbody>
            {detail.projects.map((p) => (
              <tr key={p.key}>
                <td>{projectLabel(p.key, names.get(p.key))}</td>
                <td>
                  <select
                    className="detail-input pie-filter-select"
                    aria-label={`Verdict for ${p.key}`}
                    value={p.verdict}
                    onChange={(e) => void change(() => SetVerdict(profileId, id, p.key, e.target.value, reasons[p.key] ?? ""))}
                  >
                    {VERDICTS.map((v) => (
                      <option key={v} value={v}>
                        {v}
                      </option>
                    ))}
                  </select>
                </td>
                <td className="pie-summary">
                  <input
                    className="detail-input"
                    aria-label={`Reason for ${p.key}`}
                    value={reasons[p.key] ?? ""}
                    onChange={(e) => setReasons({ ...reasons, [p.key]: e.target.value })}
                    onBlur={() => {
                      if ((reasons[p.key] ?? "") !== p.reason) void change(() => SetVerdict(profileId, id, p.key, p.verdict, reasons[p.key] ?? ""));
                    }}
                  />
                </td>
                <td>{when(p.assessedAt)}</td>
                <td>
                  <button type="button" className="btn btn-ghost" aria-label={`Remove ${p.key}`} onClick={() => void remove(p.key)}>
                    Remove
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {picking && (
        <ProjectPicker
          profileId={profileId}
          caseId={id}
          onClose={() => setPicking(false)}
          onAdded={(d) => {
            setPicking(false);
            show(d);
            onChanged();
          }}
        />
      )}
    </section>
  );
}
