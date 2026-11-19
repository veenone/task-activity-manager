package issuerepo

import (
	"context"
	"encoding/json"
	"fmt"

	"agile-suite/tam/internal/permission"
)

// settingPermissions is where the sync leaves what the token may do, by
// project. It is a profile setting for the reason settingProjectTypes is:
// one small value per profile, read whole, and already swept by PurgeProfile.
const settingPermissions = "jira_permissions"

// PutPermissions records one project's answer beside whatever is stored for
// the profile's other projects. An empty answer is not written, so a sync
// that could not read the permissions leaves the previous answer standing.
func (r *Repository) PutPermissions(ctx context.Context, profileID, project string, perms map[string]bool) error {
	if project == "" || len(perms) == 0 {
		return nil
	}
	set, err := r.Permissions(ctx, profileID)
	if err != nil {
		return err
	}
	set[project] = perms
	encoded, err := json.Marshal(set)
	if err != nil {
		return fmt.Errorf("encode the permissions of %s: %w", profileID, err)
	}
	return r.SetProfileSetting(ctx, profileID, settingPermissions, string(encoded))
}

// Permissions reads back what PutPermissions stored, empty when no sync has
// recorded any. A stored value nothing can parse is read as none, which
// allows everything: a bad row must not disable the app.
func (r *Repository) Permissions(ctx context.Context, profileID string) (permission.Set, error) {
	raw, err := r.ProfileSetting(ctx, profileID, settingPermissions)
	if err != nil {
		return nil, err
	}
	set := permission.Set{}
	if raw == "" {
		return set, nil
	}
	if err := json.Unmarshal([]byte(raw), &set); err != nil || set == nil {
		return permission.Set{}, nil
	}
	return set, nil
}
