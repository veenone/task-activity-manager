package committer

import (
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// TestMoveLabelReadsAnIssueBoardAddInWords is test 6 of task 3's brief: the
// Pending changes label has to name the board and the scope rather than
// print the raw "5|backlog" the journal holds.
func TestMoveLabelReadsAnIssueBoardAddInWords(t *testing.T) {
	backlog := issuerepo.MoveValue("5", backend.BoardScopeBacklog)
	if got := moveLabel(issuerepo.EntityIssueBoard, backlog); got != "board 5, Backlog" {
		t.Errorf("label = %q, want the board named and the backlog spelled out", got)
	}
	sprint := issuerepo.MoveValue("7", "13")
	if got := moveLabel(issuerepo.EntityIssueBoard, sprint); got != "board 7, sprint 13" {
		t.Errorf("label = %q, want the board and the sprint scope named", got)
	}
}
