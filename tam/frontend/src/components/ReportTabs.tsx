import { useRef } from "react";
import type { KeyboardEvent } from "react";

export type ReportTab = "sprint" | "people";

const TABS: { id: ReportTab; label: string }[] = [
  { id: "sprint", label: "Sprint report" },
  { id: "people", label: "People" },
];

export const reportTabId = (t: ReportTab) => `report-tab-${t}`;
export const reportPanelId = (t: ReportTab) => `report-panel-${t}`;

// ReportTabs splits a sprint report by the question it answers: how the
// sprint went, which keeps the one-window layout #113 built, and who carried
// it (#287). The outputs still carry every section; this only chooses what
// the screen shows. A tablist with a roving tabindex, so the arrow keys move
// between the tabs and Tab moves past them, the pattern core's rich text
// field uses for Write and Preview.
export function ReportTabs({ current, onSelect }: { current: ReportTab; onSelect: (t: ReportTab) => void }) {
  const refs = useRef<Partial<Record<ReportTab, HTMLButtonElement | null>>>({});

  function onKeyDown(e: KeyboardEvent<HTMLButtonElement>) {
    if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
    e.preventDefault();
    const i = TABS.findIndex((t) => t.id === current);
    const next = TABS[(i + (e.key === "ArrowRight" ? 1 : -1) + TABS.length) % TABS.length].id;
    onSelect(next);
    refs.current[next]?.focus();
  }

  return (
    <div className="richfield-tabs report-tabs" role="tablist" aria-label="Report views">
      {TABS.map((t) => (
        <button
          key={t.id}
          ref={(el) => {
            refs.current[t.id] = el;
          }}
          type="button"
          role="tab"
          id={reportTabId(t.id)}
          className="richfield-tab"
          aria-selected={current === t.id}
          aria-controls={reportPanelId(t.id)}
          tabIndex={current === t.id ? 0 : -1}
          onClick={() => onSelect(t.id)}
          onKeyDown={onKeyDown}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}
