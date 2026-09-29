package demo_test

import (
	"context"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

// TestDemoWorklogsStartEmptyAndHoldWhatWasPushed: the dataset seeds no
// entries, because a worklog only exists because somebody did the work, and a
// seeded one would be state no operation produced.
func TestDemoWorklogsStartEmptyAndHoldWhatWasPushed(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()

	first, err := b.Worklogs(ctx, "ACME-409")
	if err != nil {
		t.Fatalf("Worklogs: %v", err)
	}
	if len(first) != 0 {
		t.Errorf("worklogs = %+v, want none before anything was logged", first)
	}

	d := backend.WorklogDraft{Started: "2026-09-29T01:00:00.000+0700", TimeSpent: "2h 30m", Comment: "Pairing", Seconds: 9000}
	if err := b.AddWorklog(ctx, "ACME-409", d); err != nil {
		t.Fatalf("AddWorklog: %v", err)
	}
	logs, err := b.Worklogs(ctx, "ACME-409")
	if err != nil || len(logs) != 1 {
		t.Fatalf("worklogs = %+v, %v", logs, err)
	}
	if logs[0].TimeSpent != "2h 30m" || logs[0].Seconds != 9000 || logs[0].Started != d.Started || logs[0].Comment != "Pairing" {
		t.Errorf("entry = %+v, want the draft as pushed", logs[0])
	}
	// The entry belongs to the issue it was logged against and nowhere else.
	other, _ := b.Worklogs(ctx, "ACME-412")
	if len(other) != 0 {
		t.Errorf("another issue's worklogs = %+v, want none", other)
	}

	if err := b.AddWorklog(ctx, "NOPE-1", d); err == nil {
		t.Error("work was logged against an issue the demo dataset does not have")
	}
	if _, err := b.Worklogs(ctx, "NOPE-1"); err == nil {
		t.Error("worklogs were read for an issue the demo dataset does not have")
	}
}
