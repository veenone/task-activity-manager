package backend

import (
	"reflect"
	"testing"
)

func TestParseIDs(t *testing.T) {
	got, err := ParseIDs(" 10001, 10000,,10001 , ")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if want := []int{10001, 10000}; !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, want %v: order kept, blanks and repeats dropped", got, want)
	}
	if got, err := ParseIDs(""); err != nil || got == nil || len(got) != 0 {
		t.Fatalf("empty: got %#v, %v; want an empty, non-nil list", got, err)
	}
	if _, err := ParseIDs("10000, Auth"); err == nil {
		t.Fatal("a name in the list parsed; want an error")
	}
}

func TestFormatIDs(t *testing.T) {
	if got := FormatIDs([]int{10000, 10001}); got != "10000, 10001" {
		t.Fatalf("got %q", got)
	}
	if got := FormatIDs(nil); got != "" {
		t.Fatalf("nil: got %q", got)
	}
}
