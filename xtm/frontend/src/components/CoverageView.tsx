import { errMsg } from "../api";
import { CoverageMap } from "./CoverageMap";
import { CoverageGuide } from "./CoverageGuide";
import type { CoverageViewProps } from "./coverage-view/useCoverageState";
import { useCoverageState } from "./coverage-view/useCoverageState";
import { useCoverageActions } from "./coverage-view/useCoverageActions";
import { RequirementList } from "./coverage-view/RequirementList";
import { Welcome } from "./coverage-view/Welcome";
import { DetailHead } from "./coverage-view/DetailHead";
import { DetailTabs } from "./coverage-view/DetailTabs";
import { VersionsTab } from "./coverage-view/VersionsTab";
import { CoverageMatrix } from "./coverage-view/CoverageMatrix";
import { GapsTable } from "./coverage-view/GapsTable";
import { ReuseTable } from "./coverage-view/ReuseTable";
import { Dialogs } from "./coverage-view/Dialogs";

// CoverageView is the bounded coverage module surface: a list of canonical
// functional requirements on the left, and for the selected one a parameter
// coverage matrix, a gap list, and a reuse view — all local-only (no Jira
// admin). Populate a model by importing the Excel template, then map tests to
// parameter values to drive the coverage %.
//
// The hooks in coverage-view/ hold what it knows and does; each panel and tab
// is its own component there.
export function CoverageView(p: CoverageViewProps) {
  const s = useCoverageState(p);
  const a = useCoverageActions(s, p);
  const { selected, tab, error, notice, listError, detailQuery, loading } = s;
  const { isDemo, demoVariant } = p;

  return (
    <div className="cov-root" data-tour="coverage-body">
      <RequirementList s={s} a={a} />

      <section className="cov-detail">
        {/* A live canonical-list load failure wins over a stale imperative error. */}
        {(listError || error || detailQuery.error) && (
          <div className="cov-error">
            {listError || error || errMsg(detailQuery.error)}
          </div>
        )}
        {notice && <div className="cov-notice">{notice}</div>}
        {!selected ? (
          <Welcome s={s} a={a} p={p} />
        ) : (
          <>
            <DetailHead s={s} a={a} />

            <DetailTabs s={s} />

            {tab === "guide" ? (
              <CoverageGuide />
            ) : tab === "map" ? (
              <CoverageMap isDemo={isDemo} demoVariant={demoVariant} />
            ) : tab === "versions" ? (
              <VersionsTab s={s} a={a} />
            ) : loading ? (
              <p className="cov-empty">Loading…</p>
            ) : tab === "matrix" ? (
              <CoverageMatrix s={s} a={a} />
            ) : tab === "gaps" ? (
              <GapsTable s={s} />
            ) : (
              <ReuseTable s={s} />
            )}
          </>
        )}
      </section>

      <Dialogs s={s} />
    </div>
  );
}
