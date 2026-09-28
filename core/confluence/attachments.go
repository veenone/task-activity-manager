package confluence

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// Attachment is a file on a page, as Confluence reports it back. The id is
// what a later call addresses the attachment by; the filename is what a
// storage-format ri:attachment reference points at.
type Attachment struct {
	ID       string
	Filename string
}

// errBadAttachment is a filename or content type this package will not put in
// a multipart header. It stays unexported because a caller cannot recover
// from it: the fix is a different filename, not a retry.
var errBadAttachment = errors.New("confluence: an attachment filename must be a plain name with no path, quote or line break")

// AttachFile puts data on the page under filename. A sprint report is
// published again every time its sprint is re-reported, so the same filename
// arrives repeatedly: the file already there is looked up first and its data
// replaced, because posting the name a second time is a 400 on some Data
// Center versions and a duplicate attachment on others, and a page collecting
// copies of burndown.png is neither what the report wants nor recoverable
// from here. Replacing goes through the attachment's own data endpoint, which
// Atlassian documents as a POST, not the PUT that updates its properties.
func (c *Client) AttachFile(ctx context.Context, pageID, filename, contentType string, data []byte) (Attachment, error) {
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if err := checkAttachment(filename, contentType); err != nil {
		return Attachment{}, attachError(pageID, filename, err)
	}
	base := "/rest/api/content/" + url.PathEscape(pageID) + "/child/attachment"
	existing, err := c.attachmentID(ctx, base, filename)
	if err != nil {
		return Attachment{}, attachError(pageID, filename, err)
	}
	path := base
	if existing != "" {
		path = base + "/" + url.PathEscape(existing) + "/data"
	}
	var r rawAttachments
	if err := c.sendFile(ctx, http.MethodPost, path, filename, contentType, data).Decode(&r); err != nil {
		return Attachment{}, attachError(pageID, filename, err)
	}
	a, ok := r.attachment()
	if !ok {
		return Attachment{}, attachError(pageID, filename, errors.New("confluence: the answer named no attachment"))
	}
	if a.Filename == "" {
		// A replace answers without the title. The name sent is the name now
		// on the page, which is what a reference in the body has to match.
		a.Filename = filename
	}
	return a, nil
}

// attachmentID is the id of the file already on the page under filename, or
// empty when there is none. The filename filter is sent as a query, and the
// titles that come back are checked against it as well, because an instance
// that ignores the filter would otherwise hand back an unrelated attachment.
func (c *Client) attachmentID(ctx context.Context, base, filename string) (string, error) {
	q := url.Values{}
	q.Set("filename", filename)
	var r struct {
		Results []rawAttachment `json:"results"`
	}
	if err := c.get(ctx, base+"?"+q.Encode()).Decode(&r); err != nil {
		return "", err
	}
	for _, a := range r.Results {
		if a.Title == filename && a.ID != "" {
			return a.ID, nil
		}
	}
	return "", nil
}

// attachError says which file and which page failed, so a line made of it
// reads on its own in a summary.
func attachError(pageID, filename string, err error) error {
	return fmt.Errorf("attaching %s to page %s: %w", filename, pageID, err)
}

// checkAttachment guards the multipart part headers. Both strings reach them
// verbatim and both come from a caller, so a quote or a line break in either
// would let the caller write headers of its own.
func checkAttachment(filename, contentType string) error {
	if filename == "" || filename == "." || filename == ".." || strings.ContainsRune(filename, '/') {
		return errBadAttachment
	}
	unsafe := func(r rune) bool { return r < ' ' || r == 0x7f || r == '"' || r == '\\' }
	if strings.ContainsFunc(filename, unsafe) || strings.ContainsFunc(contentType, unsafe) {
		return errBadAttachment
	}
	return nil
}

// rawAttachments reads either answer shape: a create answers a results list,
// a replace answers the one attachment.
type rawAttachments struct {
	Results []rawAttachment `json:"results"`
	rawAttachment
}

type rawAttachment struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

func (r rawAttachments) attachment() (Attachment, bool) {
	a := r.rawAttachment
	if len(r.Results) > 0 {
		a = r.Results[0]
	}
	if a.ID == "" {
		return Attachment{}, false
	}
	return Attachment{ID: a.ID, Filename: a.Title}, true
}
