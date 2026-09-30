package boardrepo

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// The ticks against one issue's done agreement. They are local: not
// journalled, not pushed on Commit, and so the one write in TAM that never
// reaches Jira. The reason is on tamstore.doneTickDDL, beside the table.
//
// A tick records the wording it was made against, which is why nothing here
// ever rewrites or drops a row because the agreement has moved on. The point
// of the feature is a record a team can be shown at a sprint review, and a
// tick that followed the document's current wording would be a record that
// changed underneath the review. Deciding which stored wording the agreement
// still states is the caller's, and the caller does it by comparing this
// answer against the items it read out of the document; see
// tam/frontend/src/lib/doneAgreement.ts, which is where those items are
// parsed. Nothing here knows what an item is beyond being the words a row is
// keyed by.

// MaxTickIssues is the most issues one read may ask about. The panel asks
// about one and a sprint report asks about a sprint's own, so a list past
// this is a caller with the wrong set of keys in hand rather than a sprint.
const MaxTickIssues = 500

// DoneAgreementTicks answers the ticks of each issue named, keyed by issue
// key: the wording of every item ticked against that issue, whether or not
// the agreement still states it. An issue nobody has ticked anything on is
// absent rather than empty.
//
// It takes a list of issues rather than one so the sprint report's section can
// ask for a whole sprint in one call, which is the read the panel makes for
// one issue.
func (r *Repository) DoneAgreementTicks(ctx context.Context, profileID string, boardID int, issueKeys []string) (map[string][]string, error) {
	if err := tickBoard(profileID, boardID); err != nil {
		return nil, err
	}
	if len(issueKeys) > MaxTickIssues {
		return nil, fmt.Errorf("a read of %d issues is past the %d this answers for", len(issueKeys), MaxTickIssues)
	}
	keys := make([]string, 0, len(issueKeys))
	for _, key := range issueKeys {
		if k := strings.TrimSpace(key); k != "" {
			keys = append(keys, k)
		}
	}
	out := map[string][]string{}
	if len(keys) == 0 {
		return out, nil
	}
	args := []any{profileID, boardID}
	for _, key := range keys {
		args = append(args, key)
	}
	holders := strings.TrimSuffix(strings.Repeat("?,", len(keys)), ",")
	rows, err := r.db.QueryContext(ctx,
		`SELECT issue_key, item_text FROM done_agreement_tick
		 WHERE profile_id = ? AND board_id = ? AND issue_key IN (`+holders+`)
		 ORDER BY issue_key, item_text`,
		args...)
	if err != nil {
		return nil, fmt.Errorf("read the done agreement ticks of board %d: %w", boardID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var key, item string
		if err := rows.Scan(&key, &item); err != nil {
			return nil, err
		}
		out[key] = append(out[key], item)
	}
	return out, rows.Err()
}

// SetDoneAgreementTick ticks or unticks one item for one issue. item is the
// wording the tick is made against and the row is keyed by it, so ticking the
// same item twice is the same tick. Unticking what is not ticked is the state
// the caller asked for rather than an error: a second click on a box the last
// one already cleared.
func (r *Repository) SetDoneAgreementTick(ctx context.Context, profileID string, boardID int, issueKey, item string, ticked bool) error {
	if err := tickBoard(profileID, boardID); err != nil {
		return err
	}
	key := strings.TrimSpace(issueKey)
	if key == "" {
		return errors.New("no issue named")
	}
	text := strings.TrimSpace(item)
	if text == "" {
		return errors.New("no agreement item named")
	}
	if !ticked {
		if _, err := r.db.ExecContext(ctx,
			`DELETE FROM done_agreement_tick WHERE profile_id = ? AND board_id = ? AND issue_key = ? AND item_text = ?`,
			profileID, boardID, key, text); err != nil {
			return fmt.Errorf("untick %q on %s: %w", text, key, err)
		}
		return nil
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO done_agreement_tick (profile_id, board_id, issue_key, item_text) VALUES (?, ?, ?, ?)
			ON CONFLICT(profile_id, board_id, issue_key, item_text) DO NOTHING`,
		profileID, boardID, key, text); err != nil {
		return fmt.Errorf("tick %q on %s: %w", text, key, err)
	}
	return nil
}

// tickBoard checks what a tick is filed under. The item's text, the issue's
// key and the board both arrive across the Wails boundary, so they are input
// (I1): a board id of zero is the board's standing agreement document rather
// than a board, and a row with no profile belongs to nobody's purge.
func tickBoard(profileID string, boardID int) error {
	if strings.TrimSpace(profileID) == "" {
		return errors.New("no profile selected")
	}
	if boardID <= 0 {
		return fmt.Errorf("board %d is not a board", boardID)
	}
	return nil
}
