package permission

import (
	"strings"
	"testing"
)

func TestAllowsRefusesOnlyAPermissionJiraDenied(t *testing.T) {
	s := Set{"PLAT": {ManageSprints: false, EditIssues: true}}
	cases := []struct {
		project, perm string
		want          bool
	}{
		{"PLAT", ManageSprints, false},
		{"PLAT", EditIssues, true},
		{"PLAT", LinkIssues, true},     // Jira did not report it
		{"OTHER", ManageSprints, true}, // no sync asked about the project
		{"", ManageSprints, true},
	}
	for _, c := range cases {
		if got := s.Allows(c.project, c.perm); got != c.want {
			t.Errorf("Allows(%q, %s) = %v, want %v", c.project, c.perm, got, c.want)
		}
	}
	if !Set(nil).Allows("PLAT", ManageSprints) {
		t.Error("an empty set must allow everything")
	}
}

func TestCheckNamesThePermissionAndTheProject(t *testing.T) {
	s := Set{"PLAT": {ManageSprints: false}}
	err := s.Check("PLAT", ManageSprints)
	if err == nil || !strings.Contains(err.Error(), "Manage Sprints") || !strings.Contains(err.Error(), "PLAT") {
		t.Errorf("Check = %v, want an error naming Manage Sprints and PLAT", err)
	}
	if err := s.Check("PLAT", EditIssues); err != nil {
		t.Errorf("Check on an unknown permission = %v, want nil", err)
	}
}

func TestGatedKeepsOnlyTheKeysTAMActsOn(t *testing.T) {
	got := Gated(map[string]bool{EditIssues: true, ManageSprints: false, "ADMINISTER": true})
	if len(got) != 2 || !got[EditIssues] || got[ManageSprints] {
		t.Errorf("Gated = %v, want edit held, manage sprints denied, and nothing else", got)
	}
}

func TestProjectOfReadsTheKeyPrefix(t *testing.T) {
	for key, want := range map[string]string{"PLAT-431": "PLAT", "A-B-7": "A-B", "PLAT": "", "": "", "-3": ""} {
		if got := ProjectOf(key); got != want {
			t.Errorf("ProjectOf(%q) = %q, want %q", key, got, want)
		}
	}
}
