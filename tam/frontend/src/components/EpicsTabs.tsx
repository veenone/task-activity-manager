import { useRef } from "react";
import type { KeyboardEvent } from "react";

export type EpicsTab = "epics" | "noepic";

export const epicsTabId = (t: EpicsTab) => `epics-tab-${t}`;
export const epicsPanelId = (t: EpicsTab) => `epics-panel-${t}`;

// EpicsTabs splits the view by the question being asked: how the work under
// each epic is going, and what was never filed under an epic at all (#341).
// The two groups want different shapes, a tree and a paged list, and the
// second had grown past anything a tree can draw.
//
// Each tab carries its own total, counted over that whole side rather than
// over what the current filter leaves, so the labels say how big the two
// halves of the project are and hold still while their contents narrow.
// Between them they account for every non-epic issue in the cache.
//
// A tablist with a roving tabindex, so the arrow keys move between the tabs
// and Tab moves past them, the pattern ReportTabs and core's rich text
// field already use.
export function EpicsTabs({
  current, onSelect, issueTotal, orphanTotal,
}: {
  current: EpicsTab;
  onSelect: (t: EpicsTab) => void;
  issueTotal: number;
  orphanTotal: number;
}) {
  const refs = useRef<Partial<Record<EpicsTab, HTMLButtonElement | null>>>({});
  const tabs: { id: EpicsTab; label: string; total: number }[] = [
    { id: "epics", label: "Under epics", total: issueTotal },
    { id: "noepic", label: "No epic", total: orphanTotal },
  ];

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>) {
    if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
    e.preventDefault();
    const i = tabs.findIndex((t) => t.id === current);
    const next = tabs[(i + (e.key === "ArrowRight" ? 1 : -1) + tabs.length) % tabs.length].id;
    onSelect(next);
    refs.current[next]?.focus();
  }

  return (
    <div className="richfield-tabs epics-tabs" role="tablist" aria-label="Epics and issues with no epic">
      {tabs.map((t) => (
        <button
          key={t.id}
          ref={(el) => {
            refs.current[t.id] = el;
          }}
          type="button"
          role="tab"
          id={epicsTabId(t.id)}
          className="richfield-tab"
          // The count is in the accessible name rather than read as a bare
          // number after the label, so a screen reader says what it counts.
          aria-label={`${t.label}, ${t.total.toLocaleString()} issues`}
          aria-selected={current === t.id}
          aria-controls={epicsPanelId(t.id)}
          tabIndex={current === t.id ? 0 : -1}
          onClick={() => onSelect(t.id)}
          onKeyDown={onKeyDown}
        >
          {t.label}
          <span className="epics-tab-count">{t.total.toLocaleString()}</span>
        </button>
      ))}
    </div>
  );
}
