import { useCallback, useEffect, useRef, useState } from "react";
import { useConfirm, usePrompt } from "@agile-suite/core";
import { Cycles, DeleteCycle, GetCycle, PublishCycle, RetakeCycle, TakeCycle } from "../wailsjs/go/main/App";
import type { cycles, query } from "../wailsjs/go/models";
import { errorText } from "./ProfileForm";
import { VerdictGrid } from "./CaseGrid";
import { ExportButton } from "./ExportButton";
import { cycleSheets } from "./exportSheets";
import { projectLabel, projectNames } from "./linked";
import { Card } from "./Statistics";

// CyclesPage lists the profile's reporting cycles and shows one: what changed
// since the last published cycle, then its own copy of the cases.
export function CyclesPage({ profileId, result }: { profileId: string; result: query.Result | null }) {
  const [list, setList] = useState<cycles.Cycle[] | null>(null);
  const [openId, setOpenId] = useState<number | null>(null);
  const [error, setError] = useState("");
  const { prompt } = usePrompt();
  const { confirm } = useConfirm();

  // asked counts reloads, so a slow answer for an earlier profile or reload
  // cannot overwrite a later one.
  const asked = useRef(0);
  const reload = useCallback(
    async (select?: number) => {
      const mine = ++asked.current;
      try {
        const next = await Cycles(profileId);
        if (mine !== asked.current) return;
        setError("");
        setList(next);
        setOpenId(select ?? next[0]?.id ?? null);
      } catch (e) {
        if (mine === asked.current) setError(errorText(e));
      }
    },
    [profileId],
  );
  useEffect(() => {
    setList(null);
    setOpenId(null);
    void reload();
  }, [reload]);

  const take = async () => {
    const name = await prompt({ title: "New cycle", placeholder: "Name, such as Week 45", submitLabel: "Take" });
    if (name === null) return;
    try {
      const c = await TakeCycle(profileId, name);
      await reload(c.id);
    } catch (e) {
      setError(errorText(e));
    }
  };

  const remove = async (c: cycles.Cycle) => {
    const yes = await confirm({
      title: `Delete ${c.name}?`,
      message: "This removes the cycle and its copy of the cases from this computer.",
      confirmLabel: "Delete",
      danger: true,
    });
    if (!yes) return;
    try {
      await DeleteCycle(profileId, c.id);
      await reload();
    } catch (e) {
      setError(errorText(e));
    }
  };

  return (
    <div className="pie-linked pie-trace">
      <div className="pie-trace-side">
        <button type="button" className="btn btn-primary" onClick={() => void take()}>
          New cycle
        </button>
        {error && (
          <p role="alert" className="error-text">
            {error}
          </p>
        )}
        {list?.length === 0 && <p className="muted">No cycles yet. A new cycle keeps a copy of every case and its verdicts.</p>}
        <ul className="pie-set-list" aria-label="Cycles">
          {(list ?? []).map((c) => (
            <li key={c.id}>
              <button
                type="button"
                className={`pie-set pie-cycle${c.id === openId ? " pie-set-open" : ""}`}
                aria-pressed={c.id === openId}
                onClick={() => setOpenId(c.id)}
              >
                <span>{c.name}</span>
                <span className="muted">
                  {`Taken ${c.takenAt.slice(0, 10)}, `}
                  {c.publishedAt ? `published ${c.publishedAt.slice(0, 10)}` : "not published"}
                </span>
              </button>
            </li>
          ))}
        </ul>
      </div>
      {openId !== null && (
        <CycleDetail
          key={openId}
          profileId={profileId}
          id={openId}
          names={result ? projectNames(result) : new Map()}
          onDelete={(c) => void remove(c)}
          onPublished={() => void reload(openId)}
        />
      )}
    </div>
  );
}

function CycleDetail({
  profileId,
  id,
  names,
  onDelete,
  onPublished,
}: {
  profileId: string;
  id: number;
  names: Map<string, string>;
  onDelete: (c: cycles.Cycle) => void;
  onPublished: () => void;
}) {
  const [d, setD] = useState<cycles.Detail | null>(null);
  const [error, setError] = useState("");
  const [done, setDone] = useState("");
  const { confirm } = useConfirm();

  useEffect(() => {
    let current = true;
    GetCycle(profileId, id).then(
      (got) => current && setD(got),
      (e) => current && setError(errorText(e)),
    );
    return () => {
      current = false;
    };
  }, [profileId, id]);

  const retake = async () => {
    try {
      setError("");
      setD(await RetakeCycle(profileId, id));
    } catch (e) {
      setError(errorText(e));
    }
  };

  // publish asks Go first without confirming; Go answers with a question when
  // the page has the cycle's title already or was edited in Confluence.
  const publish = async () => {
    setError("");
    setDone("");
    try {
      let out = await PublishCycle(profileId, id, false);
      if (out.ask) {
        if (!(await confirm({ title: "Publish this cycle?", message: out.ask, confirmLabel: "Publish" }))) return;
        out = await PublishCycle(profileId, id, true);
      }
      setD(await GetCycle(profileId, id));
      setDone(`Published as ${out.title}.`);
      onPublished();
    } catch (e) {
      setError(errorText(e));
    }
  };

  const alert = error && (
    <p role="alert" className="error-text">
      {error}
    </p>
  );
  if (!d) return alert || <p className="muted">Reading the cycle...</p>;
  const ch = d.changes;
  return (
    <section className="pie-trace-detail" aria-labelledby="pie-cycle-heading">
      <div className="pie-filters">
        <h2 id="pie-cycle-heading">{d.name}</h2>
        <ExportButton name={`pie-cycle-${d.name}`} sheets={() => cycleSheets(d, (p) => projectLabel(p, names.get(p)))} />
        <button type="button" className="btn btn-primary" onClick={() => void publish()}>
          {d.publishedAt ? "Publish again" : "Publish"}
        </button>
        {!d.publishedAt && (
          <>
            <button type="button" className="btn" onClick={() => void retake()}>
              Take again
            </button>
            <button type="button" className="btn btn-danger" onClick={() => onDelete(d)}>
              Delete
            </button>
          </>
        )}
      </div>
      {alert}
      {done && (
        <p role="status" className="muted">
          {done}
        </p>
      )}
      <p className="muted">
        {ch.since
          ? `Compared with ${ch.since}, the last cycle published before this one.`
          : "No cycle was published before this one, so every case counts as new."}
      </p>
      <ul className="pie-stat-cards" aria-label="Changes">
        <Card value={ch.new.length} label="new cases" />
        <Card value={ch.gone.length} label="cases gone" />
        <Card value={ch.verdicts.length} label="verdicts changed" />
      </ul>
      {ch.new.length > 0 && <p className="mono">{`New: ${ch.new.join(", ")}`}</p>}
      {ch.gone.length > 0 && <p className="mono">{`Gone: ${ch.gone.join(", ")}`}</p>}
      {ch.verdicts.length > 0 && (
        <div className="pie-results">
          <table className="pie-table" aria-label="Verdicts that changed">
            <thead>
              <tr>
                <th scope="col">Case</th>
                <th scope="col">Project</th>
                <th scope="col">Was</th>
                <th scope="col">Now</th>
              </tr>
            </thead>
            <tbody>
              {ch.verdicts.map((v) => (
                <tr key={`${v.case}>${v.project}`}>
                  <td className="mono">{v.case}</td>
                  <td>{v.project}</td>
                  <td>{v.from}</td>
                  <td>{v.to}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
      <h3>Cases in this cycle</h3>
      <VerdictGrid rows={d.cases} names={names} />
    </section>
  );
}
