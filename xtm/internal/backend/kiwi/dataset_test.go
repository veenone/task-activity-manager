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
	if _, err := a.GetDataset(context.Background(), "T-1", ""); !errors.Is(err, backend.ErrUnsupported) {
		t.Fatalf("GetDataset err = %v, want ErrUnsupported", err)
	}
}
