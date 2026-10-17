package kiwi

import (
	"context"
	"errors"
	"testing"

	"agile-suite/xtm/internal/backend"
)

// Kiwi TCMS has no dataset equivalent. The capability is off and the read
// refuses rather than inventing an empty answer (#267 Phase 1).
func TestKiwiDoesNotSupportDatasets(t *testing.T) {
	a := &Adapter{}
	if a.Capabilities().SupportsDatasets {
		t.Fatal("Kiwi must not report SupportsDatasets")
	}
	if _, err := a.GetDataset(context.Background(), "T-1"); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("GetDataset err = %v, want ErrUnsupported", err)
	}
}

// Nothing to write to either, so the write capability is off and the write
// refuses. The commit path reads the capability and skips the row rather than
// failing it, which keeps the user's pending edit (#277 W1).
func TestKiwiDoesNotSupportDatasetWrites(t *testing.T) {
	a := &Adapter{}
	if a.Capabilities().SupportsDatasetWrites {
		t.Fatal("Kiwi must not report SupportsDatasetWrites")
	}
	if _, err := a.GetRunDataset(context.Background(), "EXEC-1", "T-1"); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("GetRunDataset err = %v, want ErrUnsupported", err)
	}
	err := a.SetDataset(context.Background(), "T-1", backend.Dataset{
		Parameters: []backend.DatasetParameter{{Name: "A"}},
		Rows:       [][]string{{"1"}},
	})
	if !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("SetDataset err = %v, want ErrUnsupported", err)
	}
}
