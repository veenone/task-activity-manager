package syncer

import (
	"testing"

	"xray-test-manager/internal/testrepo"
)

func TestIsRemoteAheadDetectsLaterTimestamp(t *testing.T) {
	if !isRemoteAhead(
		"2026-05-26T08:00:00.000+0700",
		"2026-05-26T07:00:00.000+0700",
	) {
		t.Error("later remote should be flagged as ahead")
	}
}

func TestIsRemoteAheadEqualTimestampsReturnFalse(t *testing.T) {
	s := "2026-05-26T07:00:00.000+0700"
	if isRemoteAhead(s, s) {
		t.Error("equal timestamps must not be ahead — no false conflict")
	}
}

func TestIsRemoteAheadEarlierRemoteReturnsFalse(t *testing.T) {
	if isRemoteAhead(
		"2026-05-26T06:00:00.000+0700",
		"2026-05-26T07:00:00.000+0700",
	) {
		t.Error("earlier remote must not be flagged as ahead")
	}
}

func TestIsRemoteAheadComparesAcrossFormats(t *testing.T) {
	// base in Jira's format; remote in RFC 3339 — both should parse.
	if !isRemoteAhead(
		"2026-05-26T08:00:00Z",
		"2026-05-26T07:00:00.000+0000",
	) {
		t.Error("should compare across timestamp formats")
	}
}

func TestIsRemoteAheadUnparseableInputReturnsFalse(t *testing.T) {
	if isRemoteAhead("not a date", "2026-05-26T07:00:00.000+0700") {
		t.Error("unparseable remote must not manufacture a false conflict")
	}
}

func TestOldestBaseVersionPicksEarliest(t *testing.T) {
	changes := []testrepo.PendingChange{
		{BaseVersion: "2026-05-26T08:00:00.000+0700"},
		{BaseVersion: "2026-05-26T07:00:00.000+0700"},
		{BaseVersion: "2026-05-26T09:00:00.000+0700"},
	}

	got := oldestBaseVersion(changes)

	if got != "2026-05-26T07:00:00.000+0700" {
		t.Errorf("got %q, want earliest base_version", got)
	}
}

func TestOldestBaseVersionSkipsEmptyValues(t *testing.T) {
	changes := []testrepo.PendingChange{
		{BaseVersion: ""},
		{BaseVersion: "2026-05-26T07:00:00.000+0700"},
	}

	got := oldestBaseVersion(changes)

	if got != "2026-05-26T07:00:00.000+0700" {
		t.Errorf("got %q, want the non-empty base", got)
	}
}

func TestOldestBaseVersionEmptyInputReturnsEmpty(t *testing.T) {
	if got := oldestBaseVersion(nil); got != "" {
		t.Errorf("empty input should yield empty, got %q", got)
	}
}
