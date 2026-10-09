// Package componentadmin creates, edits and deletes Jira project components
// for the Components view. Writes go straight to Jira (a component belongs to
// the project, not to an issue, so the pending-change journal does not apply),
// then the cached component list and the cached test components are brought
// in line.
package componentadmin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/jira"
)

// ErrForbidden is what a 403 from Jira becomes. The text is shown as is.
var ErrForbidden = errors.New("You need project admin rights in Jira to change components.")

// Repo is the local store the service keeps in step with Jira.
type Repo interface {
	ReplaceProjectFieldOptions(profileID, projectKey, field string, values []string) error
	ComponentEditsPending(profileID, name string) (int, error)
	RenameComponentOnTests(profileID, oldName, newName string) (int, error)
	RemoveComponentFromTests(profileID, name string) (int, error)
}

// Backend is the slice of a backend the service needs.
type Backend interface {
	backend.ComponentManager
	ProjectComponents(ctx context.Context, projectKey string) ([]string, error)
}

// Service manages one profile's project components.
type Service struct {
	repo       Repo
	b          Backend
	profileID  string
	projectKey string
}

// New returns a Service for profileID's project.
func New(repo Repo, b Backend, profileID, projectKey string) *Service {
	return &Service{repo: repo, b: b, profileID: profileID, projectKey: projectKey}
}

// List returns the project's components from Jira.
func (s *Service) List(ctx context.Context) ([]backend.Component, error) {
	list, err := s.b.ProjectComponentDetails(ctx, s.projectKey)
	return list, translate(err)
}

// Create creates a component and refreshes the cached list.
func (s *Service) Create(ctx context.Context, in backend.ComponentInput) (backend.Component, error) {
	in, err := clean(in)
	if err != nil {
		return backend.Component{}, err
	}
	c, err := s.b.CreateComponent(ctx, s.projectKey, in)
	if err != nil {
		return backend.Component{}, translate(err)
	}
	return c, s.refresh(ctx)
}

// Update edits a component. A rename is refused while pending component edits
// name the old name, and rewrites the cached test components once Jira agrees.
func (s *Service) Update(ctx context.Context, id string, in backend.ComponentInput) (backend.Component, error) {
	in, err := clean(in)
	if err != nil {
		return backend.Component{}, err
	}
	old, err := s.find(ctx, id)
	if err != nil {
		return backend.Component{}, err
	}
	renaming := old.Name != "" && old.Name != in.Name
	if renaming {
		if err := s.guard(old.Name); err != nil {
			return backend.Component{}, err
		}
	}
	c, err := s.b.UpdateComponent(ctx, id, in)
	if err != nil {
		_ = s.refresh(ctx)
		return backend.Component{}, translate(err)
	}
	if renaming {
		if _, err := s.repo.RenameComponentOnTests(s.profileID, old.Name, c.Name); err != nil {
			return c, err
		}
	}
	return c, s.refresh(ctx)
}

// Delete deletes a component. With moveIssuesTo (a component id), Jira moves
// its issues there and the cached tests follow; otherwise the name is dropped
// from cached tests.
func (s *Service) Delete(ctx context.Context, id, moveIssuesTo string) error {
	old, err := s.find(ctx, id)
	if err != nil {
		return err
	}
	if old.Name != "" {
		if err := s.guard(old.Name); err != nil {
			return err
		}
	}
	target := backend.Component{}
	if moveIssuesTo != "" {
		if target, err = s.find(ctx, moveIssuesTo); err != nil {
			return err
		}
	}
	if err := s.b.DeleteComponent(ctx, id, moveIssuesTo); err != nil {
		_ = s.refresh(ctx)
		return translate(err)
	}
	if old.Name != "" {
		if target.Name != "" {
			_, err = s.repo.RenameComponentOnTests(s.profileID, old.Name, target.Name)
		} else {
			_, err = s.repo.RemoveComponentFromTests(s.profileID, old.Name)
		}
		if err != nil {
			return err
		}
	}
	return s.refresh(ctx)
}

// find returns the component with id from Jira's current list, or a zero
// Component when Jira no longer has it (the write then reports Jira's 404).
func (s *Service) find(ctx context.Context, id string) (backend.Component, error) {
	list, err := s.b.ProjectComponentDetails(ctx, s.projectKey)
	if err != nil {
		return backend.Component{}, translate(err)
	}
	for _, c := range list {
		if c.ID == id {
			return c, nil
		}
	}
	return backend.Component{}, nil
}

func (s *Service) guard(name string) error {
	n, err := s.repo.ComponentEditsPending(s.profileID, name)
	if err != nil {
		return err
	}
	if n > 0 {
		noun := "edit"
		if n > 1 {
			noun = "edits"
		}
		return fmt.Errorf("Commit or discard the %d pending component %s on tests that use %q first.", n, noun, name)
	}
	return nil
}

func (s *Service) refresh(ctx context.Context) error {
	names, err := s.b.ProjectComponents(ctx, s.projectKey)
	if err != nil {
		return translate(err)
	}
	if names == nil {
		names = []string{}
	}
	return s.repo.ReplaceProjectFieldOptions(s.profileID, s.projectKey, "component", names)
}

func clean(in backend.ComponentInput) (backend.ComponentInput, error) {
	in.Name = strings.TrimSpace(in.Name)
	in.Description = strings.TrimSpace(in.Description)
	if in.Name == "" {
		return in, errors.New("A component needs a name.")
	}
	if in.AssigneeType == "" {
		in.AssigneeType = "PROJECT_DEFAULT"
	}
	return in, nil
}

func translate(err error) error {
	var he *jira.HTTPError
	if errors.As(err, &he) && he.Code == http.StatusForbidden {
		return ErrForbidden
	}
	return err
}
