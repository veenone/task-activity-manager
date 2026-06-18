package jira

import (
	"strconv"
	"strings"
	"testing"
)

func TestDemoBugsLinkOnlyToFailedTests(t *testing.T) {
	failed := map[int]bool{}
	for _, n := range demoFailedTestNums(10) {
		failed[n] = true
	}
	if len(failed) < 3 {
		t.Fatalf("demoFailedTestNums returned %d, want >= 3", len(failed))
	}

	_, links := demoBugs("DEMO")
	for _, l := range links {
		num, ok := testNumOf(l.TestKey, "DEMO")
		if !ok {
			t.Fatalf("unexpected linked test key %q", l.TestKey)
		}
		if !failed[num] {
			t.Errorf("bug linked to DEMO-%d, which is not a FAILED demo test", num)
		}
	}
}

func TestDemoBugsAreCrossProjectAndVaried(t *testing.T) {
	bugs, links := demoBugs("DEMO")
	if len(bugs) < 10 {
		t.Fatalf("demoBugs produced %d bugs, want >= 10", len(bugs))
	}

	projects := map[string]int{}
	for _, b := range bugs {
		if b.ProjectKey == "DEMO" {
			t.Errorf("bug %s is in the test project DEMO; defects should be cross-project", b.Key)
		}
		projects[b.ProjectKey]++
	}
	if len(projects) < 2 {
		t.Errorf("bugs span %d projects, want >= 2 for cross-project demo", len(projects))
	}

	bugsPerTest := map[string]int{}
	testsPerBug := map[string]int{}
	for _, l := range links {
		bugsPerTest[l.TestKey]++
		testsPerBug[l.BugKey]++
	}
	multiBugTest, multiTestBug := false, false
	for _, n := range bugsPerTest {
		if n >= 2 {
			multiBugTest = true
		}
	}
	for _, n := range testsPerBug {
		if n >= 2 {
			multiTestBug = true
		}
	}
	if !multiBugTest {
		t.Error("expected at least one test linked to two bugs")
	}
	if !multiTestBug {
		t.Error("expected at least one bug linked to two tests")
	}
}

// testNumOf parses "<project>-<n>" and returns n when the project prefix matches.
func testNumOf(key, project string) (int, bool) {
	suffix, ok := strings.CutPrefix(key, project+"-")
	if !ok {
		return 0, false
	}
	n, err := strconv.Atoi(suffix)
	if err != nil {
		return 0, false
	}
	return n, true
}
