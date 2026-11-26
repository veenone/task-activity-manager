package demo_test

import (
	"context"
	"reflect"
	"slices"
	"testing"

	"agile-suite/tam/internal/backend"
	demobackend "agile-suite/tam/internal/backend/demo"
)

func TestDemoUpdateIssueSetsComponentsAndFixVersions(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	fields, _ := b.EditableFields(ctx, "ACME-409")
	if !slices.Contains(fields, "components") || !slices.Contains(fields, "fixVersions") {
		t.Fatalf("editable = %v, want both catalog fields on the demo screen", fields)
	}
	if err := b.UpdateIssue(ctx, "ACME-409", map[string]string{"components": "10001, 10003", "fixVersions": ""}); err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	got, _ := b.GetIssue(ctx, "ACME-409")
	if !reflect.DeepEqual(got.Components, []int{10001, 10003}) || len(got.FixVersions) != 0 || got.FixVersions == nil {
		t.Fatalf("after update: components %v, fix versions %#v", got.Components, got.FixVersions)
	}
}

func TestDemoCreateIssueKeepsComponentsAndFixVersions(t *testing.T) {
	b := demobackend.New("ACME")
	ctx := context.Background()
	key, _, err := b.CreateIssue(ctx, "ACME", backend.IssueDraft{Type: backend.TypeTask, Summary: "x", Components: []int{10000}, FixVersions: []int{10102}})
	if err != nil {
		t.Fatal(err)
	}
	got, _ := b.GetIssue(ctx, key)
	if !reflect.DeepEqual(got.Components, []int{10000}) || !reflect.DeepEqual(got.FixVersions, []int{10102}) {
		t.Fatalf("created %v %v", got.Components, got.FixVersions)
	}
}
