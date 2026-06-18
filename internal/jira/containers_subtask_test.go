package jira

import (
	"context"
	"strings"
	"testing"
)

func TestDemoSeedsSubTaskExecutions(t *testing.T) {
	c := NewClient("demo", "")
	containers, links, err := c.ListContainers(context.Background(), "DEMO", nil)
	if err != nil {
		t.Fatalf("ListContainers: %v", err)
	}
	var subs []Container
	for _, ct := range containers {
		if ct.Kind == KindTestExec && ct.ParentKey != "" {
			subs = append(subs, ct)
		}
	}
	if len(subs) < 2 {
		t.Fatalf("want >=2 sub-task executions, got %d", len(subs))
	}
	for _, s := range subs {
		if !strings.HasPrefix(s.ParentKey, "DEMO-S-") {
			t.Errorf("sub-task %s has unexpected parent %q", s.Key, s.ParentKey)
		}
		if s.IssueType == "" {
			t.Errorf("sub-task %s missing issue type", s.Key)
		}
	}
	// Sub-task executions carry run links like standalone ones.
	linked := map[string]bool{}
	for _, l := range links {
		linked[l.ContainerKey] = true
	}
	for _, s := range subs {
		if !linked[s.Key] {
			t.Errorf("sub-task execution %s has no run links", s.Key)
		}
	}
}
