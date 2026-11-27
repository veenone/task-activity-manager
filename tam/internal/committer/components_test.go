package committer_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
)

// componentFake is the harness's Jira with components, logging every call,
// issue updates included, in the order they arrive.
type componentFake struct {
	*fake
	components map[int]backend.Component
	createErr  error
	calls      []string
	nextID     int
}

func (c *componentFake) UpdateIssue(ctx context.Context, key string, fields map[string]string) error {
	c.calls = append(c.calls, "update "+key)
	return c.fake.UpdateIssue(ctx, key, fields)
}

func (c *componentFake) ProjectVersions(context.Context, string) ([]backend.Version, error) {
	return nil, nil
}

func (c *componentFake) ProjectComponents(context.Context, string) ([]backend.Component, error) {
	out := []backend.Component{}
	for _, x := range c.components {
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (c *componentFake) CreateComponent(_ context.Context, project string, in backend.ComponentInput) (backend.Component, error) {
	if c.createErr != nil {
		return backend.Component{}, c.createErr
	}
	c.nextID++
	made := backend.Component{ID: c.nextID, Project: project, Name: in.Name, LeadName: in.LeadName, AssigneeType: in.AssigneeType}
	c.components[made.ID] = made
	c.calls = append(c.calls, fmt.Sprintf("create %s %s", project, in.Name))
	return made, nil
}

func (c *componentFake) UpdateComponent(_ context.Context, id int, in backend.ComponentInput) (backend.Component, error) {
	x := c.components[id]
	x.Name, x.Description, x.LeadName, x.AssigneeType = in.Name, in.Description, in.LeadName, in.AssigneeType
	c.components[id] = x
	c.calls = append(c.calls, fmt.Sprintf("edit %d %s", id, in.Name))
	return x, nil
}

func (c *componentFake) DeleteComponent(_ context.Context, id, moveTo int) error {
	delete(c.components, id)
	c.calls = append(c.calls, fmt.Sprintf("delete %d %d", id, moveTo))
	return nil
}

// newComponentHarness caches components 10000 (Auth) and 10001 (Board) on
// both sides, with PLAT-1 under Auth.
func newComponentHarness(t *testing.T) (harness, *componentFake) {
	t.Helper()
	h := newHarness(t)
	cs := []backend.Component{
		{ID: 10000, Project: "PLAT", Name: "Auth", AssigneeType: "PROJECT_DEFAULT"},
		{ID: 10001, Project: "PLAT", Name: "Board", AssigneeType: "PROJECT_DEFAULT"},
	}
	cf := &componentFake{fake: h.jira, components: map[int]backend.Component{}, nextID: 10299}
	for _, c := range cs {
		cf.components[c.ID] = c
	}
	if err := h.repo.ReplaceComponents(context.Background(), "p1", "PLAT", cs); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Exec(`UPDATE issue SET components = '[10000]' WHERE profile_id = 'p1' AND key = 'PLAT-1'`); err != nil {
		t.Fatal(err)
	}
	row := h.jira.rows["PLAT-1"]
	row.Components = []int{10000}
	h.jira.rows["PLAT-1"] = row
	h.eng = committer.New(cf, h.repo, h.order)
	return h, cf
}

func TestCommitCreatesADraftComponentBeforeTheIssueEditNamingIt(t *testing.T) {
	h, cf := newComponentHarness(t)
	ctx := context.Background()
	made, err := h.repo.CreateDraftComponent(ctx, "p1", "PLAT", issuerepo.ComponentFields{Name: "Reports"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-1", "components", "10000, "+strconv.Itoa(made.ID)); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(res.Failures) != 0 || len(res.Held) != 0 {
		t.Fatalf("failures %+v held %+v", res.Failures, res.Held)
	}
	want := []string{"create PLAT Reports", "update PLAT-1"}
	if !slices.Equal(cf.calls, want) {
		t.Fatalf("calls %v, want %v", cf.calls, want)
	}
	if !slices.Contains(res.ComponentsChanged, "Reports created") {
		t.Fatalf("changed %v", res.ComponentsChanged)
	}
	// The edit went out under the real id.
	if got := h.jira.updates; len(got) != 1 || got[0] != "PLAT-1 components=10000, 10300" {
		t.Fatalf("updates %v", got)
	}
	if left := pendingTypes(t, h); len(left) != 0 {
		t.Fatalf("journal left %v", left)
	}
}

func TestCommitHoldsAnIssueEditOnAComponentJiraRefused(t *testing.T) {
	h, cf := newComponentHarness(t)
	cf.createErr = errors.New("name taken")
	ctx := context.Background()
	made, _ := h.repo.CreateDraftComponent(ctx, "p1", "PLAT", issuerepo.ComponentFields{Name: "Reports"})
	if err := h.repo.EditField(ctx, "p1", "PLAT-1", "components", strconv.Itoa(made.ID)); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(res.Failures) != 1 || res.Failures[0].EntityType != issuerepo.EntityComponentCreate {
		t.Fatalf("failures %+v", res.Failures)
	}
	if len(res.Held) != 1 || res.Held[0].Key != "PLAT-1" {
		t.Fatalf("held %+v", res.Held)
	}
	if slices.Contains(cf.calls, "update PLAT-1") {
		t.Fatalf("the edit naming a draft went out: %v", cf.calls)
	}
}

func TestCommitEditOfAComponentChangedInJiraFails(t *testing.T) {
	h, cf := newComponentHarness(t)
	ctx := context.Background()
	if err := h.repo.EditComponent(ctx, "p1", 10000, issuerepo.ComponentFields{Name: "Login"}); err != nil {
		t.Fatal(err)
	}
	x := cf.components[10000]
	x.Name = "Identity"
	cf.components[10000] = x
	res := commit(t, h)
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Error, "changed in Jira") {
		t.Fatalf("failures %+v", res.Failures)
	}
	if len(cf.calls) != 0 {
		t.Fatalf("calls %v", cf.calls)
	}
	// Put Jira back: the edit goes.
	x.Name = "Auth"
	cf.components[10000] = x
	res = commit(t, h)
	if len(res.Failures) != 0 || !slices.Equal(cf.calls, []string{"edit 10000 Login"}) {
		t.Fatalf("failures %+v calls %v", res.Failures, cf.calls)
	}
}

func TestCommitDeletesAComponentAfterTheIssueEdits(t *testing.T) {
	h, cf := newComponentHarness(t)
	ctx := context.Background()
	if err := h.repo.EditField(ctx, "p1", "PLAT-1", "summary", "renamed"); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.QueueComponentDelete(ctx, "p1", 10000, 10001); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(res.Failures) != 0 {
		t.Fatalf("failures %+v", res.Failures)
	}
	want := []string{"update PLAT-1", "delete 10000 10001"}
	if !slices.Equal(cf.calls, want) {
		t.Fatalf("calls %v, want %v", cf.calls, want)
	}
	iss, _ := h.repo.GetIssue(ctx, "p1", "PLAT-1")
	if !slices.Equal(iss.Components, []int{10001}) {
		t.Fatalf("PLAT-1 components %v", iss.Components)
	}
	if left := pendingTypes(t, h); len(left) != 0 {
		t.Fatalf("journal left %v", left)
	}
}
