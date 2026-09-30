import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { DoneAgreementTicks, ListRitualDocuments, SetDoneAgreementTick } from "../api";
import type { RitualDocument } from "../api";
import { agreementRows, effectiveAgreement } from "../lib/doneAgreement";
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
