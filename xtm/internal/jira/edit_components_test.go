package jira

import (
	"reflect"
	"testing"
)

func TestFieldsForJiraComponents(t *testing.T) {
	got := FieldsForJira(map[string]string{"components": "\nUser Management\nAPI\n"})
	want := []map[string]string{{"name": "User Management"}, {"name": "API"}}
	if !reflect.DeepEqual(got["components"], want) {
		t.Fatalf("components %#v", got["components"])
	}
	cleared := FieldsForJira(map[string]string{"components": ""})
	v, ok := cleared["components"]
	if !ok || !reflect.DeepEqual(v, []map[string]string{}) {
		t.Fatalf("an empty value must clear: %#v (present %v)", v, ok)
	}
}
