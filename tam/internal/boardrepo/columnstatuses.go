package boardrepo

import (
	"context"
	"encoding/json"
	"fmt"
)

// ColumnStatuses is every status id that shares a board column with
// statusID, that id first, across every board of the profile.
//
// A Jira column collects statuses, not one status: a "Done" column commonly
// holds Resolved and Closed, and an instance that has migrated a workflow
// holds the old status beside the new one. A card dropped on such a column
// is asking for the column, not for whichever of its statuses happens to be
// listed first, and only one of them is usually reachable from where that
// card is now. Handing the commit the whole set is what lets it pick the
// one the issue's own workflow offers.
//
// The order is the column's own, with the journaled target promoted to the
// front, so a target that is reachable is still preferred over its
// siblings and the choice is stable between runs.
func (o Order) ColumnStatuses(ctx context.Context, profileID, statusID string) ([]string, error) {
	if statusID == "" {
		return nil, nil
	}
	rows, err := o.Boards.db.QueryContext(ctx,
		`SELECT status_ids FROM board_column WHERE profile_id = ?`, profileID)
	if err != nil {
		return nil, fmt.Errorf("read the board columns of %s: %w", profileID, err)
	}
	defer rows.Close()

	out := []string{statusID}
	seen := map[string]bool{statusID: true}
	for rows.Next() {
		var raw string
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var ids []string
		if err := json.Unmarshal([]byte(raw), &ids); err != nil {
			// A column whose ids will not parse is not worth failing a
			// commit over; it simply offers no siblings.
			continue
		}
		holds := false
		for _, id := range ids {
			if id == statusID {
				holds = true
				break
			}
		}
		if !holds {
			continue
		}
		for _, id := range ids {
			if id == "" || seen[id] {
				continue
			}
			seen[id] = true
			out = append(out, id)
		}
	}
	return out, rows.Err()
}
