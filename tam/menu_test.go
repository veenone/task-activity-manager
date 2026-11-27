package main

import "testing"

// Releases has no number left to take (#605), so a view without one must
// get no shortcut rather than a Ctrl+ with no key.
func TestViewAcceleratorSkipsAViewWithoutANumber(t *testing.T) {
	if got := viewAccelerator(""); got != nil {
		t.Errorf("viewAccelerator(\"\") = %+v, want nil", got)
	}
	if got := viewAccelerator("6"); got == nil || got.Key != "6" {
		t.Errorf("viewAccelerator(\"6\") = %+v, want Ctrl+6", got)
	}
	seen := map[string]string{}
	for _, v := range menuViews {
		if v.accelerator == "" {
			continue
		}
		if other, dup := seen[v.accelerator]; dup {
			t.Errorf("%s and %s share Ctrl+%s", other, v.id, v.accelerator)
		}
		seen[v.accelerator] = v.id
	}
}

// Components follows Releases in the View menu, with no number either (#605).
func TestComponentsFollowsReleasesInTheViewMenu(t *testing.T) {
	for i, v := range menuViews {
		if v.id != "releases" {
			continue
		}
		if i+1 >= len(menuViews) || menuViews[i+1].id != "components" || menuViews[i+1].accelerator != "" {
			t.Fatalf("after releases: %+v", menuViews[i+1:])
		}
		return
	}
	t.Fatal("no releases view in the menu")
}
