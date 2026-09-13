import { useEffect, useRef, useState } from "react";
import { useProfile } from "@agile-suite/core";
import { GetConfluenceConfig, GetRitualPage, GetSprintReport, ListConfluenceChildPages, ListRitualAssociations } from "../api";
import type { ConfluenceConfig, ConfluencePage, Profile, RitualAssociation, Settings, SprintReport } from "../api";
import { summarySentence } from "../lib/reportText";
import { sanitizeHtml } from "../lib/sanitizeHtml";

const RITUAL_META: Record<string, { label: string; icon: "calendar" | "pulse" | "check" | "refresh" }> = {
  planning: { label: "Planning", icon: "calendar" }, standup: { label: "Standup", icon: "pulse" },
  review: { label: "Review", icon: "check" }, retro: { label: "Retrospective", icon: "refresh" },
};

function RitualIcon({ name }: { name: "calendar" | "pulse" | "check" | "refresh" }) {
  const paths = { calendar: <><rect x="4" y="5" width="16" height="15" rx="2" /><path d="M8 3v4M16 3v4M4 10h16" /></>, pulse: <path d="M3 12h4l2-6 4 12 2-6h6" />, check: <path d="m5 12 4 4L19 6" />, refresh: <path d="M20 11a8 8 0 0 0-14.7-3L3 11m0 0V5m0 6h6M4 13a8 8 0 0 0 14.7 3L21 13m0 0v6m0-6h-6" /> };
  return <svg className="ritual-icon-svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{paths[name]}</svg>;
}

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

  return <section className="backlog rituals-view" aria-label="Rituals">
    <h2>Rituals</h2>
    {error && <p className="error-text" role="alert">{error} <button className="btn btn-ghost" onClick={() => { const selected = associations.find((a) => a.pageID === selectedID); if (selected) openAssociation(selected); else setError(""); }}>Retry</button></p>}
    <div className="rituals-layout">
      <nav aria-label="Ritual documents"><h3>Documents <span className="muted">({associations.length})</span></h3>
        {associations.length === 0 ? <p className="muted">No ritual pages are associated yet. Add them in Profile settings.</p> : associations.map((a) => { const meta = RITUAL_META[a.ritualType] ?? { label: a.ritualType, icon: "calendar" as const }; return <button className={`ritual-link ritual-${a.ritualType}${selectedID === a.pageID ? " ritual-link-selected" : ""}`} aria-current={selectedID === a.pageID ? "page" : undefined} key={a.pageID} onClick={() => openAssociation(a)} disabled={loadingPage && selectedID === a.pageID}><span className="ritual-icon" aria-hidden="true"><RitualIcon name={meta.icon} /></span><span className="ritual-link-copy"><strong>{a.pageTitle || a.pageID}</strong><small>{meta.label}</small></span></button>; })}
      </nav>
      <article aria-label="Selected ritual page" aria-busy={loadingPage}>{page ? <><h3 ref={headingRef} tabIndex={-1}>{page.title}</h3><p className="ritual-page-meta">{RITUAL_META[associations.find((a) => a.pageID === selectedID)?.ritualType ?? ""]?.label ?? "Ritual document"}</p>{report && !report.unavailable && <p className="ritual-report-context">{summarySentence(report.series, false)}</p>}<div className="ritual-page-body" dangerouslySetInnerHTML={{ __html: sanitizeHtml(page.body.view.value || page.body.storage.value) }} /></> : <p className="muted">{loadingPage ? "Loading ritual page…" : "Select an associated ritual page."}</p>}</article>
    </div>
  </section>;
}
