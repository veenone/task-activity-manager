package xray

import (
	"context"
	"testing"

	"agile-suite/xtm/internal/jira"
)

// Xray stores parameterized test inputs as datasets, so the capability is on
// and the adapter forwards the read (#267 Phase 1).
func TestXrayReportsDatasetSupport(t *testing.T) {
	if !(&Adapter{}).Capabilities().SupportsDatasets {
		t.Fatal("Xray must report SupportsDatasets")
	}
}

// Demo mode answers without a network call, so a demo adapter still returns a
// usable dataset for the keys that carry one.
func TestXrayDemoReturnsADatasetForEveryTwentiethTest(t *testing.T) {
	a := New(jira.NewClient("demo", ""))
	withData, err := a.GetDataset(context.Background(), "QA-1", "")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(withData.Parameters) == 0 {
		t.Fatalf("QA-1 should carry a demo dataset, got %+v", withData)
	}
	none, err := a.GetDataset(context.Background(), "QA-2", "")
	if err != nil {
		t.Fatalf("dataset: %v", err)
	}
	if len(none.Parameters) != 0 {
		t.Fatalf("QA-2 should carry no dataset, got %+v", none)
	}
}
