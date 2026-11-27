package committer

import (
	"encoding/json"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// Progress is one frame of a Commit, for the pending changes dialog's bar.
// Done and Total count journal rows, the unit the dialog's title counts, so
// "40 of 100" reads against the "Pending changes (100)" above it. Stage is
// the phase running, named as phases() names it, and Key the issue being
// pushed when the phase goes one issue at a time (edits and the three create
// phases); the other phases send one frame as they start and the next frame
// accounts for their rows. A held or failed row counts as done: Done says how
// far the Commit has got, not how much of it landed. Finished marks the last
// frame, which every Commit sends, an early failure included, so the bar
// always closes.
type Progress struct {
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Stage    string `json:"stage"`
	Key      string `json:"key"`
	Finished bool   `json:"finished"`
}

// progress is one Commit's running count. owned is how many of the rows the
// Commit started with each phase handles, which is what lets a phase that
// sends no per-issue frames still move the bar by exactly its share.
type progress struct {
	emit  func(Progress)
	total int
	owned map[string]int
	done  int
	// end is where the running phase's share of the bar stops; carry is the
	// rows of the issue named in the last frame, counted once the next frame
	// goes, so a frame says what is being pushed rather than what just was.
	end   int
	carry int
	stage string
}

// count sets the total from the journal as the Commit found it.
func (p *progress) count(rows []journal.PendingChange) {
	p.total = len(rows)
	p.owned = map[string]int{}
	for _, r := range rows {
		p.owned[ownerPhase(r)]++
	}
}

// phase starts the named phase: whatever the last one did not account for
// one issue at a time is done now.
func (p *progress) phase(name string) {
	p.done, p.carry = p.end, 0
	p.end = p.done + p.owned[name]
	p.stage = name
	p.send("", false)
}

// step names the issue about to be pushed, rows being how many journal rows
// it carries.
func (p *progress) step(key string, rows int) {
	p.done = min(p.done+p.carry, p.end)
	p.carry = rows
	p.send(key, false)
}

// finish sends the closing frame.
func (p *progress) finish() {
	p.done = p.total
	p.send("", true)
}

func (p *progress) send(key string, finished bool) {
	if p.emit == nil {
		return
	}
	p.emit(Progress{Done: p.done, Total: p.total, Stage: p.stage, Key: key, Finished: finished})
}

// ownerPhase names the phase that handles the row, by the names phases()
// gives them. A row of a type no phase takes is in none, and only the
// closing frame accounts for it.
func ownerPhase(r journal.PendingChange) string {
	switch r.EntityType {
	case issuerepo.EntityBoardCreate:
		return "boards"
	case issuerepo.EntityIssueBoard:
		return "board adds"
	case issuerepo.EntitySprintCreate:
		return "sprints"
	case issuerepo.EntitySprintEdit, issuerepo.EntitySprintDelete, issuerepo.EntitySprintStart, issuerepo.EntitySprintComplete:
		return "sprint changes"
	case issuerepo.EntityVersionCreate, issuerepo.EntityVersionEdit:
		return "releases"
	case issuerepo.EntityVersionRelease, issuerepo.EntityVersionArchive, issuerepo.EntityVersionDelete:
		return "closing"
	case issuerepo.EntityIssueCreate:
		// createDrafts reports a draft that will not decode with the
		// ordinary issues, so it is counted there too.
		var d backend.IssueDraft
		if json.Unmarshal([]byte(r.AfterVal), &d) != nil {
			return "issues"
		}
		return [...]string{levelEpic: "epics", levelIssue: "issues", levelSubtask: "sub-tasks"}[levelOf(d.Type)]
	case issuerepo.EntityIssue:
		return "edits"
	case issuerepo.EntityTransition, issuerepo.EntityRank, issuerepo.EntitySprintMove:
		return "board moves"
	case issuerepo.EntityLink:
		return "links"
	case issuerepo.EntityAttachment:
		return "attachments"
	case issuerepo.EntityWorklog:
		return "worklogs"
	}
	return ""
}
