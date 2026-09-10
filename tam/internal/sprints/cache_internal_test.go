package sprints

import (
	"context"
	"errors"
	"testing"

	"agile-suite/tam/internal/backend"
)

// The one line that makes refreshSprintsAllowEmpty exist is the refusal it
// does not have, and its sibling's whole reason for being is the refusal it
// does. Neither is reachable from the external sprints_test package, so this
// file is internal and carries its own doubles: two of them, small enough to
// read in one go, because the interfaces they satisfy are wider than the two
// methods this exercises.

type refreshBackend struct {
	sprints []backend.Sprint
	err     error
}

func (b refreshBackend) BoardSprints(context.Context, int) ([]backend.Sprint, error) {
	return b.sprints, b.err
}
func (refreshBackend) MoveIssuesToSprint(context.Context, string, []string) error { return nil }
func (refreshBackend) StartSprint(context.Context, int, backend.SprintDraft) error {
	return nil
}
func (refreshBackend) CompleteSprint(context.Context, int) error { return nil }
func (refreshBackend) CreateSprint(context.Context, int, backend.SprintDraft) (backend.Sprint, error) {
	return backend.Sprint{}, nil
}
func (refreshBackend) EditSprint(context.Context, int, backend.SprintDraft, bool) error { return nil }
func (refreshBackend) DeleteSprint(context.Context, int) error                          { return nil }

// refreshStore records what reached ReplaceSprints, which is the only thing
// either refresh writes, and answers everything else with a zero value.
type refreshStore struct {
	wrote   bool
	sprints []backend.Sprint
}

func (s *refreshStore) ReplaceSprints(_ context.Context, _ string, _ int, list []backend.Sprint) error {
	s.wrote = true
	s.sprints = list
	return nil
}
func (*refreshStore) Columns(context.Context, string, int) ([]backend.BoardColumn, error) {
	return nil, nil
}
func (*refreshStore) BoardSprintState(context.Context, string, int, string) (string, bool, error) {
	return "", false, nil
}
func (*refreshStore) SprintName(context.Context, string, string) (string, error) { return "", nil }
func (*refreshStore) SprintIssues(context.Context, string, int, string) ([]string, error) {
	return nil, nil
}
func (*refreshStore) ReplaceSprintIssues(context.Context, string, int, string, []string) error {
	return nil
}
func (*refreshStore) DeleteSprintEverywhere(context.Context, string, int) error { return nil }

func TestARefreshAfterACeremonyRefusesToWriteAnEmptyAnswer(t *testing.T) {
	store := &refreshStore{}
	s := &Service{store: store}

	err := s.refreshSprints(context.Background(), refreshBackend{sprints: []backend.Sprint{}}, "p1", 1)
	if err == nil {
		t.Fatal("refreshSprints = nil, want a refusal: an empty answer is what a 400 looks like from here")
	}
	if store.wrote {
		t.Error("the cached list was replaced, which would delete the board's whole sprint history")
	}
}

func TestARefreshAfterADeleteWritesAnEmptyAnswerBecauseItIsTrue(t *testing.T) {
	store := &refreshStore{}
	s := &Service{store: store}

	err := s.refreshSprintsAllowEmpty(context.Background(), refreshBackend{sprints: []backend.Sprint{}}, "p1", 1)
	if err != nil {
		t.Fatalf("refreshSprintsAllowEmpty = %v, want no error: a deleted board's last sprint really does leave nothing", err)
	}
	if !store.wrote {
		t.Fatal("nothing was written, so the deleted sprint would stay in the cache forever")
	}
	if len(store.sprints) != 0 {
		t.Errorf("wrote %d sprints, want the empty list through", len(store.sprints))
	}
}

// preLifecycleBackend answers the four methods a ceremony backend answered
// before this package grew CreateSprint, EditSprint, and DeleteSprint. It
// is what Service.board() must refuse: a backend that can start and
// complete a sprint but cannot create, edit, or delete one is not the
// lifecycle interface asks for, and the refusal has to say so in words that
// cover all six writes, not just the two this double still answers.
type preLifecycleBackend struct{}

func (preLifecycleBackend) SearchIssuesPage(context.Context, string, string, string, []string, int, int) ([]backend.Issue, int, error) {
	return nil, 0, nil
}
func (preLifecycleBackend) BoardSprints(context.Context, int) ([]backend.Sprint, error) {
	return nil, nil
}
func (preLifecycleBackend) MoveIssuesToSprint(context.Context, string, []string) error { return nil }
func (preLifecycleBackend) StartSprint(context.Context, int, backend.SprintDraft) error {
	return nil
}
func (preLifecycleBackend) CompleteSprint(context.Context, int) error { return nil }

// TestBoardRefusesABackendThatCannotCreateEditOrDeleteSprints pins down both
// the refusal and its wording: a backend answering only the four methods
// that existed before create, edit, and delete were added must still be
// refused by Service.board(), with the one sentence a ceremony returns for
// every kind of unsupported backend, wide enough now to cover all six
// writes rather than only Start and Complete.
func TestBoardRefusesABackendThatCannotCreateEditOrDeleteSprints(t *testing.T) {
	s := &Service{b: preLifecycleBackend{}}

	_, err := s.board()
	if err == nil {
		t.Fatal("board() = nil error, want a refusal: this backend cannot create, edit, or delete a sprint")
	}
	want := "this connection cannot manage sprints"
	if err.Error() != want {
		t.Errorf("board() error = %q, want %q", err.Error(), want)
	}
}

func TestNeitherRefreshWritesWhenTheReadItselfFailed(t *testing.T) {
	for _, tc := range []struct {
		name string
		call func(*Service, lifecycle) error
	}{
		{"after a ceremony", func(s *Service, b lifecycle) error {
			return s.refreshSprints(context.Background(), b, "p1", 1)
		}},
		{"after a delete", func(s *Service, b lifecycle) error {
			return s.refreshSprintsAllowEmpty(context.Background(), b, "p1", 1)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := &refreshStore{}
			s := &Service{store: store}
			if err := tc.call(s, refreshBackend{err: errors.New("boom")}); err == nil {
				t.Fatal("a failed read reported success")
			}
			if store.wrote {
				t.Error("a failed read still wrote to the cache")
			}
		})
	}
}
