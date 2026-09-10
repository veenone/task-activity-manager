package sprints_test

import (
	"reflect"
	"sort"
	"strings"
	"testing"

	"agile-suite/tam/internal/sprints"
)

// immediateWrites is the fence. Every method named here reaches Jira the
// moment it is called, outside the journal and outside Commit, and they are
// the only writes in TAM that do.
//
// It is the Service's own exported method set and deliberately not the
// lifecycle interface, which is a different list for a different purpose:
// that one carries BoardSprints, a read, and MoveIssuesToSprint, which its
// other caller journals like every other membership change. A test claiming
// those were immediate writes would have been false the day it was written,
// and a fence nobody believes is worse than no fence.
var immediateWrites = []string{"Complete", "Create", "Delete", "Edit", "Start"}

// TestTheImmediateWritesAreExactlyTheFiveThatWereArguedFor is what makes the
// rule structural. The previous version of it lived in a sentence in a spec
// and lasted one phase.
func TestTheImmediateWritesAreExactlyTheFiveThatWereArguedFor(t *testing.T) {
	service := reflect.TypeOf(&sprints.Service{})
	got := make([]string, 0, service.NumMethod())
	for i := 0; i < service.NumMethod(); i++ {
		got = append(got, service.Method(i).Name)
	}
	sort.Strings(got)

	if !reflect.DeepEqual(got, immediateWrites) {
		t.Errorf("sprints.Service exports %s, want exactly %s.\n"+
			"This list is the fence around the one exception in TAM: a write that goes to Jira "+
			"without passing through the journal and without waiting for Commit. Adding a method "+
			"here adds a sixth, so amend section 3 of the sprints design "+
			"(docs/superpowers/specs/2026-09-10-tam-sprints-view-design.md) with the argument for "+
			"it and then add its name above, deliberately. If what you are adding is not an "+
			"immediate write, it does not belong on Service: Suggest is a package function for "+
			"exactly that reason.",
			strings.Join(got, ", "), strings.Join(immediateWrites, ", "))
	}
}
