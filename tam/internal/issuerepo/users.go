package issuerepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
)

// userSearchLimit caps a cached lookup, matching what the Jira search itself
// returns so the two answer with a comparable amount.
const userSearchLimit = 30

// CacheUsers records the people a search found, so the next keystroke is
// answered from disk and the picker still works with Jira unreachable. It is
// additive: a user Jira no longer returns keeps its row until a full sync of
// the profile purges it, because the alternative is a picker that empties
// itself whenever a search happens to be narrow.
func (r *Repository) CacheUsers(ctx context.Context, profileID string, users []backend.User) error {
	if len(users) == 0 {
		return nil
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	now := time.Now().UTC().Format(time.RFC3339)
	for _, u := range users {
		if strings.TrimSpace(u.Name) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO jira_user (profile_id, name, display_name, cached_at) VALUES (?, ?, ?, ?)
			 ON CONFLICT(profile_id, name) DO UPDATE SET display_name = excluded.display_name, cached_at = excluded.cached_at`,
			profileID, u.Name, u.DisplayName, now); err != nil {
			return fmt.Errorf("cache user %s: %w", u.Name, err)
		}
	}
	return tx.Commit()
}

// SearchCachedUsers matches query against a cached username or display name.
// A blank query returns the first page by display name, which is what an
// empty picker shows before anything is typed.
func (r *Repository) SearchCachedUsers(ctx context.Context, profileID, query string) ([]backend.User, error) {
	q := strings.TrimSpace(query)
	// LIKE's own wildcards in a user's query would otherwise silently change
	// what it matches, so they are escaped and the pattern is built here.
	esc := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(q)
	rows, err := r.db.QueryContext(ctx,
		`SELECT name, display_name FROM jira_user
		 WHERE profile_id = ?
		   AND (? = '' OR name LIKE ? ESCAPE '\' OR display_name LIKE ? ESCAPE '\')
		 ORDER BY display_name, name LIMIT ?`,
		profileID, q, "%"+esc+"%", "%"+esc+"%", userSearchLimit)
	if err != nil {
		return nil, fmt.Errorf("search cached users: %w", err)
	}
	defer rows.Close()
	out := []backend.User{}
	for rows.Next() {
		var u backend.User
		if err := rows.Scan(&u.Name, &u.DisplayName); err != nil {
			return nil, err
		}
		out = append(out, u)
	}
	return out, rows.Err()
}

// CountCachedUsers is how the app decides whether a profile's picker has
// anything to fall back on yet.
func (r *Repository) CountCachedUsers(ctx context.Context, profileID string) (int, error) {
	var n int
	err := r.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM jira_user WHERE profile_id = ?`, profileID).Scan(&n)
	return n, err
}
