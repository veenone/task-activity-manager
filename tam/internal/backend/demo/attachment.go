package demo

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"agile-suite/tam/internal/backend"
)

var _ backend.AttachmentBackend = (*Backend)(nil)

// demoAttachment is a file a Commit pushed this run, kept whole so it can be
// saved again. The demo has no server to hold it.
type demoAttachment struct {
	meta backend.Attachment
	data []byte
}

// Attachments answers with whatever this run pushed. The sample seeds none.
func (b *Backend) Attachments(_ context.Context, key string) ([]backend.Attachment, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(key); !ok {
		return nil, fmt.Errorf("demo: no issue %s", key)
	}
	out := []backend.Attachment{}
	for _, a := range b.attachments[key] {
		out = append(out, a.meta)
	}
	return out, nil
}

// AddAttachment reads the file and keeps it, so the section lists it from
// now on and Save can write it back out.
func (b *Backend) AddAttachment(_ context.Context, key string, d backend.AttachmentDraft) error {
	data, err := os.ReadFile(d.Path)
	if err != nil {
		return fmt.Errorf("%s cannot be read: %w", d.Path, err)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(key); !ok {
		return fmt.Errorf("demo: no issue %s", key)
	}
	if b.attachments == nil {
		b.attachments = map[string][]demoAttachment{}
	}
	b.nextAttachment++
	b.attachments[key] = append(b.attachments[key], demoAttachment{
		meta: backend.Attachment{
			ID: strconv.Itoa(b.nextAttachment), Name: d.Name, Author: "demo", AuthorName: "Demo User",
			Created: time.Now().Format("2006-01-02T15:04:05.000-0700"), Size: int64(len(data)),
		},
		data: data,
	})
	return nil
}

// SaveAttachment writes a kept file to dest.
func (b *Backend) SaveAttachment(_ context.Context, key, id, dest string) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, a := range b.attachments[key] {
		if a.meta.ID == id {
			if err := os.WriteFile(dest, a.data, 0o600); err != nil {
				return fmt.Errorf("%s cannot be written: %w", dest, err)
			}
			return nil
		}
	}
	return fmt.Errorf("%s has no attachment %s", key, id)
}
