package jira

import (
	"context"
	"fmt"
	"os"

	"agile-suite/tam/internal/backend"
)

var _ backend.AttachmentBackend = (*Backend)(nil)

// Attachments reads the files Jira holds for the issue.
func (b *Backend) Attachments(ctx context.Context, key string) ([]backend.Attachment, error) {
	raw, err := b.c.Attachments(ctx, key)
	if err != nil {
		return nil, err
	}
	out := make([]backend.Attachment, 0, len(raw))
	for _, a := range raw {
		out = append(out, backend.Attachment{
			ID: a.ID, Name: a.Filename, Author: a.Author.Name, AuthorName: a.Author.DisplayName,
			Created: a.Created, Size: a.Size, MimeType: a.MimeType,
		})
	}
	return out, nil
}

// AddAttachment uploads the draft's file under the name it was picked with.
func (b *Backend) AddAttachment(ctx context.Context, key string, d backend.AttachmentDraft) error {
	f, err := os.Open(d.Path)
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w", d.Path, err)
	}
	defer f.Close()
	return b.c.AddAttachment(ctx, key, d.Name, f)
}

// SaveAttachment downloads one of the issue's attachments to dest.
//
// The issue's list is read again and the id looked up in it, so the link
// that is followed is the one Jira gives for this issue now, and an id that
// is not on the issue is refused. A download that fails part way removes
// what it wrote: half a file under the right name is worse than none.
func (b *Backend) SaveAttachment(ctx context.Context, key, id, dest string) error {
	raw, err := b.c.Attachments(ctx, key)
	if err != nil {
		return err
	}
	for _, a := range raw {
		if a.ID != id {
			continue
		}
		f, err := os.Create(dest)
		if err != nil {
			return fmt.Errorf("%s cannot be written: %w", dest, err)
		}
		err = b.c.DownloadAttachment(ctx, a.Content, f)
		if cerr := f.Close(); err == nil {
			err = cerr
		}
		if err != nil {
			_ = os.Remove(dest)
		}
		return err
	}
	return fmt.Errorf("%s has no attachment %s", key, id)
}
