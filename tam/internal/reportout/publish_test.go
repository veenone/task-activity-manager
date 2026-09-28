package reportout

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"agile-suite/core/confluence"
	"agile-suite/tam/internal/demo"
)

func space(t *testing.T) *demo.Confluence {
	t.Helper()
	return demo.NewConfluence("TEAM", "root", false)
}

// withChart is the sample report with one chart in it, which is what makes a
// publish attach anything at all.
func withChart(t *testing.T) Document {
	t.Helper()
	d := sample()
	d.Sections[0].Images = []Image{sampleImage(t)}
	return d
}

// attachSpy is the fake space with a note of every file a publish put on a
// page. The fake keeps one entry per filename, as Confluence does, and says
// nothing about what it was handed, which is the part the pictures on the page
// depend on.
type attachSpy struct {
	*demo.Confluence
	attached []attachedFile
}

type attachedFile struct {
	pageID, name, contentType, attachmentID string
	data                                    []byte
}

func (s *attachSpy) AttachFile(ctx context.Context, pageID, filename, contentType string, data []byte) (confluence.Attachment, error) {
	a, err := s.Confluence.AttachFile(ctx, pageID, filename, contentType, data)
	if err != nil {
		return a, err
	}
	s.attached = append(s.attached, attachedFile{pageID: pageID, name: filename, contentType: contentType, attachmentID: a.ID, data: data})
	return a, nil
}

// The page is written first and the charts follow it, so the reference in the
// body has a file to resolve to once the upload lands.
func TestPublishPutsEachChartOnThePageTheBodyReferences(t *testing.T) {
	pages := &attachSpy{Confluence: space(t)}
	d := withChart(t)
	got, err := Publish(context.Background(), pages, "TEAM", "root", d)
	if err != nil {
		t.Fatalf("publish: %v", err)
	}
	if got.Warning != "" {
		t.Errorf("a publish where everything landed carries a warning: %q", got.Warning)
	}
	if len(pages.attached) != 1 {
		t.Fatalf("%d files went to the page, want the one chart the report has", len(pages.attached))
	}
	a := pages.attached[0]
	if a.pageID != got.PageID {
		t.Errorf("the chart went to page %s and the report to page %s", a.pageID, got.PageID)
	}
	if a.name != "chart-1.png" || a.contentType != "image/png" {
		t.Errorf("the chart arrived as %q, %q; the body references chart-1.png as a PNG", a.name, a.contentType)
	}
	raw, _, err := sampleImage(t).PNG()
	if err != nil {
		t.Fatalf("the sample chart: %v", err)
	}
	if !bytes.Equal(a.data, raw) {
		t.Errorf("the bytes on the page are %d long, want the chart's %d", len(a.data), len(raw))
	}
	page, ok := pages.Page(got.PageID)
	if !ok {
		t.Fatalf("page %s is not in the space", got.PageID)
	}
	if !strings.Contains(page.Body, `ri:filename="chart-1.png"`) {
		t.Errorf("the page does not reference the file that was attached to it:\n%s", page.Body)
	}
}

// Acceptance: the same sprint published twice leaves one page with one set of
// attachments, not a pile of them.
func TestPublishingTwiceLeavesOnePageAndOneSetOfCharts(t *testing.T) {
	pages := &attachSpy{Confluence: space(t)}
	first, err := Publish(context.Background(), pages, "TEAM", "root", withChart(t))
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	second, err := Publish(context.Background(), pages, "TEAM", "root", withChart(t))
	if err != nil {
		t.Fatalf("second publish: %v", err)
	}
	if second.PageID != first.PageID {
		t.Errorf("a second page %s was written beside %s", second.PageID, first.PageID)
	}
	if len(pages.attached) != 2 {
		t.Fatalf("the two publishes sent %d files, want the one chart each time", len(pages.attached))
	}
	if pages.attached[0].attachmentID != pages.attached[1].attachmentID {
		t.Errorf("the second publish left a second attachment %s beside %s instead of replacing it",
			pages.attached[1].attachmentID, pages.attached[0].attachmentID)
	}
}

// The tables are the report and the charts are an addition, so a chart
// Confluence refuses costs the user a picture and not the page.
func TestPublishKeepsThePageAndWarnsWhenAChartWillNotAttach(t *testing.T) {
	pages := space(t)
	first, err := Publish(context.Background(), pages, "TEAM", "root", withChart(t))
	if err != nil {
		t.Fatalf("first publish: %v", err)
	}
	const reason = "413 Payload Too Large: the attachment is over this instance's limit"
	pages.FailNext("attach", first.PageID, errors.New(reason))
	d := withChart(t)
	d.Sections[0].Table.Rows[1] = []string{"Completed", "31 points"}
	got, err := Publish(context.Background(), pages, "TEAM", "root", d)
	if err != nil {
		t.Fatalf("a chart that would not attach cost the user the page: %v", err)
	}
	if got.PageID != first.PageID || got.Title != d.Title {
		t.Errorf("the publish says it wrote %q (%s), want %q (%s)", got.Title, got.PageID, d.Title, first.PageID)
	}
	for _, want := range []string{"chart-1.png", reason} {
		if !strings.Contains(got.Warning, want) {
			t.Errorf("the warning does not carry %q: %q", want, got.Warning)
		}
	}
	page, _ := pages.Page(got.PageID)
	if !strings.Contains(page.Body, "31 points") {
		t.Errorf("the figures did not reach the page a chart failed on:\n%s", page.Body)
	}
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
