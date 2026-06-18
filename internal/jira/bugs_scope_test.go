package jira

import (
	"context"
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
