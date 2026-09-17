package sprints_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/sprints"
)

// TestAGuardRefusalAtPushTimeIsMarkedRefused is what lets Commit tell a push
// the sprint's own state refused, which no retry fixes, from one that failed
// on the way to Jira.
func TestAGuardRefusalAtPushTimeIsMarkedRefused(t *testing.T) {
	ctx := context.Background()
	closed := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "closed"}}}
	store := newStore()
	if _, err := sprints.ForCommit(manageService(closed, store, newIssues(store))).Edit(ctx, "p1", 1, 13, draft("Sprint 13", ""), false); !errors.Is(err, sprints.ErrRefused) {
		t.Errorf("editing a closed sprint = %v, want ErrRefused", err)
	}

	active := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "active"}}}
	if _, err := sprints.ForCommit(manageService(active, store, newIssues(store))).Delete(ctx, "p1", 1, 13); !errors.Is(err, sprints.ErrRefused) {
		t.Errorf("deleting an active sprint = %v, want ErrRefused", err)
	}

	future := &fakeBackend{sprints: []backend.Sprint{{ID: 13, BoardID: 1, Name: "Sprint 13", State: "future"}}}
	s := manageService(future, store, newIssues(store))
	s.Pending = func(context.Context, string, int) (int, error) { return 1, nil }
	if _, err := sprints.ForCommit(s).Delete(ctx, "p1", 1, 13); !errors.Is(err, sprints.ErrRefused) {
		t.Errorf("deleting a sprint with pending moves = %v, want ErrRefused", err)
	}
	if err := sprints.RefusePendingDelete(0); err != nil {
		t.Errorf("nothing pending = %v", err)
	}

	failing := &fakeBackend{sprintErr: errors.New("502 Bad Gateway")}
	if _, err := sprints.ForCommit(manageService(failing, store, newIssues(store))).Edit(ctx, "p1", 1, 13, draft("Sprint 13", ""), false); err == nil || errors.Is(err, sprints.ErrRefused) {
		t.Errorf("a failed read = %v, want an error that is not a refusal", err)
	}
}

// TestAStoppedCompletionNamesTheCardsItMoved is what lets Commit's failure
// say which cards already left an open sprint.
func TestAStoppedCompletionNamesTheCardsItMoved(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1", "5", "1"), completeErr: errors.New("403 Forbidden")}
	done, err := sprints.ForCommit(newService(b, newStore())).Complete(context.Background(), "p1", 1, 12, "")
	if err != nil || !strings.Contains(done.Message, "(PLAT-1, PLAT-3)") {
		t.Errorf("completion = %+v, %v; want the moved keys named", done, err)
	}
}

// TestACompletionOfASprintTheCacheHoldsAsClosedMovesNothing is the retry of
// a completion whose close landed and whose journal row did not clear.
func TestACompletionOfASprintTheCacheHoldsAsClosedMovesNothing(t *testing.T) {
	b := &fakeBackend{issues: sprintOf("1", "1")}
	store := newStore()
	store.onBoard["1/12"] = "closed"
	done, err := sprints.ForCommit(newService(b, store)).Complete(context.Background(), "p1", 1, 12, "")
	if err != nil || done.Message != "" || done.Moved != 0 || !strings.Contains(done.Note, "already closed") {
		t.Errorf("completion = %+v, %v; want a success that moved nothing", done, err)
	}
	if len(b.order) != 0 {
		t.Errorf("Jira was asked %v", b.order)
	}
}

// TestACompletionRefusalIsMarkedRefused keeps a future sprint's completion
// from being offered as something a retry fixes.
func TestACompletionRefusalIsMarkedRefused(t *testing.T) {
	store := newStore()
	store.onBoard["1/12"] = "future"
	if _, err := sprints.ForCommit(newService(&fakeBackend{}, store)).Complete(context.Background(), "p1", 1, 12, ""); !errors.Is(err, sprints.ErrRefused) {
		t.Errorf("completing a future sprint = %v, want ErrRefused", err)
	}
	if err := sprints.RefusePendingComplete(2); !errors.Is(err, sprints.ErrRefused) || !strings.Contains(err.Error(), "commit them before completing") {
		t.Errorf("pending = %v", err)
	}
}
