package jira_test

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// catalogScreen is sixFieldScreen with components and fix versions on it.
const catalogScreen = `{"fields":{
	"summary":{"required":true,"name":"Summary","operations":["set"],"schema":{"type":"string","system":"summary"}},
	"components":{"required":false,"name":"Component/s","operations":["add","set","remove"],"schema":{"type":"array","items":"component","system":"components"}},
	"fixVersions":{"required":false,"name":"Fix Version/s","operations":["set","add","remove"],"schema":{"type":"array","items":"version","system":"fixVersions"}}
}}`

func TestUpdateIssueSendsCatalogIDObjects(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = catalogScreen
	err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"components": "10000, 10001", "fixVersions": ""})
	if err != nil {
		t.Fatalf("UpdateIssue: %v", err)
	}
	var put string
	for _, w := range f.writes {
		if strings.HasPrefix(w, "PUT ") {
			put = w
		}
	}
	for _, want := range []string{`"components":[{"id":"10000"},{"id":"10001"}]`, `"fixVersions":[]`} {
		if !strings.Contains(put, want) {
			t.Errorf("body lacks %s: %s", want, put)
		}
	}
}

func TestUpdateIssueRefusesFixVersionsOffTheScreen(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = sixFieldScreen
	err := b.UpdateIssue(context.Background(), "PLAT-412", map[string]string{"fixVersions": "10100"})
	if err == nil || !strings.Contains(err.Error(), "Fix versions") {
		t.Fatalf("got %v, want a refusal naming Fix versions", err)
	}
	for _, w := range f.writes {
		if strings.HasPrefix(w, "PUT ") {
			t.Fatalf("sent anyway: %s", w)
		}
	}
}

func TestEditableFieldsNamesComponentsAndFixVersions(t *testing.T) {
	b, f := newBackend(t, threeFields)
	f.editMeta = catalogScreen
	got, err := b.EditableFields(context.Background(), "PLAT-412")
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Contains(got, "components") || !slices.Contains(got, "fixVersions") {
		t.Fatalf("editable = %v, want components and fixVersions", got)
	}
}
