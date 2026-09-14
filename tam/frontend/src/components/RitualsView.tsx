import { useEffect, useRef, useState } from "react";
import { useProfile } from "@agile-suite/core";
import {
  GetConfluenceConfig, GetRitualPage, GetSprintReport, ListBoards, ListBoardSprints,
  ListConfluenceChildPages, ListRitualAssociations, ListRitualDrafts, ListSprintIssues, ScaffoldSprintRituals,
} from "../api";
import type {
  Board, ConfluenceConfig, ConfluencePage, Issue, Profile, RitualAssociation, RitualDraft, Settings, Sprint, SprintReport,
} from "../api";
import { summarySentence } from "../lib/reportText";
import { sanitizeHtml } from "../lib/sanitizeHtml";
import { RitualWizard } from "./RitualWizard";

const RITUAL_META: Record<string, { label: string; icon: "calendar" | "pulse" | "check" | "refresh" }> = {
  planning: { label: "Planning", icon: "calendar" }, standup: { label: "Standup", icon: "pulse" },
  review: { label: "Review", icon: "check" }, retro: { label: "Retrospective", icon: "refresh" },
};

function RitualIcon({ name }: { name: "calendar" | "pulse" | "check" | "refresh" }) {
  const paths = { calendar: <><rect x="4" y="5" width="16" height="15" rx="2" /><path d="M8 3v4M16 3v4M4 10h16" /></>, pulse: <path d="M3 12h4l2-6 4 12 2-6h6" />, check: <path d="m5 12 4 4L19 6" />, refresh: <path d="M20 11a8 8 0 0 0-14.7-3L3 11m0 0V5m0 6h6M4 13a8 8 0 0 0 14.7 3L21 13m0 0v6m0-6h-6" /> };
  return <svg className="ritual-icon-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

function RitualTemplate({ type, report }: { type: string; report: SprintReport | null }) {
  const items = type === "planning" ? ["Confirm the sprint goal", "Review capacity and priorities", "Call out risks before work starts"]
    : type === "standup" ? ["What moved yesterday?", "What moves today?", "What is blocked and needs help?"]
      : type === "review" ? ["Demo completed work", "Capture stakeholder feedback", "Record follow-up work"]
        : ["What should we keep?", "What should we improve?", "What will we try next sprint?"];
  return <section className="ritual-template" aria-label="Ritual template">
    <div className="ritual-template-head"><h4>{RITUAL_META[type]?.label ?? "Ritual"} template</h4><span className="muted small">Jira snapshot</span></div>
    {report && !report.unavailable && <p className="ritual-template-context">{report.series.sprintName}: {report.series.completed} completed of {report.series.committed} committed {report.series.unit}.</p>}
    <ul>{items.map((item) => <li key={item}><span aria-hidden="true">□</span>{item}</li>)}</ul>
  </section>;
}

// RITUAL_ORDER is the sprint slots' fixed display order: planning opens the
// sprint, standup runs through it, review and retro close it. Fixed rather
// than however ListRitualDrafts happens to order its rows (alphabetically,
// on the ritual_type column), so the slots read the same way every sprint.
const RITUAL_ORDER = ["planning", "standup", "review", "retro"];

export function RitualsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const [config, setConfig] = useState<ConfluenceConfig | null>(null);
  const [associations, setAssociations] = useState<RitualAssociation[]>([]);
  const [page, setPage] = useState<ConfluencePage | null>(null);
  const [report, setReport] = useState<SprintReport | null>(null);
  const [error, setError] = useState("");
  const [loadingPage, setLoadingPage] = useState(false);
  const [selectedID, setSelectedID] = useState("");
  const headingRef = useRef<HTMLHeadingElement>(null);

  // The sprint the authoring slots belong to: a scrum board and one of its
  // sprints, the issues that sprint holds (what the wizard offers), the
  // drafts already stored for it, and which ritual type the wizard is open
  // on, if any. This is a separate concern from the association nav above:
  // linking a page somebody already wrote is a different job from authoring
  // one, and both live on this view side by side.
  const [ritualBoards, setRitualBoards] = useState<Board[]>([]);
  const [ritualBoardId, setRitualBoardId] = useState(0);
  const [ritualSprints, setRitualSprints] = useState<Sprint[]>([]);
  const [ritualSprintId, setRitualSprintId] = useState(0);
  const [ritualIssues, setRitualIssues] = useState<Issue[]>([]);
  const [ritualDrafts, setRitualDrafts] = useState<RitualDraft[] | null>(null);
  const [ritualError, setRitualError] = useState("");
  const [scaffolding, setScaffolding] = useState(false);
  const [wizardType, setWizardType] = useState<string | null>(null);

  async function loadAssociations() {
    const next = await ListRitualAssociations(activeId, 0, 0);
    setAssociations(next);
    return next;
  }

  useEffect(() => {
    let live = true;
    setConfig(null); setPage(null); setReport(null); setSelectedID(""); setError("");
    void GetConfluenceConfig(activeId).then((c) => {
      if (!live) return;
      setConfig(c);
      if (!c.rootPageID) return;
      if (c.baseURL.trim().toLowerCase() === "demo") { void loadAssociations().then((items) => { const standup = items.find((item) => item.ritualType === "standup"); if (standup && live) openAssociation(standup); }).catch((e) => live && setError(String(e))); return; }
      void Promise.all([loadAssociations(), ListConfluenceChildPages(activeId, c.rootPageID, 0, 100)]).then(([items]) => { const standup = items.find((item) => item.ritualType === "standup"); if (standup && live) openAssociation(standup); }).catch((e) => live && setError(String(e)));
    }).catch((e) => live && setError(String(e)));
    return () => { live = false; };
  }, [activeId]);

  // Which scrum board the slots draw from. Keyed off the Confluence config
  // rather than off nothing, so a profile whose Confluence is unconfigured
  // never spends a call on boards it cannot author rituals for anyway (the
  // early return below leaves this view before it draws slots at all).
  useEffect(() => {
    let live = true;
    setRitualBoards([]); setRitualBoardId(0); setRitualSprints([]); setRitualSprintId(0);
    setRitualDrafts(null); setRitualIssues([]); setRitualError("");
    if (!config?.baseURL) return;
    void ListBoards(activeId).then((boards) => {
      if (!live) return;
      const scrum = boards.filter((b) => b.type === "scrum");
      setRitualBoards(scrum);
      setRitualBoardId(scrum[0]?.id ?? 0);
    }).catch((e) => live && setRitualError(String(e)));
    return () => { live = false; };
  }, [activeId, config?.baseURL]);

  // Which of that board's sprints the slots belong to. An active sprint is
  // the one a team is authoring rituals for most of the time; the picker
  // below lets a planning session ahead of it, or a review just after
  // close, choose a different one.
  useEffect(() => {
    let live = true;
    setRitualSprints([]); setRitualSprintId(0);
    if (!ritualBoardId) return;
    void ListBoardSprints(activeId, ritualBoardId).then((sprints) => {
      if (!live) return;
      setRitualSprints(sprints);
      const active = sprints.find((s) => s.state === "active");
      setRitualSprintId(active?.id ?? sprints[0]?.id ?? 0);
    }).catch((e) => live && setRitualError(String(e)));
    return () => { live = false; };
  }, [activeId, ritualBoardId]);

  function loadRitualSlots() {
    setRitualDrafts(null); setRitualError("");
    return Promise.all([
      ListRitualDrafts(activeId, ritualBoardId, ritualSprintId),
      ListSprintIssues(activeId, ritualBoardId, ritualSprintId),
    ]).then(([drafts, issues]) => { setRitualDrafts(drafts); setRitualIssues(issues); })
      .catch((e) => setRitualError(String(e)));
  }

  useEffect(() => {
    if (!ritualBoardId || !ritualSprintId) return;
    void loadRitualSlots();
    // loadRitualSlots reads ritualBoardId and ritualSprintId directly; it is
    // not itself a dependency, and adding it would refire this effect every
    // render since it is redefined each time.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeId, ritualBoardId, ritualSprintId]);

  function scaffoldSprint() {
    setScaffolding(true); setRitualError("");
    void ScaffoldSprintRituals(activeId, ritualBoardId, ritualSprintId)
      .then((drafts) => setRitualDrafts(drafts))
      .catch((e) => setRitualError(String(e)))
      .finally(() => setScaffolding(false));
  }

  function openAssociation(association: RitualAssociation) {
    setSelectedID(association.pageID); setError(""); setLoadingPage(true); setReport(null);
    void Promise.all([GetRitualPage(activeId, association.pageID), association.sprintID ? GetSprintReport(activeId, association.boardID, association.sprintID, false) : Promise.resolve(null)])
      .then(([nextPage, nextReport]) => { setPage(nextPage); setReport(nextReport); requestAnimationFrame(() => headingRef.current?.focus()); })
      .catch((e) => setError(String(e)))
      .finally(() => setLoadingPage(false));
  }

  const missingPage = /not found|404/i.test(error);

  if (error && !page) return <section className="backlog" aria-label="Rituals"><h2>Rituals</h2>{missingPage ? <div className="ritual-missing-page" role="alert"><div className="ritual-missing-code">404</div><h3>Ritual page unavailable</h3><p>This document may have been deleted, moved, or is no longer accessible.</p><button className="btn" onClick={() => setError("")}>Back to documents</button></div> : <><p className="error-text" role="alert">Could not load Rituals: {error}</p><button className="btn" onClick={() => window.location.reload()}>Retry</button></>}</section>;
  if (!config) return <section className="backlog" aria-label="Rituals"><h2>Rituals</h2><p className="muted" role="status">Loading Confluence settings</p></section>;
  if (!config.baseURL) return <section className="backlog" aria-label="Rituals"><h2>Rituals</h2><p className="muted">Confluence is not configured for this profile. Add it in Profile settings.</p></section>;

  const ritualBoard = ritualBoards.find((b) => b.id === ritualBoardId);
  const ritualSprint = ritualSprints.find((s) => s.id === ritualSprintId);

  return <section className="backlog rituals-view" aria-label="Rituals">
    <h2>Rituals</h2>
    {error && <p className="error-text" role="alert">{error} <button className="btn btn-ghost" onClick={() => { const selected = associations.find((a) => a.pageID === selectedID); if (selected) openAssociation(selected); else setError(""); }}>Retry</button></p>}

    {ritualBoards.length > 0 && (
      <section aria-label="Sprint rituals" className="ritual-slots-frame">
        <div className="board-head">
          {ritualBoards.length > 1 ? (
            <label className="board-picker">
              <span>Board</span>
              <select aria-label="Ritual board" value={ritualBoardId} onChange={(e) => setRitualBoardId(Number(e.target.value))}>
                {ritualBoards.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
              </select>
            </label>
          ) : ritualBoard && <h3 className="board-head-name">{ritualBoard.name}</h3>}
          {ritualSprints.length > 0 && (
            <label className="board-picker">
              <span>Sprint</span>
              <select aria-label="Ritual sprint" value={ritualSprintId} onChange={(e) => setRitualSprintId(Number(e.target.value))}>
                {ritualSprints.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
              </select>
            </label>
          )}
        </div>

        {ritualError && <p className="error-text" role="alert">{ritualError}</p>}

        {ritualBoardId > 0 && ritualSprintId > 0 && (
          ritualDrafts === null ? <p className="muted" role="status">Loading this sprint's rituals</p>
            : ritualDrafts.length === 0 ? (
              <button className="btn" disabled={scaffolding} onClick={scaffoldSprint}>
                {scaffolding ? "Setting up rituals" : `Set up ${ritualSprint?.name ?? "this sprint"}'s rituals`}
              </button>
            ) : (
              <ul className="ritual-slots">
                {RITUAL_ORDER.map((type) => {
                  const draft = ritualDrafts.find((d) => d.ritualType === type);
                  if (!draft) return null;
                  const meta = RITUAL_META[type] ?? { label: type, icon: "calendar" as const };
                  return (
                    <li key={type} className="ritual-slot">
                      <button className="ritual-slot-open" onClick={() => setWizardType(type)}>
                        <span className="ritual-icon" aria-hidden="true"><RitualIcon name={meta.icon} /></span>
                        <span className="ritual-link-copy"><strong>{draft.title || meta.label}</strong><small>{meta.label}</small></span>
                      </button>
                      <span className="status">{draft.status}</span>
                    </li>
                  );
                })}
              </ul>
            )
        )}

        {wizardType && (
          <RitualWizard
            profileId={activeId}
            boardId={ritualBoardId}
            sprintId={ritualSprintId}
            ritualType={wizardType}
            sprintIssues={ritualIssues}
            onSaved={() => { setWizardType(null); void loadRitualSlots(); }}
            onCancel={() => setWizardType(null)}
          />
        )}
      </section>
    )}

    <div className="rituals-layout">
      <nav aria-label="Ritual documents"><h3>Documents <span className="muted">({associations.length})</span></h3>
        {associations.length === 0 ? <p className="muted">No ritual pages are associated yet. Add them in Profile settings.</p> : associations.map((a) => { const meta = RITUAL_META[a.ritualType] ?? { label: a.ritualType, icon: "calendar" as const }; return <button className={`ritual-link ritual-${a.ritualType}${selectedID === a.pageID ? " ritual-link-selected" : ""}`} aria-current={selectedID === a.pageID ? "page" : undefined} key={a.pageID} onClick={() => openAssociation(a)} disabled={loadingPage && selectedID === a.pageID}><span className="ritual-icon" aria-hidden="true"><RitualIcon name={meta.icon} /></span><span className="ritual-link-copy"><strong>{a.pageTitle || a.pageID}</strong><small>{meta.label}</small></span></button>; })}
      </nav>
    <article aria-label="Selected ritual page" aria-busy={loadingPage}>{page ? <><h3 ref={headingRef} tabIndex={-1}>{page.title}</h3><p className="ritual-page-meta">{RITUAL_META[associations.find((a) => a.pageID === selectedID)?.ritualType ?? ""]?.label ?? "Ritual document"}</p>{report && !report.unavailable && <p className="ritual-report-context">{summarySentence(report.series, false)}</p>}<div className="ritual-page-body" dangerouslySetInnerHTML={{ __html: sanitizeHtml(page.body.view.value || page.body.storage.value) }} /><RitualTemplate type={associations.find((a) => a.pageID === selectedID)?.ritualType ?? "standup"} report={report} /></> : <p className="muted">{loadingPage ? "Loading ritual page…" : "Select an associated ritual page."}</p>}</article>
    </div>
  </section>;
}
