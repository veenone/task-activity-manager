import { errMsg, useNotice } from "@agile-suite/core";
import type { AgreementRow } from "../lib/doneAgreement";
import { useDoneAgreement, useSetDoneAgreementTick } from "../queries/doneAgreement";

interface Props {
  profileId: string;
  boardId: number;
  issueKey: string;
  // The issue's sprint, or 0 for an issue in none. A sprint's additions to
  // the agreement apply to that sprint's issues alone.
  sprintId: number;
}

// agreementProgressLine is what the section's summary carries: how many of
// the items the agreement states are ticked for this issue. A tick made
// against wording the agreement has since changed counts towards neither
// number, because the issue is no longer held to those words. Nothing at all
// while there is no agreement to count against, which is what keeps the
// summary of a board without one from reading "0 of 0".
export function agreementProgressLine(rows: AgreementRow[] | undefined): string | undefined {
  const stated = (rows ?? []).filter((r) => r.stated);
  if (stated.length === 0) return undefined;
  return `${stated.filter((r) => r.ticked).length} of ${stated.length}`;
}

// DoneAgreementSection is the effective done agreement for one issue: the
// board's items, this sprint's additions, and a box per item that a reader
// ticks for this issue.
//
// The ticks are local. They are not journalled and not pushed on Commit,
// which makes them the one write in TAM that never reaches Jira, so the
// section says so where a reader ticks: Work log says the opposite two
// sections above, and a reader has no way to tell them apart otherwise. The
// reason is on tamstore.doneTickDDL, beside the table.
//
// An item is identified by its words, so a tick records the words it was made
// against. Reword an item and the old tick stays, shown against the old words
// and marked as made against wording since changed, because these ticks are
// what a team is shown at a sprint review and a tick that followed the
// document would be evidence that changed underneath the review.
export function DoneAgreementSection({ profileId, boardId, issueKey, sprintId }: Props) {
  const agreement = useDoneAgreement(profileId, boardId, issueKey, sprintId);
  const setTick = useSetDoneAgreementTick(profileId, boardId, issueKey, sprintId);
  const { notice } = useNotice();
  const rows = agreement.data ?? [];

  if (agreement.isPending) return <p className="muted">Loading the done agreement</p>;
  if (agreement.isError) {
    return (
      <p className="error-text" data-testid="agreement-error">
        Could not load the done agreement: {agreement.error.message}{" "}
        <button type="button" className="btn btn-ghost" onClick={() => void agreement.refetch()}>Retry</button>
      </p>
    );
  }
  if (rows.length === 0) {
    return <p className="muted">This board&apos;s done agreement has no items yet. The Rituals view is where a team writes them.</p>;
  }

  return (
    <>
      <ul className="linked-list">
        {rows.map((row, i) => {
          const id = `agreement-item-${i}`;
          return (
            <li key={row.text} className="linked-row">
              <input
                id={id}
                type="checkbox"
                checked={row.ticked}
                disabled={setTick.isPending}
                onChange={(e) =>
                  setTick.mutate(
                    { item: row.text, ticked: e.target.checked },
                    { onError: (err) => void notice({ title: "The tick was not saved", message: errMsg(err), tone: "error" }) },
                  )
                }
              />
              <label htmlFor={id}>{row.text}</label>
              {!row.stated && <span className="muted small">Made against wording the agreement has since changed.</span>}
            </li>
          );
        })}
      </ul>
      <p className="muted small">Kept in TAM. Not sent to Jira and not pushed on Commit.</p>
    </>
  );
}
