import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { DoneAgreementTicks, ListRitualDocuments, SetDoneAgreementTick } from "../api";
import type { RitualDocument } from "../api";
import { agreementRows, effectiveAgreement } from "../lib/doneAgreement";
import type { AgreementFigures } from "../lib/reportDocument";
import { DONE_AGREEMENT } from "../lib/ritualText";
import { keys } from "./keys";

// The done agreement one issue is held to, with the ticks made against it.
//
// Two reads, both local, in one query: the documents come from
// ListRitualDocuments, which is how every other surface reaches a ritual
// document and which already answers a sprint's list with the board's
// standing agreement beside it, and the ticks from the store. Neither touches
// Jira, so this runs when the panel opens rather than when the section is
// expanded: the count belongs in the section's summary, where it is worth
// something to a reader who has not opened it.
function bodyOf(docs: RitualDocument[], sprintId: number): string {
  return docs.find((d) => d.ritualType === DONE_AGREEMENT && d.sprintId === sprintId)?.body ?? "";
}

export function useDoneAgreement(profileId: string, boardId: number, issueKey: string, sprintId: number) {
  return useQuery({
    queryKey: keys.doneAgreement(profileId, issueKey, boardId, sprintId),
    queryFn: async () => {
      const docs = await call(() => ListRitualDocuments(profileId, boardId, sprintId));
      // A sprint's additions apply to that sprint's issues alone, so an issue
      // with no sprint is held to the board's items and nothing else.
      const items = effectiveAgreement(bodyOf(docs, 0), sprintId ? bodyOf(docs, sprintId) : "");
      const ticks = await call(() => DoneAgreementTicks(profileId, boardId, [issueKey]));
      return agreementRows(items, ticks[issueKey] ?? []);
    },
    enabled: !!profileId && boardId > 0 && !!issueKey,
    retry: false,
  });
}

// sprintDoneAgreement is the same two reads for a whole sprint, which is
// what the report's own done agreement section counts: the sprint's
// effective items, and the ticks of every card the report was built from.
// DoneAgreementTicks takes a list of issues so this is one call.
//
// It is a plain function rather than a hook because the report reads it at
// the moment somebody publishes, the way the charts are rasterised on the
// click: what a published page carries is what the store held when it was
// written. A read that fails refuses the publish through the caller's own
// error handling, rather than writing a page with the section quietly
// missing, which at a review would read as a team that agreed nothing.
export async function sprintDoneAgreement(
  profileId: string,
  boardId: number,
  sprintId: number,
  issueKeys: string[],
): Promise<AgreementFigures> {
  const docs = await call(() => ListRitualDocuments(profileId, boardId, sprintId));
  const items = effectiveAgreement(bodyOf(docs, 0), sprintId ? bodyOf(docs, sprintId) : "");
  // An agreement stating nothing has nothing to count, and a sprint holding
  // no card has nothing to count it over. The section is absent either way,
  // so the ticks are not asked for.
  if (items.length === 0 || issueKeys.length === 0) return { items, ticked: {} };
  return { items, ticked: await call(() => DoneAgreementTicks(profileId, boardId, issueKeys)) };
}

// useSetDoneAgreementTick ticks or unticks one item for one issue. It
// invalidates nothing but this issue's agreement: a tick is not journalled
// and not pushed on Commit, so no pending count, no row and nothing in Jira
// changes because of it.
export function useSetDoneAgreementTick(profileId: string, boardId: number, issueKey: string, sprintId: number) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: ({ item, ticked }: { item: string; ticked: boolean }) =>
      call(() => SetDoneAgreementTick(profileId, boardId, issueKey, item, ticked)),
    onSuccess: () =>
      qc.invalidateQueries({ queryKey: keys.doneAgreement(profileId, issueKey, boardId, sprintId) }),
  });
}
