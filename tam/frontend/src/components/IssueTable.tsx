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
// Rows are virtualised so a 500-row page stays light; the overscan is wide
// enough that a viewport with no measured height (jsdom in tests) still
// renders a full 25-row page.
export function IssueTable({ issues, selectedKey, onSelect }: Props) {
  const scrollRef = useRef<HTMLDivElement>(null);
  const virtualizer = useVirtualizer({
    count: issues.length,
    getScrollElement: () => scrollRef.current,
    estimateSize: () => ROW_HEIGHT,
    overscan: 30,
  });

  return (
    <div className="issue-table" role="table" aria-label="Issues">
      <div className="issue-row issue-head" role="row" aria-label="key type summary status assignee sprint pts">
        <span role="columnheader">KEY</span>
        <span role="columnheader">TYPE</span>
        <span role="columnheader">SUMMARY</span>
        <span role="columnheader">STATUS</span>
        <span role="columnheader">ASSIGNEE</span>
        <span role="columnheader">SPRINT</span>
        <span role="columnheader">PTS</span>
      </div>
      <div className="issue-body" ref={scrollRef} role="rowgroup">
        <div style={{ height: virtualizer.getTotalSize(), position: "relative" }}>
          {virtualizer.getVirtualItems().map((v) => {
            const iss = issues[v.index];
            const selected = iss.key === selectedKey;
            return (
              <div
                key={iss.key}
                role="row"
                aria-selected={selected}
                aria-label={`${iss.key} ${iss.summary}`}
                className={`issue-row${selected ? " issue-row-selected" : v.index % 2 ? " issue-row-alt" : ""}`}
                style={{ position: "absolute", top: 0, left: 0, right: 0, height: v.size, transform: `translateY(${v.start}px)` }}
                onClick={() => onSelect(iss.key)}
                onKeyDown={(e) => {
                  if (e.key === "Enter" || e.key === " ") {
                    e.preventDefault();
                    onSelect(iss.key);
                  }
                }}
                tabIndex={0}
              >
                <span role="cell" className="issue-key">{iss.key}</span>
                <span role="cell"><TypeChip type={iss.type} /></span>
                <span role="cell" className="issue-summary" title={iss.summary}>{iss.summary}</span>
                <span role="cell"><span className={`chip chip-status chip-status-${statusClass(iss.status)}`}>{iss.status}</span></span>
                <span role="cell">{iss.assignee || "-"}</span>
                <span role="cell" title={iss.sprintName}>{iss.sprintId || "-"}</span>
                <span role="cell">{iss.storyPoints ?? "-"}</span>
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
