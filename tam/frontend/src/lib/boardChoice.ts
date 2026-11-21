import { useViewState } from "@agile-suite/core";

// useBoardChoice is the board the Boards, Sprints and Reports views share.
// Each used to keep its own picker, so "Open Boards" from a report landed on
// whatever board Boards last showed rather than the one being reported on
// (#524). It is view state, so it lasts until the profile is switched.
export function useBoardChoice(profileId: string) {
  return useViewState(profileId, "board", "id", 0);
}
