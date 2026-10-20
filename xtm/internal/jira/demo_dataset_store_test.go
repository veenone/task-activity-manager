package jira

import (
	"context"
	"reflect"
	"testing"
)

// TestADemoCommitIsWhatTheNextReadSees is #307: XTM builds a fresh client for
// every call, so a demo write has to outlive the client that made it, or the
// dataset catalog's scan reads the fixture back and the committed dataset is
// gone. It stays per demo URL, so two demo profiles do not share edits.
func TestADemoCommitIsWhatTheNextReadSees(t *testing.T) {
	t.Cleanup(resetDemoDatasets)
	ctx := context.Background()
	want := Dataset{Parameters: []DatasetParameter{{Name: "BROWSER"}}, Rows: [][]string{{"firefox"}, {"chrome"}}}
	if err := NewClient("demo", "").SetDataset(ctx, "DEMO2-7", want); err != nil {
		t.Fatalf("set: %v", err)
	}
	got, err := NewClient("demo", "").GetDataset(ctx, "DEMO2-7")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("a fresh client read %+v, want the committed %+v", got, want)
	}
	other, _ := NewClient("demo-2", "").GetDataset(ctx, "DEMO2-7")
	if reflect.DeepEqual(other, want) {
		t.Error("another demo profile saw this profile's edit")
	}
	// An emptied dataset is a write too: the test no longer has one.
	if err := NewClient("demo", "").SetDataset(ctx, "DEMO2-20", Dataset{}); err != nil {
		t.Fatalf("clear: %v", err)
	}
	if got, _ := NewClient("demo", "").GetDataset(ctx, "DEMO2-20"); len(got.Parameters) != 0 {
		t.Errorf("a cleared fixture dataset read back as %+v", got)
	}
}
