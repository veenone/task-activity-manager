package jira

import (
	"context"
	"fmt"
	"testing"
)

func TestListBugsHonorsTestKeys(t *testing.T) {
	c := NewClient("demo", "")
	ctx := context.Background()

	// Full set reproduces the seed: at least one bug, with links.
	full, fullLinks, err := c.ListBugs(ctx, "DEMO", nil, "Bug", nil)
	if err != nil {
		t.Fatalf("full ListBugs: %v", err)
	}
	if len(full) == 0 || len(fullLinks) == 0 {
		t.Fatalf("expected seeded bugs and links, got %d bugs, %d links", len(full), len(fullLinks))
	}

	// Restrict to a single linked test key: every returned link must reference it,
	// and every returned bug must be referenced by some surviving link.
	target := fullLinks[0].TestKey
	bugs, links, err := c.ListBugs(ctx, "DEMO", []string{target}, "Bug", nil)
	if err != nil {
		t.Fatalf("scoped ListBugs: %v", err)
	}
	if len(links) == 0 {
		t.Fatalf("expected at least one link for %s", target)
	}
	for _, l := range links {
		if l.TestKey != target {
			t.Errorf("link references out-of-scope test %s (want only %s)", l.TestKey, target)
		}
	}
	bugKeys := map[string]bool{}
	for _, b := range bugs {
		bugKeys[b.Key] = true
	}
	for _, l := range links {
		if !bugKeys[l.BugKey] {
			t.Errorf("link to %s has no matching bug in the result", l.BugKey)
		}
	}

	// Empty (no in-scope tests) returns nothing.
	noBugs, noLinks, err := c.ListBugs(ctx, "DEMO", []string{}, "Bug", nil)
	if err != nil {
		t.Fatalf("empty ListBugs: %v", err)
	}
	if len(noBugs) != 0 || len(noLinks) != 0 {
		t.Errorf("empty testKeys should yield nothing, got %d bugs, %d links", len(noBugs), len(noLinks))
	}
}

func TestListBugsSpanningDefectPartialScope(t *testing.T) {
	c := NewClient("demo", "")
	ctx := context.Background()

	failed := demoFailedTestNums(10)
	if len(failed) < 3 {
		t.Skipf("need >=3 failed demo tests, got %d", len(failed))
	}
	// Scope to failed[1] only — one endpoint of the spanning defect. The other
	// endpoint (failed[2]) must not appear in any link.
	target := fmt.Sprintf("DEMO-%d", failed[1])
	excluded := fmt.Sprintf("DEMO-%d", failed[2])

	bugs, links, err := c.ListBugs(ctx, "DEMO", []string{target}, "Bug", nil)
	if err != nil {
		t.Fatalf("ListBugs: %v", err)
	}
	for _, l := range links {
		if l.TestKey == excluded {
			t.Errorf("link references excluded endpoint %s", excluded)
		}
		if l.TestKey != target {
			t.Errorf("link references out-of-scope test %s (want only %s)", l.TestKey, target)
		}
	}
	// Every returned link's bug must be present in the bugs slice (set integrity).
	bugKeys := map[string]bool{}
	for _, b := range bugs {
		bugKeys[b.Key] = true
	}
	for _, l := range links {
		if !bugKeys[l.BugKey] {
			t.Errorf("link to %s has no matching bug", l.BugKey)
		}
	}
	if len(links) == 0 {
		t.Errorf("expected at least one in-scope link for %s", target)
	}
}
