package testrepo

import (
	"fmt"
	"time"
)

// SavedView is a named, reusable browse filter (FR-11.4). Query is an opaque
// JSON blob the frontend owns — the store doesn't interpret it.
type SavedView struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Query     string `json:"query"`
	CreatedAt string `json:"createdAt"`
}

// CreateSavedView stores a named browse filter for a profile and returns it.
// The id is time-based so it's unique without a separate sequence.
func (r *Repository) CreateSavedView(profileID, name, query string) (SavedView, error) {
	if name == "" {
		return SavedView{}, fmt.Errorf("a name is required")
	}
	now := time.Now().UTC()
	view := SavedView{
		ID:        fmt.Sprintf("view-%d", now.UnixNano()),
		Name:      name,
		Query:     query,
		CreatedAt: now.Format(time.RFC3339),
	}
	if _, err := r.db.Exec(
		`INSERT INTO saved_view (profile_id, id, name, query, created_at)
		 VALUES (?, ?, ?, ?, ?)`,
		profileID, view.ID, view.Name, view.Query, view.CreatedAt,
	); err != nil {
		return SavedView{}, fmt.Errorf("create saved view: %w", err)
	}
	return view, nil
}

// ListSavedViews returns a profile's saved browse filters, newest first.
func (r *Repository) ListSavedViews(profileID string) ([]SavedView, error) {
	rows, err := r.db.Query(
		`SELECT id, name, query, created_at FROM saved_view
		 WHERE profile_id = ? ORDER BY created_at DESC, id DESC`,
		profileID)
	if err != nil {
		return nil, fmt.Errorf("list saved views: %w", err)
	}
	defer rows.Close()

	out := []SavedView{}
	for rows.Next() {
		var v SavedView
		if err := rows.Scan(&v.ID, &v.Name, &v.Query, &v.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// DeleteSavedView removes a saved browse filter.
func (r *Repository) DeleteSavedView(profileID, id string) error {
	if _, err := r.db.Exec(
		`DELETE FROM saved_view WHERE profile_id = ? AND id = ?`,
		profileID, id,
	); err != nil {
		return fmt.Errorf("delete saved view: %w", err)
	}
	return nil
}
