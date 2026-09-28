package reportout

import (
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/tam/internal/demo"
)

func space(t *testing.T) *demo.Confluence {
	t.Helper()
	return demo.NewConfluence("TEAM", "root", false)
}

func TestPublishCreatesThePageAndSaysWhichOneItWrote(t *testing.T) {
	pages := space(t)
	got, err := Publish(context.Background(), pages, "TEAM", "root", sample())
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got.Title != "Sprint 11 · Report" {
		t.Errorf("page title = %q, want the document's own", got.Title)
	}
	if got.PageID == "" {
		t.Error("nothing named the page that was written")
	}
	page, ok := pages.Page(got.PageID)
	if !ok {
		t.Fatalf("page %s is not in the space", got.PageID)
	}
	if !strings.Contains(page.Body, "34 points") {
		t.Errorf("the figures did not reach the page:\n%s", page.Body)
	}
	if !strings.Contains(page.Body, "Committed is a minimum estimate.") {
		t.Errorf("the caveat did not reach the page:\n%s", page.Body)
	}
}

func TestPublishReplacesThePageItWroteLastTime(t *testing.T) {
	pages := space(t)
	first, err := Publish(context.Background(), pages, "TEAM", "root", sample())
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	d := sample()
	d.Sections[0].Table.Rows[1] = []string{"Completed", "31 points"}
	second, err := Publish(context.Background(), pages, "TEAM", "root", d)
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if second.PageID != first.PageID {
		t.Errorf("a second page %s was written beside %s", second.PageID, first.PageID)
	}
	page, _ := pages.Page(second.PageID)
	if !strings.Contains(page.Body, "31 points") || strings.Contains(page.Body, "29 points") {
		t.Errorf("the page still holds the old figures:\n%s", page.Body)
	}
}

func TestPublishRefusesADocumentWithNothingInIt(t *testing.T) {
	pages := space(t)
	if _, err := Publish(context.Background(), pages, "TEAM", "root", Document{}); err == nil {
		t.Fatal("want a refusal, got nil")
	}
	if _, ok := pages.Page("1000"); ok {
		t.Error("a page was written for a report that does not exist")
	}
}

// Whichever of the three calls fails, the failure has to carry the page, the
// space or the page id, and what Confluence said. A user told only that
// publishing failed has nothing to act on.
func TestPublishNamesThePageWhereItGoesAndTheReasonWhicheverCallFails(t *testing.T) {
	const title = "Sprint 11 · Report"
	const reason = "403 Forbidden: you cannot add a page here"
	for _, tc := range []struct {
		name string
		arm  func(t *testing.T, pages *demo.Confluence) []string
	}{
		{"the lookup", func(t *testing.T, pages *demo.Confluence) []string {
			pages.FailNext("find", title, errors.New(reason))
			return []string{`Confluence did not answer whether the page "` + title + `" is in TEAM`, reason}
		}},
		{"the update", func(t *testing.T, pages *demo.Confluence) []string {
			first, err := Publish(context.Background(), pages, "TEAM", "root", sample())
			if err != nil {
				t.Fatalf("first publish: %v", err)
			}
			pages.FailNext("update", first.PageID, errors.New(reason))
			return []string{`Confluence did not update the page "` + title + `" (` + first.PageID + `)`, reason}
		}},
		{"the create", func(t *testing.T, pages *demo.Confluence) []string {
			pages.FailNext("create", title, errors.New(reason))
			return []string{`Confluence did not create the page "` + title + `" in TEAM`, reason}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			pages := space(t)
			wants := tc.arm(t, pages)
			_, err := Publish(context.Background(), pages, "TEAM", "root", sample())
			if err == nil {
				t.Fatal("want a failure, got nil")
			}
			for _, want := range wants {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("the failure does not carry %q: %v", want, err)
				}
			}
		})
	}
}
