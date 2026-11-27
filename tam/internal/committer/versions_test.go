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

// versionFake is the harness's Jira with fix versions: it answers
// CatalogBackend and VersionBackend, and logs every call, issue updates
// included, in the order they arrive.
type versionFake struct {
	*fake
	versions   map[int]backend.Version
	unresolved map[int][]string
	createErr  error
	calls      []string
	nextID     int
}

func (v *versionFake) UpdateIssue(ctx context.Context, key string, fields map[string]string) error {
	v.calls = append(v.calls, "update "+key)
	return v.fake.UpdateIssue(ctx, key, fields)
}

func (v *versionFake) ProjectVersions(context.Context, string) ([]backend.Version, error) {
	out := []backend.Version{}
	for _, x := range v.versions {
		out = append(out, x)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

func (v *versionFake) ProjectComponents(context.Context, string) ([]backend.Component, error) {
	return nil, nil
}

func (v *versionFake) CreateVersion(_ context.Context, project string, in backend.VersionInput) (backend.Version, error) {
	if v.createErr != nil {
		return backend.Version{}, v.createErr
	}
	v.nextID++
	made := backend.Version{ID: v.nextID, Project: project, Name: in.Name, ReleaseDate: in.ReleaseDate}
	v.versions[made.ID] = made
	v.calls = append(v.calls, fmt.Sprintf("create %s %s", project, in.Name))
	return made, nil
}

func (v *versionFake) UpdateVersion(_ context.Context, id int, in backend.VersionInput) (backend.Version, error) {
	x := v.versions[id]
	x.Name, x.Description, x.StartDate, x.ReleaseDate = in.Name, in.Description, in.StartDate, in.ReleaseDate
	v.versions[id] = x
	v.calls = append(v.calls, fmt.Sprintf("edit %d %s", id, in.Name))
	return x, nil
}

func (v *versionFake) ReleaseVersion(_ context.Context, id int, date string, moveTo int) error {
	x := v.versions[id]
	x.Released = true
	v.versions[id] = x
	v.calls = append(v.calls, fmt.Sprintf("release %d %s %d", id, date, moveTo))
	return nil
}

func (v *versionFake) ArchiveVersion(_ context.Context, id int, archived bool) error {
	v.calls = append(v.calls, fmt.Sprintf("archive %d %v", id, archived))
	return nil
}

func (v *versionFake) DeleteVersion(_ context.Context, id, moveTo int) error {
	delete(v.versions, id)
	v.calls = append(v.calls, fmt.Sprintf("delete %d %d", id, moveTo))
	return nil
}

func (v *versionFake) UnresolvedInVersion(_ context.Context, id int) ([]string, error) {
	return v.unresolved[id], nil
}

// newVersionHarness caches releases 10100 (v1.2), 10101 (v1.1, released)
// and 10103 (v1.3) on both sides, with PLAT-1 in v1.2.
func newVersionHarness(t *testing.T) (harness, *versionFake) {
	t.Helper()
	h := newHarness(t)
	vs := []backend.Version{
		{ID: 10100, Project: "PLAT", Name: "v1.2"},
		{ID: 10101, Project: "PLAT", Name: "v1.1", Released: true},
		{ID: 10103, Project: "PLAT", Name: "v1.3"},
	}
	vf := &versionFake{fake: h.jira, versions: map[int]backend.Version{}, unresolved: map[int][]string{}, nextID: 10199}
	for _, v := range vs {
		vf.versions[v.ID] = v
	}
	if err := h.repo.ReplaceVersions(context.Background(), "p1", "PLAT", vs); err != nil {
		t.Fatal(err)
	}
	if _, err := h.db.Exec(`UPDATE issue SET fix_versions = '[10100]' WHERE profile_id = 'p1' AND key = 'PLAT-1'`); err != nil {
		t.Fatal(err)
	}
	row := h.jira.rows["PLAT-1"]
	row.FixVersions = []int{10100}
	h.jira.rows["PLAT-1"] = row
	h.eng = committer.New(vf, h.repo, h.order)
	return h, vf
}

func commit(t *testing.T, h harness) committer.Result {
	t.Helper()
	res, err := h.eng.Commit(context.Background(), "p1", "PLAT")
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func pendingTypes(t *testing.T, h harness) []string {
	t.Helper()
	rows, err := h.repo.ListPendingChanges(context.Background(), "p1")
	if err != nil {
		t.Fatal(err)
	}
	return slices.Sorted(func(yield func(string) bool) {
		for _, p := range rows {
			if !yield(p.EntityType + " " + p.EntityKey) {
				return
			}
		}
	})
}

func TestCommitCreatesADraftReleaseBeforeTheEditsNamingIt(t *testing.T) {
	h, vf := newVersionHarness(t)
	ctx := context.Background()
	draft, err := h.repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	if err != nil {
		t.Fatal(err)
	}
	if err := h.repo.EditField(ctx, "p1", "PLAT-2", "fixVersions", strconv.Itoa(draft.ID)); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(res.Failures) != 0 || len(res.Held) != 0 {
		t.Fatalf("failures %+v held %+v", res.Failures, res.Held)
	}
	if want := []string{"create PLAT v2.0", "update PLAT-2"}; !slices.Equal(vf.calls, want) {
		t.Fatalf("calls %v, want %v", vf.calls, want)
	}
	if got := h.jira.updates; len(got) != 1 || got[0] != "PLAT-2 fixVersions=10200" {
		t.Fatalf("updates %v", got)
	}
	if !slices.Contains(res.VersionsChanged, "v2.0 created") {
		t.Fatalf("versions changed %v", res.VersionsChanged)
	}
	if rest := pendingTypes(t, h); len(rest) != 0 {
		t.Fatalf("journal left %v", rest)
	}
}

func TestCommitHoldsEditsWhenTheReleaseCreateIsRefused(t *testing.T) {
	h, vf := newVersionHarness(t)
	vf.createErr = errors.New("A version with this name already exists")
	ctx := context.Background()
	draft, _ := h.repo.CreateDraftVersion(ctx, "p1", "PLAT", issuerepo.VersionFields{Name: "v2.0"})
	if err := h.repo.EditField(ctx, "p1", "PLAT-2", "fixVersions", strconv.Itoa(draft.ID)); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(h.jira.updates) != 0 {
		t.Fatalf("sent %v", h.jira.updates)
	}
	if len(res.Failures) != 1 || !strings.Contains(res.Failures[0].Error, "already exists") {
		t.Fatalf("failures %+v", res.Failures)
	}
	if len(res.Held) != 1 || res.Held[0].Key != "PLAT-2" || !strings.Contains(res.Held[0].Reason, "v2.0") {
		t.Fatalf("held %+v", res.Held)
	}
}

func TestCommitEditsARelease(t *testing.T) {
	h, vf := newVersionHarness(t)
	if err := h.repo.EditVersion(context.Background(), "p1", 10100, issuerepo.VersionFields{Name: "v1.2b", ReleaseDate: "2026-12-20"}); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(res.Failures) != 0 || !slices.Equal(vf.calls, []string{"edit 10100 v1.2b"}) {
		t.Fatalf("failures %+v calls %v", res.Failures, vf.calls)
	}
	if rest := pendingTypes(t, h); len(rest) != 0 {
		t.Fatalf("journal left %v", rest)
	}
}

func TestCommitReportsAReleaseChangedInJira(t *testing.T) {
	h, vf := newVersionHarness(t)
	if err := h.repo.EditVersion(context.Background(), "p1", 10100, issuerepo.VersionFields{Name: "v1.2b"}); err != nil {
		t.Fatal(err)
	}
	remote := vf.versions[10100]
	remote.Name = "v1.2 hotfix"
	vf.versions[10100] = remote
	res := commit(t, h)
	if len(vf.calls) != 0 {
		t.Fatalf("sent %v", vf.calls)
	}
	if len(res.Failures) != 1 || res.Failures[0].Retryable || !strings.Contains(res.Failures[0].Error, "v1.2 hotfix") {
		t.Fatalf("failures %+v", res.Failures)
	}
	if rest := pendingTypes(t, h); !slices.Equal(rest, []string{"version_edit 10100"}) {
		t.Fatalf("journal %v", rest)
	}
}

func TestCommitReleasesAfterTheCarryOvers(t *testing.T) {
	h, vf := newVersionHarness(t)
	if err := h.repo.QueueVersionRelease(context.Background(), "p1", 10100, "2026-12-15", 10103, []issuerepo.ReleaseMove{{Key: "PLAT-1", To: 10103}}); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(res.Failures) != 0 || len(res.Held) != 0 {
		t.Fatalf("failures %+v held %+v", res.Failures, res.Held)
	}
	if want := []string{"update PLAT-1", "release 10100 2026-12-15 10103"}; !slices.Equal(vf.calls, want) {
		t.Fatalf("calls %v, want %v", vf.calls, want)
	}
	vs, _ := h.repo.Versions(context.Background(), "p1", "PLAT")
	for _, v := range vs {
		if v.ID == 10100 && !v.Released {
			t.Fatal("cache not marked released")
		}
	}
}

func TestCommitHoldsAReleaseWhoseCarryOverDidNotLand(t *testing.T) {
	h, vf := newVersionHarness(t)
	h.jira.updateErr = map[string]error{"PLAT-1": errors.New("Field 'fixVersions' cannot be set")}
	if err := h.repo.QueueVersionRelease(context.Background(), "p1", 10100, "2026-12-15", 0, []issuerepo.ReleaseMove{{Key: "PLAT-1", To: 0}}); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if slices.ContainsFunc(vf.calls, func(c string) bool { return strings.HasPrefix(c, "release") }) {
		t.Fatalf("released with a descope pending: %v", vf.calls)
	}
	held := slices.IndexFunc(res.Held, func(x committer.Held) bool { return x.EntityType == issuerepo.EntityVersionRelease })
	if held < 0 || !strings.Contains(res.Held[held].Reason, "PLAT-1") {
		t.Fatalf("held %+v", res.Held)
	}
}

func TestCommitHoldsAReleaseWithUnfinishedIssuesNobodyCovered(t *testing.T) {
	h, vf := newVersionHarness(t)
	vf.unresolved[10100] = []string{"OTHER-7"}
	if err := h.repo.QueueVersionRelease(context.Background(), "p1", 10100, "2026-12-15", 0, nil); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(vf.calls) != 0 {
		t.Fatalf("sent %v", vf.calls)
	}
	if len(res.Held) != 1 || !strings.Contains(res.Held[0].Reason, "OTHER-7") {
		t.Fatalf("held %+v", res.Held)
	}
	// With a carry-over release named, the same issue goes with the release.
	h2, vf2 := newVersionHarness(t)
	vf2.unresolved[10100] = []string{"OTHER-7"}
	if err := h2.repo.QueueVersionRelease(context.Background(), "p1", 10100, "2026-12-15", 10103, nil); err != nil {
		t.Fatal(err)
	}
	commit(t, h2)
	if !slices.Equal(vf2.calls, []string{"release 10100 2026-12-15 10103"}) {
		t.Fatalf("calls %v", vf2.calls)
	}
}

func TestCommitArchivesThenDeletes(t *testing.T) {
	h, vf := newVersionHarness(t)
	ctx := context.Background()
	if err := h.repo.QueueVersionDelete(ctx, "p1", 10103, 10100); err != nil {
		t.Fatal(err)
	}
	if err := h.repo.QueueVersionArchive(ctx, "p1", 10101, true); err != nil {
		t.Fatal(err)
	}
	res := commit(t, h)
	if len(res.Failures) != 0 {
		t.Fatalf("failures %+v", res.Failures)
	}
	if want := []string{"archive 10101 true", "delete 10103 10100"}; !slices.Equal(vf.calls, want) {
		t.Fatalf("calls %v, want %v", vf.calls, want)
	}
	vs, _ := h.repo.Versions(ctx, "p1", "PLAT")
	if slices.ContainsFunc(vs, func(v backend.Version) bool { return v.ID == 10103 }) {
		t.Fatal("deleted release still cached")
	}
}
