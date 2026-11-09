package cycles

import (
	"database/sql"
	"errors"
	"time"
)

// Page is the Confluence page a cycle was published to, and the version PIE
// wrote last. A different version on the page means someone edited it since.
type Page struct {
	ID      string
	Version int
}

// Page returns the page the cycle was published to, if it was.
func (s *Store) Page(profileID string, id int64) (Page, bool, error) {
	var p Page
	err := s.db.QueryRow(`SELECT page_id, version FROM report_page WHERE profile_id = ? AND cycle_id = ?`, profileID, id).
		Scan(&p.ID, &p.Version)
	if errors.Is(err, sql.ErrNoRows) {
		return Page{}, false, nil
	}
	return p, err == nil, err
}

// MarkPublished records the page the cycle was written to. The cycle keeps
// the date it was first published; publishing it again only moves the page
// record on.
func (s *Store) MarkPublished(profileID string, id int64, p Page, at time.Time) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()
	r, err := tx.Exec(`UPDATE report_cycle SET published_at = CASE WHEN published_at = '' THEN ? ELSE published_at END
		WHERE profile_id = ? AND id = ?`, at.UTC().Format(time.RFC3339), profileID, id)
	if err != nil {
		return err
	}
	if n, _ := r.RowsAffected(); n == 0 {
		return errNoCycle
	}
	if _, err := tx.Exec(`INSERT INTO report_page (profile_id, cycle_id, page_id, version) VALUES (?, ?, ?, ?)
		ON CONFLICT (profile_id, cycle_id) DO UPDATE SET page_id = excluded.page_id, version = excluded.version`,
		profileID, id, p.ID, p.Version); err != nil {
		return err
	}
	return tx.Commit()
}

// ForgetPages drops the profile's page records. The settings call it when the
// Confluence URL changes, because page ids from one instance mean nothing on
// another. The cycles stay published.
func (s *Store) ForgetPages(profileID string) error {
	_, err := s.db.Exec(`DELETE FROM report_page WHERE profile_id = ?`, profileID)
	return err
}
