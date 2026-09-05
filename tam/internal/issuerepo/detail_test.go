package issuerepo_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

func TestDetailIsCachedWithItsFetchTime(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	if _, _, ok, err := r.ReadDetail(ctx, "p1", "PLAT-412"); err != nil || ok {
		t.Fatalf("before write: ok=%v err=%v; want no cached detail", ok, err)
	}
	at := time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC)
	d := backend.IssueDetail{
		Key:         "PLAT-412",
		Description: "As a shopper I can enter a promo code.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "Promo code applies discount", IssueType: "Test"},
			{Direction: "outward", Type: "Relates", Key: "PLAT-350", Summary: "Promotions and discounts", IssueType: "Epic"},
		},
		Fields: map[string]any{"customfield_10016": 5.0},
	}
	if err := r.WriteDetail(ctx, "p1", "PLAT-412", d, at); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, fetchedAt, ok, err := r.ReadDetail(ctx, "p1", "PLAT-412")
	if err != nil || !ok {
		t.Fatalf("read: ok=%v err=%v", ok, err)
	}
	if !fetchedAt.Equal(at) {
		t.Errorf("fetchedAt = %v, want %v", fetchedAt, at)
	}
	if got.Description != d.Description || len(got.Links) != 2 || got.Fields["customfield_10016"] != 5.0 {
		t.Errorf("detail = %+v", got)
	}
	if err := r.WriteDetail(ctx, "p1", "PLAT-1", d, at); !errors.Is(err, issuerepo.ErrNotFound) {
		t.Errorf("writing detail for an uncached key: err = %v, want ErrNotFound", err)
	}
}

func TestWriteDetailReplacesLinks(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	first := backend.IssueDetail{Links: []backend.Link{
		{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "old", IssueType: "Test"},
		{Direction: "inward", Type: "Tested By", Key: "XT-1019", Summary: "Expired promo code rejected", IssueType: "Test"},
	}}
	if err := r.WriteDetail(ctx, "p1", "PLAT-412", first, time.Now()); err != nil {
		t.Fatalf("first write: %v", err)
	}
	second := backend.IssueDetail{Links: []backend.Link{
		{Direction: "inward", Type: "Tested By", Key: "XT-1019", Summary: "Expired promo code rejected", IssueType: "Test"},
	}}
	if err := r.WriteDetail(ctx, "p1", "PLAT-412", second, time.Now()); err != nil {
		t.Fatalf("second write: %v", err)
	}
	links, err := r.ListLinks(ctx, "p1", "PLAT-412")
	if err != nil {
		t.Fatalf("links: %v", err)
	}
	if len(links) != 1 || links[0].Key != "XT-1019" {
		t.Errorf("links = %+v, want only XT-1019", links)
	}
}

func TestListLinkedTestsFiltersByLinkTypeCaseInsensitively(t *testing.T) {
	r := newRepo(t)
	ctx := context.Background()
	if err := r.UpsertPage(ctx, "p1", sample(), time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	d := backend.IssueDetail{Links: []backend.Link{
		{Direction: "inward", Type: "Tested By", Key: "XT-1019", Summary: "Expired promo code rejected", IssueType: "Test"},
		{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "Promo code applies discount", IssueType: "Test"},
		{Direction: "outward", Type: "Relates", Key: "PLAT-350", Summary: "Promotions and discounts", IssueType: "Epic"},
		{Direction: "inward", Type: "Verified By", Key: "XT-2001", Summary: "Promo code audit trail", IssueType: "Test"},
		{Direction: "inward", Type: "Test Case Linkage", Key: "XT-2002", Summary: "Promo code expiry", IssueType: "Test"},
	}}
	if err := r.WriteDetail(ctx, "p1", "PLAT-412", d, time.Now()); err != nil {
		t.Fatalf("write: %v", err)
	}
	tests, err := r.ListLinkedTests(ctx, "p1", "PLAT-412", "tested by")
	if err != nil {
		t.Fatalf("linked tests: %v", err)
	}
	if len(tests) != 2 || tests[0].Key != "XT-1018" || tests[1].Key != "XT-1019" || tests[0].LinkType != "Tested By" {
		t.Errorf("tests = %+v", tests)
	}
	tests, err = r.ListLinkedTests(ctx, "p1", "PLAT-412", "Tested By")
	if err != nil || len(tests) != 2 {
		t.Errorf("an exact link type excludes Verified By: %+v, %v", tests, err)
	}
	tests, err = r.ListLinkedTests(ctx, "p1", "PLAT-412", "")
	if err != nil || len(tests) != 3 || tests[2].Key != "XT-2002" {
		t.Errorf("an empty link type matches anything test-shaped: %+v, %v", tests, err)
	}
	for _, lt := range tests {
		if lt.Key == "XT-2001" {
			t.Error(`the fallback matches on "test", so "Verified By" stays out`)
		}
	}
	tests, err = r.ListLinkedTests(ctx, "p1", "PLAT-412", "Verifies")
	if err != nil || len(tests) != 0 {
		t.Errorf("unrelated link type: %+v, %v", tests, err)
	}
}
