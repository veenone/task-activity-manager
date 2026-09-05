import { useRef } from "react";
import { useVirtualizer } from "@tanstack/react-virtual";
import type { Issue } from "../api";
import { TypeChip } from "./TypeChip";

const ROW_HEIGHT = 34;

interface Props {
  issues: Issue[];
  selectedKey: string;
  onSelect: (key: string) => void;
}

// IssueTable renders one page of issues with the mockup's seven columns.
// Rows are virtualised so a long page stays light. jsdom performs no layout,
// so the test stubs the scroll container's height (see BacklogView.test.tsx).
export function IssueTable({ issues, selectedKey, onSelect }: Props) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: issues.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 30,
  });

  // moveFocus walks the roving tabindex to a neighbouring row. The rows are
  // absolutely positioned siblings, so the index attribute is what finds them.
  function moveFocus(from: number, delta: number) {
    const next = scrollRef.current?.querySelector<HTMLElement>(`[data-row-index="${from + delta}"]`);
    next?.focus();
  }

  return (
    <div
      className="issue-table"
      role="grid"
      aria-label="Issues"
      aria-multiselectable="false"
      aria-rowcount={issues.length + 1}
    >
      <div className="issue-row issue-head" role="row" aria-rowindex={1} aria-label="key type summary status assignee sprint pts">
        <span role="columnheader">KEY</span>
        <span role="columnheader">TYPE</span>
        <span role="columnheader">SUMMARY</span>
        <span role="columnheader">STATUS</span>
        <span role="columnheader">ASSIGNEE</span>
        <span role="columnheader">SPRINT</span>
        <span role="columnheader">PTS</span>
      </div>
      <div className="issue-body" ref={scrollRef} role="rowgroup">
        <div role="presentation" style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
          {virtualizer.getVirtualItems().map((v) => {
            const iss = issues[v.index];
            const selected = iss.key === selectedKey;
            return (
              <div
                key={iss.key}
                role="row"
                aria-selected={selected}
                aria-rowindex={v.index + 2}
                aria-label={`${iss.key} ${iss.summary}`}
                data-row-index={v.index}
                className={`issue-row${selected ? " issue-row-selected" : v.index % 2 ? " issue-row-alt" : ""}`}
                style={{ position: "absolute", top: 0, left: 0, right: 0, height: v.size, transform: `translateY(${v.start}px)` }}
                onClick={() => onSelect(iss.key)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelect(iss.key);
                  } else if (e.key === "ArrowDown") {
                    e.preventDefault();
                    moveFocus(v.index, 1);
                  } else if (e.key === "ArrowUp") {
                    e.preventDefault();
                    moveFocus(v.index, -1);
                  }
                }}
                tabIndex={selected || (selectedKey === "" && v.index === 0) ? 0 : -1}
              >
                <span role="gridcell" className="issue-key">{iss.key}</span>
                <span role="gridcell"><TypeChip type={iss.type} /></span>
                <span role="gridcell" className="issue-summary" title={iss.summary}>{iss.summary}</span>
                <span role="gridcell"><span className={`chip chip-status chip-status-${statusClass(iss.status)}`}>{iss.status}</span></span>
                <span role="gridcell">{iss.assignee || "-"}</span>
                <span role="gridcell" title={iss.sprintName}>{iss.sprintId || "-"}</span>
                <span role="gridcell">{iss.storyPoints ?? "-"}</span>
              </div>
            );
          })}
        </div>
      </div>
    </div>
  );
}

// statusClass buckets a Jira status name into the three colours the
// mockup uses: done, in progress, and everything else.
function statusClass(status: string): "done" | "active" | "todo" {
  const s = status.toLowerCase();
  if (s === "done" || s === "closed" || s === "resolved" || s === "approved") return "done";
  if (s.includes("progress") || s === "in review") return "active";
  return "todo";
}
