import type { PreconditionsViewProps } from "./preconditions/usePreconditionsState";
import { usePreconditionsState } from "./preconditions/usePreconditionsState";
import { usePreconditionsActions } from "./preconditions/usePreconditionsActions";
import { PreconditionList } from "./preconditions/PreconditionList";
import { DetailHeader } from "./preconditions/DetailHeader";
import { DetailFields } from "./preconditions/DetailFields";
import { UsedByTests } from "./preconditions/UsedByTests";
import { Overlays } from "./preconditions/Overlays";
import { errMsg } from "../api";

// PreconditionsView is the dedicated management surface for Preconditions
// (FR-13.4): a searchable master list on the left, and a detail pane on the
// right to edit a Precondition's summary / type / description, see and manage
// the Tests that reference it, create new ones, and delete. Everything is
// computed from the local store and queued for commit; it recomputes when the
// profile changes or a sync / commit invalidates the query cache.
//
// The hooks in preconditions/ hold what it knows and does; the list, detail
// header and fields, Used by table and overlays render it.
export function PreconditionsView(p: PreconditionsViewProps) {
  const s = usePreconditionsState(p);
  const a = usePreconditionsActions(s, p);
  const { error, detailKey, listError, testsQuery, selectedPre } = s;

  return (
    <div
      className={`precond-view${detailKey ? " precond-with-detail" : ""}`}
      data-tour="preconditions-body"
    >
      <PreconditionList s={s} a={a} />

      <section className="precond-detail">
        {/* A live list/linked-tests load failure wins over a stale imperative
            error. */}
        {(listError || error || testsQuery.error) && (
          <div className="error-text">
            {listError || error || errMsg(testsQuery.error)}
          </div>
        )}

        {!selectedPre ? (
          <p className="muted precond-detail-empty">
            Select a precondition to view and manage it.
          </p>
        ) : (
          <>
            <DetailHeader s={s} a={a} />

            <DetailFields s={s} />

            <UsedByTests s={s} a={a} />
          </>
        )}
      </section>

      <Overlays s={s} p={p} />
    </div>
  );
}
