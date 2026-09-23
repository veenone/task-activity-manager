package issuerepo

import (
	"context"
	"encoding/json"
	"fmt"

	"agile-suite/tam/internal/backend"
)

// settingProjectTypes is where the sync leaves the issue types the profile's
// project actually offers, each under the project's own name for it and
// carrying the logical type TAM maps it onto.
//
// It is a profile setting rather than a table of its own. profile_setting is
// already profile-keyed, already swept by both PurgeProfile implementations,
// and already the home of the other per-profile facts a sync discovers (the
// connected username, whether the instance serves boards at all). One list
// per profile, read whole and written whole, needs nothing a table would
// give it, and the schema contract's purge lists already cover it, so this
// adds no migration and no new name to keep in step.
//
// The dialog reads it and nothing else: a local-first app must not send the
// user to Jira when they press New, and the instance behind issue #65
// answers the per-type create-metadata endpoint with an error anyway
// (issue #51).
const settingProjectTypes = "project_types"

// PutProjectTypes records the project's issue types for the profile. An
// empty list is not written: nothing known and a project with no types are
// different things, and the caller falls back to TAM's own list for the
// first.
func (r *Repository) PutProjectTypes(ctx context.Context, profileID string, types []backend.IssueType) error {
	if len(types) == 0 {
		return nil
	}
	encoded, err := json.Marshal(types)
	if err != nil {
		return fmt.Errorf("encode the issue types of %s: %w", profileID, err)
	}
	return r.SetProfileSetting(ctx, profileID, settingProjectTypes, string(encoded))
}

// ProjectTypes reads back what PutProjectTypes stored, empty when no sync
// has recorded any. A stored value nothing can parse is read as none, so a
// bad row costs the caller its fallback and never the dialog.
func (r *Repository) ProjectTypes(ctx context.Context, profileID string) ([]backend.IssueType, error) {
	raw, err := r.ProfileSetting(ctx, profileID, settingProjectTypes)
	if err != nil {
		return nil, err
	}
	out := []backend.IssueType{}
	if raw == "" {
		return out, nil
	}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		return []backend.IssueType{}, nil
	}
	return out, nil
}
