package testrepo

import (
	"fmt"
	"slices"
	"strings"
)

// AllocateResult reports a bulk allocation's outcome: which Tests were newly
// added to the Container and which were already members (FR-3.4–3.6).
type AllocateResult struct {
	Added          []string `json:"added"`
	AlreadyMembers []string `json:"alreadyMembers"`
}

// AllocateTests adds Tests to an existing Container locally and queues the
// membership for commit (FR-3.4–3.6). Tests already in the Container are
// reported as such. The pending add / remove sets are kept disjoint: adding a
// Test that's pending removal just cancels the removal.
func (r *Repository) AllocateTests(profileID, containerKey string, testKeys []string) (AllocateResult, error) {
	added, already, err := r.changeMembership(profileID, containerKey, testKeys, true)
	return AllocateResult{Added: added, AlreadyMembers: already}, err
}

// DeallocateResult reports a bulk de-allocation's outcome: which Tests were
// removed from the Container and which weren't members to begin with.
type DeallocateResult struct {
	Removed    []string `json:"removed"`
	NotMembers []string `json:"notMembers"`
}

// DeallocateTests removes Tests from a Container locally and queues the removal
// for commit (FR-3.4–3.6). Removing a Test that was only locally added cancels
// the add instead of queuing a remote removal; the add / remove sets stay
// disjoint.
func (r *Repository) DeallocateTests(profileID, containerKey string, testKeys []string) (DeallocateResult, error) {
	removed, notMembers, err := r.changeMembership(profileID, containerKey, testKeys, false)
	return DeallocateResult{Removed: removed, NotMembers: notMembers}, err
}

// changeMembership adds the Tests to a Container (add=true) or removes them,
// and returns the Tests it changed and the ones already in the wanted state.
func (r *Repository) changeMembership(profileID, containerKey string, testKeys []string, add bool) (changed, skipped []string, err error) {
	changed, skipped = []string{}, []string{}

	tx, err := r.db.Begin()
	if err != nil {
		return changed, skipped, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	kind, err := containerKindTx(tx, profileID, containerKey)
	if err != nil {
		return changed, skipped, err
	}
	adds, err := readMembers(tx, profileID, entityMembershipAdd, containerKey)
	if err != nil {
		return changed, skipped, err
	}
	removes, err := readMembers(tx, profileID, entityMembershipRemove, containerKey)
	if err != nil {
		return changed, skipped, err
	}

	// queued is the pending set this change adds to. A Test already in the
	// opposite set is dropped from it instead: re-adding a Test that was
	// pending removal cancels the removal, and removing one that was only
	// locally added cancels the add.
	queued, opposite := &removes, &adds
	stmt, verb, entityType, action := `DELETE FROM test_container_test
		 WHERE profile_id = ? AND container_key = ? AND test_key = ?`,
		"delete", entityMembershipRemove, "deallocate-local"
	if add {
		queued, opposite = &adds, &removes
		stmt, verb, entityType, action = `INSERT INTO test_container_test (profile_id, container_key, test_key, run_status)
		 VALUES (?, ?, ?, '')`,
			"insert", entityMembershipAdd, "allocate-local"
	}

	for _, testKey := range testKeys {
		linked, err := isMember(tx, profileID, containerKey, testKey)
		if err != nil {
			return changed, skipped, err
		}
		if linked == add {
			skipped = append(skipped, testKey)
			continue
		}
		if _, err := tx.Exec(stmt, profileID, containerKey, testKey); err != nil {
			return changed, skipped, fmt.Errorf("%s membership: %w", verb, err)
		}
		if slices.Contains(*opposite, testKey) {
			*opposite = removeString(*opposite, testKey)
		} else if !slices.Contains(*queued, testKey) {
			*queued = append(*queued, testKey)
		}
		changed = append(changed, testKey)
	}

	if len(changed) == 0 {
		return changed, skipped, nil
	}
	if err := writeMembers(tx, profileID, entityMembershipAdd, containerKey, kind, adds); err != nil {
		return changed, skipped, err
	}
	if err := writeMembers(tx, profileID, entityMembershipRemove, containerKey, kind, removes); err != nil {
		return changed, skipped, err
	}
	before, after := strings.Join(changed, " "), ""
	if add {
		before, after = after, before
	}
	if err := writeAudit(tx, profileID, entityType, containerKey, action, "members", before, after, ""); err != nil {
		return changed, skipped, err
	}
	if err := tx.Commit(); err != nil {
		return changed, skipped, fmt.Errorf("commit membership change: %w", err)
	}
	return changed, skipped, nil
}
