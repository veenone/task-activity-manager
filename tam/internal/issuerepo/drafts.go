package issuerepo

import (
	"context"
	"fmt"
	"strings"
)

// DraftIndex maps every draft of the profile to its temporary key, keyed by
// strings.ToLower(type + "|" + summary), so a second pass over an import
// file can tell a row that already became a draft from a genuinely new one.
func (r *Repository) DraftIndex(ctx context.Context, profileID string) (map[string]string, error) {
	rows, err := r.db.QueryContext(ctx,
		`SELECT key, type, summary FROM issue WHERE profile_id = ? AND key LIKE ?`,
		profileID, DraftPrefix+"%")
	if err != nil {
		return nil, fmt.Errorf("draft index: %w", err)
	}
	defer rows.Close()
	index := map[string]string{}
	for rows.Next() {
		var key, typ, summary string
		if err := rows.Scan(&key, &typ, &summary); err != nil {
			return nil, err
		}
		index[strings.ToLower(typ+"|"+summary)] = key
	}
	return index, rows.Err()
}
