package boardrepo_test

import (
	"context"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
)

func TestCellOrderIsTheBoardColumnByColumnWithThePendingMovesApplied(t *testing.T) {
	r, _ := newRepo(t)
	cards := []backend.Issue{
		card("PLAT-409", "To Do", "1"),
		card("PLAT-412", "In Progress", "3"),
		card("PLAT-331", "Done", "5"),
		card("PLAT-350", "To Do", "1"),
	}
	// One value per issue, the way issuerepo folds the journal: this card
	// was dragged into In Progress and dropped above PLAT-412, so the order
	// has to read where it was dropped, not where the last sync left it.
	src := seedBoard(t, r, sampleColumns(), cards).withMoves(backend.PendingMove{
		Key: "PLAT-350", StatusID: "3", StatusName: "In Progress", HasTransition: true,
		RankNeighbour: "PLAT-412", RankBefore: true, HasRank: true,
	})

	order, err := boardrepo.Order{Boards: r, Issues: src}.CellOrder(context.Background(), "p1", 1)
	if err != nil {
		t.Fatalf("cell order: %v", err)
	}
	want := "PLAT-409,PLAT-350,PLAT-412,PLAT-331"
	if strings.Join(order, ",") != want {
		t.Errorf("order = %v, want %s", order, want)
	}
}

func TestCellOrderLeavesOutCardsNoColumnCollects(t *testing.T) {
	r, _ := newRepo(t)
	cards := []backend.Issue{
		card("PLAT-409", "To Do", "1"),
		card("PLAT-777", "Blocked", "99"), // a status no column collects
	}
	src := seedBoard(t, r, sampleColumns(), cards)

	order, err := boardrepo.Order{Boards: r, Issues: src}.CellOrder(context.Background(), "p1", 1)
	if err != nil {
		t.Fatalf("cell order: %v", err)
	}
	// A card the board does not draw cannot anchor another card's rank.
	if strings.Join(order, ",") != "PLAT-409" {
		t.Errorf("order = %v", order)
	}
}

func TestCellOrderRefusesABoardTheStoreDoesNotHold(t *testing.T) {
	r, _ := newRepo(t)
	src := seedBoard(t, r, sampleColumns(), []backend.Issue{card("PLAT-409", "To Do", "1")})

	_, err := boardrepo.Order{Boards: r, Issues: src}.CellOrder(context.Background(), "p1", 9)
	if err == nil || !strings.Contains(err.Error(), "board 9") {
		t.Errorf("a board that is gone names itself: %v", err)
	}
}
