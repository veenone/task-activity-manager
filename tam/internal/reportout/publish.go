package reportout

import (
	"context"
	"fmt"
	"strings"

	"agile-suite/core/confluence"
	"agile-suite/tam/internal/errtext"
)

// Published is the page a publish wrote, so the user is told which one it
// was rather than that something happened somewhere.
//
// Warning is what went wrong without costing the user that page: a chart
// Confluence would not take as an attachment. The tables are the report and
// the charts are an addition to them, so a refused upload is reported beside a
// page that was written and is empty when every chart landed.
type Published struct {
	Title   string `json:"title"`
	PageID  string `json:"pageId"`
	Warning string `json:"warning"`
}

// pngType is what a chart is attached as. The frontend rasterises to PNG and
// Image.PNG refuses anything else, so there is one type to declare.
const pngType = "image/png"

// Publish writes a report to its own page in the space the rituals sync
// uses, under parentID, through the same confluence.Pages transport.
//
// It is a page of its own and not the sprint's overview page, which the
// rituals sync owns: writing the report into a page that sync tracks would
// bump the version behind the sync's back and leave that page in conflict
// on the next pass, every sprint, forever.
//
// It takes the page the title already names when there is one, so
// publishing the same sprint twice replaces the report rather than
// refusing on Confluence's unique title rule or leaving two pages of
// figures that disagree. A failure names the page and the reason, because
// "publishing failed" tells a user nothing they can act on.
//
// The body goes up first and the charts follow it, because an attachment
// needs a page id: the body names the files it expects and the references
// resolve as the uploads land. Writing the page, attaching, then writing the
// body a second time would be two versions of every report published and a
// broken picture on the page in between.
func Publish(ctx context.Context, pages confluence.Pages, spaceKey, parentID string, d Document) (Published, error) {
	if err := d.Check(); err != nil {
		return Published{}, err
	}
	found, exists, err := pages.FindPageByTitle(ctx, spaceKey, d.Title)
	if err != nil {
		return Published{}, fmt.Errorf("Confluence did not answer whether the page %q is in %s: %s", d.Title, spaceKey, errtext.Line(err))
	}
	body := Storage(d)
	written := Published{Title: d.Title}
	if exists {
		updated, err := pages.UpdatePage(ctx, found.ID, d.Title, body, found.Version+1)
		if err != nil {
			return Published{}, fmt.Errorf("Confluence did not update the page %q (%s): %s", d.Title, found.ID, errtext.Line(err))
		}
		written.PageID = updated.ID
	} else {
		created, err := pages.CreatePage(ctx, spaceKey, parentID, d.Title, body)
		if err != nil {
			return Published{}, fmt.Errorf("Confluence did not create the page %q in %s: %s", d.Title, spaceKey, errtext.Line(err))
		}
		written.PageID = created.ID
	}
	written.Warning = attachCharts(ctx, pages, written.PageID, d)
	return written, nil
}

// attachCharts puts every chart on the page the body already references and
// answers with the ones that did not make it, as a line for the user.
//
// The page exists by the time this runs and its tables are the report, so a
// chart Confluence refused is collected rather than returned as an error: the
// user keeps the page and is told which picture is missing from it. Each
// filename is named, because the reader of the line has to know which chart to
// go and look for.
func attachCharts(ctx context.Context, pages confluence.Pages, pageID string, d Document) string {
	var failed []string
	for _, s := range d.Sections {
		for _, im := range s.Images {
			raw, _, err := im.PNG()
			if err == nil {
				_, err = pages.AttachFile(ctx, pageID, im.Name, pngType, raw)
			}
			if err != nil {
				failed = append(failed, im.Name+": "+errtext.Line(err))
			}
		}
	}
	if len(failed) == 0 {
		return ""
	}
	return "Not every chart reached the page: " + strings.Join(failed, "; ") + "."
}
