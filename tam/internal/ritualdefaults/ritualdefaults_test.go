package ritualdefaults_test

import (
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/ritualdefaults"
)

func points(v float64) *float64 { return &v }

func sample() []backend.Issue {
	return []backend.Issue{
		{Key: "PLAT-1", Status: "To Do", StoryPoints: points(3)},
		{Key: "PLAT-2", Status: "In Progress", StoryPoints: nil},
		{Key: "PLAT-3", Status: "Done", StoryPoints: points(5)},
		{Key: "PLAT-4", Status: "Blocked", StoryPoints: points(2)},
	}
}

func keys(issues []ritualdefaults.Selected) []string {
	out := make([]string, 0, len(issues))
	for _, i := range issues {
		out = append(out, i.Key)
	}
	return out
}

func TestPlanningTakesTheWholeSprintUnestimatedFirst(t *testing.T) {
	got := keys(ritualdefaults.Select("planning", sample()))
	want := []string{"PLAT-2", "PLAT-1", "PLAT-3", "PLAT-4"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestStandupTakesOnlyWorkInFlight(t *testing.T) {
	got := keys(ritualdefaults.Select("standup", sample()))
	if len(got) != 2 || got[0] != "PLAT-2" || got[1] != "PLAT-4" {
		t.Fatalf("got %v, want the in-progress and blocked issues", got)
	}
}

func TestReviewTakesOnlyWhatIsDone(t *testing.T) {
	got := keys(ritualdefaults.Select("review", sample()))
	if len(got) != 1 || got[0] != "PLAT-3" {
		t.Fatalf("got %v, want only the done issue", got)
	}
}

func TestRetroLeadsWithWhatDidNotFinish(t *testing.T) {
	got := keys(ritualdefaults.Select("retro", sample()))
	want := []string{"PLAT-1", "PLAT-2", "PLAT-4", "PLAT-3"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestAnUnknownRitualTypeSelectsNothing(t *testing.T) {
	if got := ritualdefaults.Select("retrospective-party", sample()); len(got) != 0 {
		t.Fatalf("got %v, want none", got)
	}
}

func TestTitleNamesTheSprintAndTheRitual(t *testing.T) {
	if got := ritualdefaults.Title("retro", "Sprint 14"); got != "Sprint 14 Retrospective" {
		t.Fatalf("title = %q", got)
	}
}
