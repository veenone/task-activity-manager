package main

// The done agreement's per-issue ticks. These two bindings are the whole of
// what crosses into the frontend for them, and they are the one pair of writes
// in TAM that never reach Jira: a tick is not journalled and is not pushed on
// Commit. Jira has nowhere to put it, and it is TAM's own record of what a
// team checked rather than a change to a Jira issue. The reason sits with the
// table, on tamstore.doneTickDDL, and the store's own rules are in
// boardrepo/doneticks.go.
//
// Neither of them knows what an item is. The items come from the done
// agreement's document body, which lib/doneAgreement.ts on the frontend parses
// out of the stored Confluence storage format, because the same parse feeds
// the sprint report's document, which is built in TypeScript.

// DoneAgreementTicks answers, per issue key, the wording of every agreement
// item ticked against that issue. The caller compares it against the items the
// agreement states today: a stored wording the document no longer carries is a
// tick made against different words, which the panel shows as that rather than
// dropping it.
//
// It takes several issues so the sprint report's section can ask for a whole
// sprint in the call the panel makes for one issue.
func (a *App) DoneAgreementTicks(profileID string, boardID int, issueKeys []string) (map[string][]string, error) {
	if err := a.requireStore(); err != nil {
		return nil, err
	}
	return a.boards.DoneAgreementTicks(a.ctx, profileID, boardID, issueKeys)
}

// SetDoneAgreementTick ticks or unticks one item for one issue. item is the
// wording ticked, and checking it is the repository's so a blank one is
// refused in one place rather than once per caller.
func (a *App) SetDoneAgreementTick(profileID string, boardID int, issueKey, item string, ticked bool) error {
	if err := a.requireStore(); err != nil {
		return err
	}
	return a.boards.SetDoneAgreementTick(a.ctx, profileID, boardID, issueKey, item, ticked)
}
