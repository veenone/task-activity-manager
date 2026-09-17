package sprints_test

import (
	"context"
	"errors"
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
