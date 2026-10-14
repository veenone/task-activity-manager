import type { BugsPanelProps } from "./bugs/useBugsState";
import { useBugsState } from "./bugs/useBugsState";
import { useBugsActions } from "./bugs/useBugsActions";
import { BugListPane } from "./bugs/BugListPane";
import { BugDetailCard } from "./bugs/BugDetailCard";
import { AffectedTests } from "./bugs/AffectedTests";
import { BugSidebars } from "./bugs/BugSidebars";
import { AddToExecModal } from "./bugs/AddToExecModal";

// BugsPanel is a master-detail view of the defects linked to the profile's
// tests: a filterable, paginated bug list on the left and, for the selected
// bug, a detail pane on the right showing its full info plus the affected tests
// enriched with their consolidated run status. Bug keys open in the browser;
// test keys open the test detail.
//
// The hooks in bugs/ hold what it knows and does; the list pane, detail card,
// affected tests, sidebars and the Add to execution dialog render it.
export function BugsPanel(p: BugsPanelProps) {
  const s = useBugsState(p);
  const a = useBugsActions(s);
  const { sidebarDetail, sel } = s;

  return (
    <div className={`bugs-md${sidebarDetail ? " bugs-md-with-detail" : ""}`}>
      <BugListPane s={s} a={a} />

      <div className="bugs-md-detail">
        {!sel ? (
          <p className="muted">Select a bug to see its details.</p>
        ) : (
          <>
            <BugDetailCard s={s} />

            <AffectedTests s={s} p={p} />
          </>
        )}
      </div>

      <BugSidebars s={s} p={p} />

      <AddToExecModal s={s} a={a} />
    </div>
  );
}
