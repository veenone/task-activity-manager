package boardrepo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/tam/internal/dbtx"
)

// A column limit of the user's own is what fills the gap on the boards whose
// admin never set one in Jira, which is most of them. Jira's limit wins
// wherever there is one; TAM never writes board configuration, so this is a
// local planning aid and every surface that prints it says where it came
// from.
//
// The limit is keyed on the column's name, for the reasons on
// tamstore.columnLimitDDL: a boards pass replaces a board's columns
// wholesale and keys them by position, so neither of those survives what a
// user's own number has to survive.

// MaxColumnLimit is the largest limit a column may be given. A ceiling is
// here because the value is typed by a person and a mistyped one is routine:
// nothing about a column of a Jira board means twenty thousand cards, and a
// figure like that in a report reads as a bug in TAM rather than as a limit.
const MaxColumnLimit = 9999

// SetColumnLimit stores the limit a user typed for one board column, or
// clears it when they typed nothing.
//
// limit is the raw text from the editor rather than a number, because that is
// what crosses the boundary: the frontend can refuse a word before sending
// it, and a binding that took an int would have nothing left to refuse. Empty
// text clears the limit, which is the one destructive thing here and is
// undone by typing the number again.
func (r *Repository) SetColumnLimit(ctx context.Context, profileID string, boardID int, column, limit string) error {
	if strings.TrimSpace(profileID) == "" {
		return errors.New("no profile selected")
	}
	name := strings.TrimSpace(column)
	if name == "" {
		return errors.New("no column named")
	}
	if boardID <= 0 {
		return fmt.Errorf("board %d is not a board", boardID)
	}
	max, err := parseColumnLimit(limit)
	if err != nil {
		return err
	}
	if max == nil {
		_, err := r.db.ExecContext(ctx,
			`DELETE FROM board_column_limit WHERE profile_id = ? AND board_id = ? AND column_name = ?`,
			profileID, boardID, name)
		if err != nil {
			return fmt.Errorf("clear the limit on %q: %w", name, err)
		}
		return nil
	}
	_, err = r.db.ExecContext(ctx,
		`INSERT INTO board_column_limit (profile_id, board_id, column_name, wip_max) VALUES (?, ?, ?, ?)
			ON CONFLICT(profile_id, board_id, column_name) DO UPDATE SET wip_max = excluded.wip_max`,
		profileID, boardID, name, *max)
	if err != nil {
		return fmt.Errorf("store the limit on %q: %w", name, err)
	}
	return nil
}

// parseColumnLimit reads what the user typed. Nothing means no limit; a word,
// a fraction, a negative number and a number past the ceiling are each
// refused with what is wrong rather than with a rounded or clamped value,
// since a limit quietly changed on its way to the store is a limit the user
// will read back as somebody else's.
func parseColumnLimit(limit string) (*int, error) {
	text := strings.TrimSpace(limit)
	if text == "" {
		return nil, nil
	}
	max, err := strconv.Atoi(text)
	if err != nil {
		return nil, fmt.Errorf("a column limit is a whole number of cards, and %q is not one", text)
	}
	if max < 0 {
		return nil, fmt.Errorf("a column limit cannot be negative, and %d is", max)
	}
	if max > MaxColumnLimit {
		return nil, fmt.Errorf("a column limit of %d is past the %d this allows", max, MaxColumnLimit)
	}
	return &max, nil
}

// columnLimitsOf is one board's limits by column name, read on the querier
// the rest of the board is being read on so the limits and the columns they
// belong to are the same snapshot.
func columnLimitsOf(ctx context.Context, q dbtx.Querier, profileID string, boardID int) (map[string]int, error) {
	rows, err := q.QueryContext(ctx,
		`SELECT column_name, wip_max FROM board_column_limit WHERE profile_id = ? AND board_id = ?`,
		profileID, boardID)
	if err != nil {
		return nil, fmt.Errorf("read the column limits of board %d: %w", boardID, err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var (
			name string
			max  int
		)
		if err := rows.Scan(&name, &max); err != nil {
			return nil, err
		}
		out[name] = max
	}
	return out, rows.Err()
}

// ownLimit is the pointer a column head carries: the stored limit when the
// column has one, nil otherwise. It is a pointer for the reason Min and Max
// are, that zero is a limit somebody can really set.
func ownLimit(limits map[string]int, name string) *int {
	max, ok := limits[name]
	if !ok {
		return nil
	}
	return &max
}
