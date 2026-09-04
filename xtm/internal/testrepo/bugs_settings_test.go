package testrepo_test

import (
	"errors"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func TestBugSettingsDefaultWithoutALookup(t *testing.T) {
	r := newRepo(t)
	if got := r.ProfileBugIssueType("p1"); got != "Bug" {
		t.Fatalf("issue type = %q; want Bug", got)
	}
	if got := r.ProfileBugProjectMode("p1"); got != "test" {
		t.Fatalf("mode = %q; want test", got)
	}
	if got := r.ProfileBugProjectKey("p1"); got != "" {
		t.Fatalf("key = %q; want empty", got)
	}
}

func TestBugSettingsComeFromTheLookup(t *testing.T) {
	r := newRepo(t)
	r.SetBugSettingsLookup(func(id string) (testrepo.BugSettings, error) {
		if id != "p1" {
			return testrepo.BugSettings{}, errors.New("unknown profile")
		}
		return testrepo.BugSettings{IssueType: "Defect", ProjectMode: "dedicated", ProjectKey: "BUGS"}, nil
	})
	if got := r.ProfileBugIssueType("p1"); got != "Defect" {
		t.Fatalf("issue type = %q; want Defect", got)
	}
	if got := r.ProfileBugProjectMode("p1"); got != "dedicated" {
		t.Fatalf("mode = %q; want dedicated", got)
	}
	if got := r.ProfileBugProjectKey("p1"); got != "BUGS" {
		t.Fatalf("key = %q; want BUGS", got)
	}
	// A lookup failure falls back to the defaults rather than surfacing.
	if got := r.ProfileBugIssueType("missing"); got != "Bug" {
		t.Fatalf("issue type for unknown profile = %q; want Bug", got)
	}
}
