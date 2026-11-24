package testrepo

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
)

// CreateAttempt is what a pending create row keeps, under "createAttempt" in
// its payload, when a create was sent and Jira never answered (#607). The
// issue may exist, so the next commit searches for it before sending the
// create again. At is RFC 3339, taken before the request went out.
type CreateAttempt struct {
	At        string `json:"at"`
	Project   string `json:"project"`
	Kind      string `json:"kind"`
	IssueType string `json:"issueType,omitempty"`
	Summary   string `json:"summary"`
}

// MarkCreateAttempt records attempt on a pending create row, leaving every
// other key of its payload as it was. A later attempt replaces an earlier
// one: the commit that sends a second create has already searched for the
// first and found nothing.
func (r *Repository) MarkCreateAttempt(profileID string, changeID int64, attempt CreateAttempt) error {
	tx, err := r.db.Begin()
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var afterVal string
	err = tx.QueryRow(
		`SELECT after_val FROM pending_change WHERE id = ? AND profile_id = ?`,
		changeID, profileID).Scan(&afterVal)
	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("pending change %d not found", changeID)
	}
	if err != nil {
		return fmt.Errorf("read pending change: %w", err)
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal([]byte(afterVal), &payload); err != nil {
		return fmt.Errorf("malformed create payload: %w", err)
	}
	if payload == nil {
		payload = map[string]json.RawMessage{}
	}
	if payload["createAttempt"], err = json.Marshal(attempt); err != nil {
		return fmt.Errorf("encode create attempt: %w", err)
	}
	next, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode create payload: %w", err)
	}
	if _, err := tx.Exec(
		`UPDATE pending_change SET after_val = ? WHERE id = ? AND profile_id = ?`,
		string(next), changeID, profileID); err != nil {
		return fmt.Errorf("record create attempt: %w", err)
	}
	return tx.Commit()
}
