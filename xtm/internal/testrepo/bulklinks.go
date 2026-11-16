package testrepo

// The bulk link edits: adding, removing or swapping Preconditions and
// requirement links across a batch of Tests. Both kinds run the same loop and
// differ only in how a Test's current set is read and how the new one is
// queued.

// BulkAssociatePreconditions adds (add=true) or removes (add=false) the given
// Preconditions across a batch of Tests (FR-13.6), computing each Test's new
// set and queuing it via SetTestPreconditions. A Test already in the desired
// state is reported as succeeded.
func (r *Repository) BulkAssociatePreconditions(profileID string, testKeys, precondKeys []string, add bool) (BulkEditResult, error) {
	toRemove, toAdd := linkDelta(precondKeys, add)
	return r.BulkReplacePreconditions(profileID, testKeys, toRemove, toAdd)
}

// BulkReplacePreconditions swaps Preconditions across a batch of Tests: for each
// Test it removes toRemove and adds toAdd in one apply, computing the new set as
// (current minus toRemove) plus toAdd and queuing it via SetTestPreconditions
// (FR-13.6). A Test already in the desired state is reported as succeeded.
func (r *Repository) BulkReplacePreconditions(profileID string, testKeys, toRemove, toAdd []string) (BulkEditResult, error) {
	return bulkSetLinks(testKeys, toRemove, toAdd,
		func(testKey string) ([]string, error) { return r.preconditionKeys(profileID, testKey) },
		func(testKey string, keys []string) error { return r.SetTestPreconditions(profileID, testKey, keys) },
	), nil
}

// BulkAssociateRequirements adds or removes a set of requirement links across
// many Tests at once, reporting per-Test success/failure.
func (r *Repository) BulkAssociateRequirements(profileID string, testKeys, reqKeys []string, add bool) (BulkEditResult, error) {
	toRemove, toAdd := linkDelta(reqKeys, add)
	return r.BulkReplaceRequirements(profileID, testKeys, toRemove, toAdd)
}

// BulkReplaceRequirements swaps requirement links across many Tests: for each
// Test it removes toRemove and adds toAdd in one apply, computing the new set as
// (current minus toRemove) plus toAdd and queuing it via SetTestRequirements.
// A Test already in the desired state is reported as succeeded.
func (r *Repository) BulkReplaceRequirements(profileID string, testKeys, toRemove, toAdd []string) (BulkEditResult, error) {
	return bulkSetLinks(testKeys, toRemove, toAdd,
		func(testKey string) ([]string, error) { return r.testRequirementKeys(profileID, testKey) },
		func(testKey string, keys []string) error { return r.SetTestRequirements(profileID, testKey, keys) },
	), nil
}

// linkDelta expresses a plain add or remove as the remove and add lists of a
// swap.
func linkDelta(keys []string, add bool) (toRemove, toAdd []string) {
	if add {
		return nil, keys
	}
	return keys, nil
}

// bulkSetLinks reads each Test's current links with read, applies the swap and
// queues the result with set. One Test failing does not stop the rest.
func bulkSetLinks(testKeys, toRemove, toAdd []string, read func(testKey string) ([]string, error), set func(testKey string, keys []string) error) BulkEditResult {
	result := BulkEditResult{Succeeded: []string{}, Failed: []BulkFailure{}}
	for _, testKey := range testKeys {
		current, err := read(testKey)
		if err == nil {
			err = set(testKey, applyReplaceDelta(current, toRemove, toAdd))
		}
		if err != nil {
			result.Failed = append(result.Failed, BulkFailure{TestKey: testKey, Error: err.Error()})
			continue
		}
		result.Succeeded = append(result.Succeeded, testKey)
	}
	return result
}

// applyReplaceDelta returns the sorted set produced by removing toRemove from
// current and then adding toAdd. Removal happens first so a key present in both
// lists ends up added.
func applyReplaceDelta(current, toRemove, toAdd []string) []string {
	set := make(map[string]struct{}, len(current))
	for _, k := range current {
		set[k] = struct{}{}
	}
	for _, k := range toRemove {
		delete(set, k)
	}
	for _, k := range toAdd {
		set[k] = struct{}{}
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	return uniqueSorted(out)
}
