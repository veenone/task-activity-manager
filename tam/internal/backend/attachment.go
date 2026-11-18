package backend

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
)

// Attachment is one file on an issue, as the panel reads it.
//
// Pending marks a file the journal holds and Commit has not uploaded, and
// PendingID is its journal row, for Discard. They mirror Worklog's pair for
// the same reason: the section draws Jira's files and TAM's own in one list,
// and only the second kind can be taken back. Path is where a pending file
// sits on this machine; it is empty for a file Jira holds.
type Attachment struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Author     string `json:"author"`
	AuthorName string `json:"authorName"`
	Created    string `json:"created"`
	Size       int64  `json:"size"`
	MimeType   string `json:"mimeType"`
	Path       string `json:"path"`
	Pending    bool   `json:"pending"`
	PendingID  int64  `json:"pendingId"`
}

// AttachmentDraft is a file TAM holds in the journal until Commit uploads
// it. The journal keeps where the file is, not the file: copying every
// picked file into the store would double the disk it takes and leave the
// copy behind after a discard. The cost is that the file has to still be at
// Path when Commit runs, which Commit checks and reports.
type AttachmentDraft struct {
	Path string `json:"path"`
	Name string `json:"name"`
	Size int64  `json:"size"`
}

// AttachmentBackend is the attachment calls. It is an optional interface, the
// way WorklogBackend is: a backend that cannot answer it says so once, and
// the section and the Commit phase report that.
type AttachmentBackend interface {
	// Attachments is every file Jira holds for the issue.
	Attachments(ctx context.Context, key string) ([]Attachment, error)
	// AddAttachment uploads the draft's file to the issue.
	AddAttachment(ctx context.Context, key string, d AttachmentDraft) error
	// SaveAttachment writes the attachment with this id, on this issue, to
	// dest. It takes the id rather than a link so that what the window sends
	// is checked against what Jira lists for the issue.
	SaveAttachment(ctx context.Context, key, id, dest string) error
}

// NewAttachmentDraft builds the journal payload for the file at path. The
// path arrives from the window, so it is checked here: it has to name a
// regular file that can be read now. A folder, or a file that has already
// gone, is refused where it was picked rather than at Commit.
func NewAttachmentDraft(path string) (AttachmentDraft, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return AttachmentDraft{}, fmt.Errorf("%q is not a path TAM can read", path)
	}
	info, err := os.Stat(abs)
	if err != nil {
		return AttachmentDraft{}, fmt.Errorf("%s cannot be read: %w", abs, err)
	}
	if !info.Mode().IsRegular() {
		return AttachmentDraft{}, fmt.Errorf("%s is not a file", abs)
	}
	return AttachmentDraft{Path: abs, Name: filepath.Base(abs), Size: info.Size()}, nil
}
