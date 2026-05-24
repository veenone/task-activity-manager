package jira

import "testing"

func TestIsDemoURLRecognisesDemoSchemes(t *testing.T) {
	cases := map[string]bool{
		"demo":                     true,
		"DEMO":                     true,
		"demo://anything":          true,
		"mock://local":             true,
		"  demo  ":                 true,
		"https://jira.example.com": false,
		"":                         false,
		"jira.demo.example.com":    false,
	}
	for url, want := range cases {
		if got := isDemoURL(url); got != want {
			t.Errorf("isDemoURL(%q) = %v, want %v", url, got, want)
		}
	}
}

func TestDemoTestsPageReportsTheFullTotal(t *testing.T) {
	_, total := demoTestsPage("QA", 0, 100)
	if total != demoTestCount {
		t.Errorf("total = %d, want %d", total, demoTestCount)
	}
}

func TestDemoTestsPagePaginates(t *testing.T) {
	first, _ := demoTestsPage("QA", 0, 100)
	second, _ := demoTestsPage("QA", 100, 100)

	if len(first) != 100 || len(second) != 100 {
		t.Fatalf("page sizes = %d / %d, want 100 / 100", len(first), len(second))
	}
	if first[0].Key == second[0].Key {
		t.Errorf("page boundary leaked: first[0]=%s second[0]=%s",
			first[0].Key, second[0].Key)
	}
}

func TestDemoTestsPageClampsBeyondTotal(t *testing.T) {
	page, _ := demoTestsPage("QA", demoTestCount-3, 100)
	if len(page) != 3 {
		t.Errorf("tail page size = %d, want 3", len(page))
	}
}

func TestMakeDemoTestIsDeterministic(t *testing.T) {
	a := makeDemoTest("QA", 42)
	b := makeDemoTest("QA", 42)
	if a.Summary != b.Summary || a.Status != b.Status || a.Key != b.Key {
		t.Errorf("makeDemoTest not deterministic: %+v vs %+v", a, b)
	}
}
