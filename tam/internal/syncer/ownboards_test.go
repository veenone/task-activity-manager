package syncer

import (
	"testing"

	"agile-suite/tam/internal/backend"
)

func boardsFixture() []backend.Board {
	return []backend.Board{
		{ID: 1, Name: "Jakarta Board", Type: backend.BoardTypeScrum, ProjectKey: "RND_P_4JKTEE_05"},
		{ID: 2, Name: "Nakula Sadewa 1 & 2", Type: backend.BoardTypeKanban, ProjectKey: "RND_P_4JKTNS_05"},
		{ID: 3, Name: "Unlocated", Type: backend.BoardTypeKanban},
	}
}

// Jira's board list answers with every board whose filter mentions the
// project, so a board another team owns comes back too. One seen in the
// field held 8,485 cards while the project being synced had 38.
func TestOwnBoardsKeepsOnlyTheProjectsOwn(t *testing.T) {
	own, foreign := ownBoards(boardsFixture(), "RND_P_4JKTEE_05", false)
	if len(own) != 2 || own[0].ID != 1 || own[1].ID != 3 {
		t.Fatalf("own = %+v", own)
	}
	if len(foreign) != 1 || foreign[0] != "Nakula Sadewa 1 & 2" {
		t.Errorf("foreign = %v", foreign)
	}
}

// A board whose home the instance did not report is kept: an instance that
// sends no location must not lose every board it has.
func TestOwnBoardsKeepsABoardWithNoLocation(t *testing.T) {
	own, foreign := ownBoards([]backend.Board{{ID: 3, Name: "Unlocated"}}, "PLAT", false)
	if len(own) != 1 || len(foreign) != 0 {
		t.Errorf("own = %+v foreign = %v", own, foreign)
	}
}

// The project key is compared without case, since Jira's own casing for a
// key is not something a profile has to match.
func TestOwnBoardsIgnoresKeyCase(t *testing.T) {
	own, _ := ownBoards(boardsFixture(), "rnd_p_4jktee_05", false)
	if len(own) != 2 {
		t.Errorf("own = %+v", own)
	}
}

// A profile that genuinely works across a programme board keeps them all.
func TestOwnBoardsKeepsEverythingWhenAsked(t *testing.T) {
	own, foreign := ownBoards(boardsFixture(), "RND_P_4JKTEE_05", true)
	if len(own) != 3 || len(foreign) != 0 {
		t.Errorf("own = %+v foreign = %v", own, foreign)
	}
}
