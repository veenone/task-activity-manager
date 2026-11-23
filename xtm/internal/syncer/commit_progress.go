package syncer

import "agile-suite/xtm/internal/testrepo"

// commitProgressPhase tags commit events so a listener can tell them from a
// sync's.
const commitProgressPhase = "commit"

// commitTracker counts the pending rows one commit run has dealt with. A row
// counts once, whichever pass reaches it first: cleared after a push, or
// passed over by its pass because it failed, conflicted or was skipped. Rows
// outside the set the run started with are ignored, so a re-read after a
// create pass cannot push the count past the total.
type commitTracker struct {
	emit  func(Progress)
	keys  map[int64]string
	seen  map[int64]bool
	open  []testrepo.PendingChange
	stage string
}

func newCommitTracker(emit func(Progress), rows []testrepo.PendingChange) *commitTracker {
	t := &commitTracker{emit: emit, keys: make(map[int64]string, len(rows)), seen: map[int64]bool{}}
	for _, c := range rows {
		t.keys[c.ID] = c.EntityKey
	}
	return t
}

func (t *commitTracker) send() {
	if t == nil || t.emit == nil {
		return
	}
	t.emit(Progress{Phase: commitProgressPhase, Fetched: len(t.seen), Total: len(t.keys), Stage: t.stage})
}

// mark counts the given ids and reports when any of them was new.
func (t *commitTracker) mark(ids []int64) {
	if t == nil {
		return
	}
	fresh := false
	for _, id := range ids {
		if key, ok := t.keys[id]; ok && !t.seen[id] {
			t.seen[id] = true
			t.stage = key
			fresh = true
		}
	}
	if fresh {
		t.send()
	}
}

// begin closes the rows the previous begin opened, which their pass has now
// either pushed or reported, and announces the next group by label.
func (t *commitTracker) begin(label string, rows []testrepo.PendingChange) {
	if t == nil {
		return
	}
	t.mark(rowIDs(t.open))
	t.open = rows
	if len(rows) > 0 {
		t.stage = label
		t.send()
	}
}

// beginPass is begin for the buckets one pass works through, labelled by
// their first row.
func (t *commitTracker) beginPass(buckets ...[]testrepo.PendingChange) {
	var rows []testrepo.PendingChange
	for _, b := range buckets {
		rows = append(rows, b...)
	}
	label := ""
	if len(rows) > 0 {
		label = rows[0].EntityKey
	}
	t.begin(label, rows)
}

// finish counts every row the run started with, so the bar ends full: by the
// time the run returns, each row has been pushed or reported.
func (t *commitTracker) finish() {
	if t == nil {
		return
	}
	all := make([]int64, 0, len(t.keys))
	for id := range t.keys {
		all = append(all, id)
	}
	t.mark(all)
}

func rowIDs(rows []testrepo.PendingChange) []int64 {
	ids := make([]int64, 0, len(rows))
	for _, c := range rows {
		ids = append(ids, c.ID)
	}
	return ids
}

// clearPending deletes pushed rows from the journal and counts them toward
// the commit's progress. Every pass clears through here rather than calling
// the repository directly, so the count moves row by row.
func (e *Engine) clearPending(profileID string, ids []int64) error {
	if err := e.repo.CommitPendingChanges(profileID, ids); err != nil {
		return err
	}
	e.commitRun.mark(ids)
	return nil
}
