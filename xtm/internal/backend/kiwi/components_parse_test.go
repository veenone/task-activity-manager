package kiwi

import (
	"reflect"
	"testing"
)

// The journal's components value is the newline-bounded form that
// test_case.components holds, and names may contain spaces and commas.
func TestParseComponentSetReadsTheStoredForm(t *testing.T) {
	got := parseComponentSet("\nUser Management\nBilling, EU\n")
	want := []string{"User Management", "Billing, EU"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %q, want %q", got, want)
	}
	if got := parseComponentSet(""); len(got) != 0 {
		t.Fatalf("empty value: %q", got)
	}
}
