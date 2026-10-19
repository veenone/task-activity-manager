package boardrepo_test

import (
	"context"
	"fmt"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

// TestBoardIssuesIsEveryCardOnTheBoardUncapped feeds the board allocation
// (#288): every card the board holds, in its sprints and its backlog, with
// no view budget, since an allocation short of the cards past the cap would
// understate whoever holds them.
func TestBoardIssuesIsEveryCardOnTheBoardUncapped(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	board := backend.Board{ID: 1, Name: "PLAT Scrum", Type: backend.BoardTypeScrum}
	sprint := backend.Sprint{ID: 12, BoardID: 1, Name: "Sprint 12", State: "active", StartDate: "2026-08-18T09:00:00Z", EndDate: "2026-09-01T09:00:00Z"}
	var cards []backend.Issue
	keys := map[string][]string{}
	for i := 0; i < boardrepo.MaxCardsPerView+5; i++ {
		key := fmt.Sprintf("PLAT-%d", i+1)
		cards = append(cards, card(key, "To Do", "1"))
		where := "12"
		if i%2 == 1 {
			where = ""
		}
		keys[where] = append(keys[where], key)
	}
	if err := r.ReplaceBoard(ctx, "p1", board, sampleColumns(), []backend.Sprint{sprint}, keys); err != nil {
		t.Fatalf("seed: %v", err)
	}
	got, err := r.BoardIssues(ctx, newIssues(cards...), "p1", 1)
	if err != nil {
		t.Fatalf("board issues: %v", err)
	}
	if len(got) != len(cards) {
		t.Errorf("got %d cards, want all %d, sprint and backlog together", len(got), len(cards))
	}
}
