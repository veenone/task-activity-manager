package backend

import "strings"

// IsDone is the status bucket every part of TAM that counts progress calls
// done: the Backlog grid's chip, the Epics tree's progress counts, the Show
// done filter, and the board's done points. It lives here, on the package
// both issuerepo and boardrepo already depend on, because two copies of a
// three-name list is exactly how two views come to disagree about one
// issue. The frontend's statusClass mirrors it.
func IsDone(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "done", "closed", "resolved":
		return true
	}
	return false
}
