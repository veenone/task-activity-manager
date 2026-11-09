import { type CSSProperties, useEffect, useMemo, useRef, useState } from "react";
import { RefreshLinks } from "../wailsjs/go/main/App";
import type { query } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";
import { ExportButton } from "./ExportButton";
import { linkSheets } from "./exportSheets";
import { LinkFlow } from "./LinkFlow";
import { ProjectsChooser } from "./ProjectInput";
import { LinkNetwork } from "./LinkNetwork";
import { byKey, projectLabel, projectNames } from "./linked";
import { byCell, type Edge, edges as edgesOf, projectOf } from "./linkData";

const LINK_VIEWS = ["Grid", "Flow", "Network"] as const;

// LinkMap counts Jira links between projects by type, each read from its
// outward end, so a row's project points at a column's project in the
// outward wording. A cell lists its links below the grid.
export function LinkMap({
  result,
  onOpen,
  refresh: { profileId, queryId, runAt, onResult },
}: {
  result: query.Result;
  onOpen: (key: string) => void;
  // refresh names the run on screen, for Refresh links: the saved query it
  // belongs to (0 for none) and when it ran.
  refresh: { profileId: string; queryId: number; runAt: string; onResult: (r: query.Result) => void };
}) {
  const [view, setView] = useState<(typeof LINK_VIEWS)[number]>("Grid");
  const [refreshing, setRefreshing] = useState(false);
  const [refreshError, setRefreshError] = useState("");
  // onScreen is the run on screen, or null once the map is gone. A refresh whose
  // run is no longer shown answers for a run the analyst has left (#436).
  const onScreen = useRef<query.Result | null>(result);
  onScreen.current = result;
  useEffect(
    () => () => {
      onScreen.current = null;
    },
    [],
  );
  const [off, setOff] = useState<Set<string>>(new Set());
  const [inside, setInside] = useState(false);
  const [keyText, setKeyText] = useState("");
  const [picked, setPicked] = useState<string[]>([]);
  const [bothEnds, setBothEnds] = useState(true);
  const [chosen, setChosen] = useState<{ from: string; to: string } | null>(null);
  // Everything below reads the run once per change of run or filter. A large
  // run has thousands of links, and scanning them for every cell on every
  // render is what made the map slow (#431).
  const { issues, names, edges, types, stale } = useMemo(
    () => ({ issues: byKey(result), names: projectNames(result), ...edgesOf(result.issues) }),
    [result],
  );
  const label = (p: string) => projectLabel(p, names.get(p));
  const { shown, projects } = useMemo(() => {
    const ends = (e: Edge) => [projectOf(e.from, issues), projectOf(e.to, issues)] as const;
    const text = keyText.trim().toLowerCase();
    const kept = edges.filter((e) => {
      const [pf, pt] = ends(e);
      return (
        (inside || pf !== pt) &&
        (!bothEnds || (issues.has(e.from) && issues.has(e.to))) &&
        (picked.length === 0 || (picked.includes(pf) && picked.includes(pt))) &&
        (!text || e.from.toLowerCase().includes(text) || e.to.toLowerCase().includes(text))
      );
    });
    return { shown: kept, projects: [...new Set(kept.flatMap((e) => ends(e)))].sort() };
  }, [edges, issues, inside, bothEnds, picked, keyText]);
  const everyProject = useMemo(
    () => [...new Set(edges.flatMap((e) => [projectOf(e.from, issues), projectOf(e.to, issues)]))].sort(),
    [edges, issues],
  );
  const ticked = useMemo(() => shown.filter((e) => !off.has(e.type)), [shown, off]);
  const cells = useMemo(() => byCell(ticked, issues), [ticked, issues]);
  const between = (from: string, to: string) => cells.get(`${from}>${to}`) ?? [];
  const most = Math.max(1, ...[...cells.values()].map((c) => c.length));

  // Refresh links fetches the linked issues again and keeps the query's own
  // rows, so it costs a fraction of a run (#435).
  const refreshLinks = async () => {
    setRefreshing(true);
    setRefreshError("");
    try {
      const fresh = await RefreshLinks(profileId, queryId, runAt, result);
      if (onScreen.current === result) onResult(fresh);
    } catch (e) {
      setRefreshError(errorText(e));
    } finally {
      setRefreshing(false);
    }
  };
  const words = (type: string) => {
    const t = types.find((x) => x.name === type);
    return t?.outward || t?.inward || type;
  };

  if (stale) {
    return (
      <p role="alert" className="error-text">
        This run was stored before PIE kept the direction of each link. Run the query again to see the Link map.
      </p>
    );
  }

  const cellLinks = chosen ? between(chosen.from, chosen.to) : [];
  return (
    <div className="pie-linkmap">
      <fieldset className="pie-linkmap-rail" aria-label="Link types">
        <legend>Link types</legend>
        <button type="button" className="btn" disabled={refreshing} aria-busy={refreshing} onClick={() => void refreshLinks()}>
          Refresh links
        </button>
        <ProjectsChooser projects={everyProject} projectLabel={label} chosen={picked} onChange={setPicked} />
        <label>
          Key
          <input
            className="detail-input"
            type="search"
            value={keyText}
            placeholder="Part of a key"
            onChange={(e) => setKeyText(e.target.value)}
          />
        </label>
        {refreshError && (
          <p role="alert" className="error-text">
            {refreshError}
          </p>
        )}
        <div className="pie-linkmap-all">
          <button type="button" className="btn btn-ghost" onClick={() => setOff(new Set())}>
            All
          </button>
          <button type="button" className="btn btn-ghost" onClick={() => setOff(new Set(types.map((t) => t.name)))}>
            None
          </button>
        </div>
        {types.map((t) => (
          <div key={t.name} className="pie-linkmap-type">
            <label className="pie-check">
              <input
                type="checkbox"
                checked={!off.has(t.name)}
                onChange={() => {
                  const next = new Set(off);
                  if (next.has(t.name)) next.delete(t.name);
                  else next.add(t.name);
                  setOff(next);
                }}
              />
              {t.name} <span className="muted">{shown.filter((e) => e.type === t.name).length}</span>
            </label>
            <ul className="muted">
              {t.outward === t.inward ? (
                <li>{`${t.outward} (both ways)`}</li>
              ) : (
                <>
                  {t.outward && (
                    <li>
                      <span aria-hidden="true">{"→"}</span> <span>{t.outward}</span>
                    </li>
                  )}
                  {t.inward && (
                    <li>
                      <span aria-hidden="true">{"←"}</span> <span>{t.inward}</span>
                    </li>
                  )}
                </>
              )}
            </ul>
          </div>
        ))}
        <label className="pie-check">
          <input type="checkbox" checked={inside} onChange={(e) => setInside(e.target.checked)} />
          Show links inside one project
        </label>
        <label className="pie-check">
          <input type="checkbox" checked={bothEnds} onChange={(e) => setBothEnds(e.target.checked)} />
          Only links where both ends are in the run
        </label>
        <p className="muted">The wording is Jira&apos;s own. A type worded the same both ways has no direction.</p>
      </fieldset>
      <div className="pie-linkmap-main">
        <div className="pie-filters">
          <ExportButton name="pie-link-map" sheets={() => linkSheets(projects, between, ticked, issues, label)} />
          <div className="view-tabs pie-map-views" role="group" aria-label="Link view">
            {LINK_VIEWS.map((v) => (
              <button
                key={v}
                type="button"
                className={`view-tab${v === view ? " view-tab-active" : ""}`}
                aria-pressed={v === view}
                onClick={() => setView(v)}
              >
                {v}
              </button>
            ))}
          </div>
        </div>
        {view === "Flow" && <LinkFlow edges={ticked} issues={issues} label={label} />}
        {view === "Network" && <LinkNetwork edges={ticked} issues={issues} label={label} />}
        {view === "Grid" && (
          <>
            <div className="pie-results">
              <table className="pie-table pie-map pie-linkmap-grid" aria-label="Projects connected by links">
                <thead>
                  <tr>
                    <th scope="col">From \ to</th>
                    {projects.map((p) => (
                      <th key={p} scope="col" title={label(p)}>
                        {p}
                      </th>
                    ))}
                    <th scope="col">Total</th>
                  </tr>
                </thead>
                <tbody>
                  {projects.map((from) => (
                    <tr key={from}>
                      <th scope="row">{label(from)}</th>
                      {projects.map((to) => {
                        const here = between(from, to);
                        if (here.length === 0)
                          return (
                            <td key={to} className="pie-linkmap-empty">
                              -
                            </td>
                          );
                        const counts = new Map<string, number>();
                        here.forEach((e) => counts.set(e.type, (counts.get(e.type) ?? 0) + 1));
                        const open = chosen?.from === from && chosen.to === to;
                        return (
                          <td
                            key={to}
                            className="pie-linkmap-full"
                            style={{ "--shade": `${Math.round(8 + (32 * here.length) / most)}%` } as CSSProperties}
                          >
                            <button
                              type="button"
                              className="pie-linkmap-cell"
                              aria-pressed={open}
                              aria-label={`${label(from)} to ${label(to)}: ${here.length === 1 ? "1 link" : `${here.length} links`}`}
                              onClick={() => setChosen(open ? null : { from, to })}
                            >
                              {[...counts].map(([type, n]) => (
                                <span key={type} className="pie-linkmap-chip">{`${type} ${n}`}</span>
                              ))}
                            </button>
                          </td>
                        );
                      })}
                      <td className="pie-linkmap-total">{projects.reduce((n, to) => n + between(from, to).length, 0)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            {projects.length === 0 && <p className="muted">No links between projects to show.</p>}
            {chosen && (
              <section className="pie-linkmap-links">
                <div className="pie-filters">
                  <h3>{`${label(chosen.from)} to ${label(chosen.to)}`}</h3>
                  <button type="button" className="btn btn-ghost" onClick={() => setChosen({ from: chosen.to, to: chosen.from })}>
                    {`${label(chosen.to)} to ${label(chosen.from)} (${between(chosen.to, chosen.from).length})`}
                  </button>
                </div>
                <table className="pie-table" aria-label={`Links from ${label(chosen.from)} to ${label(chosen.to)}`}>
                  <thead>
                    <tr>
                      <th scope="col">From issue</th>
                      <th scope="col">Status</th>
                      <th scope="col">Link</th>
                      <th scope="col">To issue</th>
                      <th scope="col">Status</th>
                      <th scope="col">Summary</th>
                    </tr>
                  </thead>
                  <tbody>
                    {cellLinks.map((e) => (
                      <tr key={`${e.from}>${e.to}>${e.type}`}>
                        <IssueCells issueKey={e.from} issue={issues.get(e.from)} onOpen={onOpen} />
                        <td>{words(e.type)}</td>
                        <IssueCells issueKey={e.to} issue={issues.get(e.to)} onOpen={onOpen} />
                        <td className="pie-summary">{issues.get(e.from)?.summary ?? "-"}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
                {cellLinks.length === 0 && <p className="muted">No links this way.</p>}
              </section>
            )}
          </>
        )}
      </div>
    </div>
  );
}

// An issue the run did not fetch cannot open in Trace, so its key is text.
function IssueCells({ issueKey, issue, onOpen }: { issueKey: string; issue?: query.Issue; onOpen: (key: string) => void }) {
  return (
    <>
      <td className="mono">
        {issue ? (
          <button type="button" className="pie-map-key" onClick={() => onOpen(issueKey)}>
            {issueKey}
          </button>
        ) : (
          issueKey
        )}
      </td>
      <td>{issue?.status ?? "-"}</td>
    </>
  );
}
