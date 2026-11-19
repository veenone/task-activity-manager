package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agile-suite/tam/internal/backend"
)

// attachmentBackend is a Jira holding one file. It records every read and
// upload, and writes the file's name where a save asks for it.
type attachmentBackend struct {
	stubIssueBackend
	reads   []string
	uploads []string
}

func (b *attachmentBackend) Attachments(_ context.Context, key string) ([]backend.Attachment, error) {
	b.reads = append(b.reads, key)
	return []backend.Attachment{{ID: "10010", Name: "trace.log", AuthorName: "R. Anand", Size: 2048}}, nil
}

func (b *attachmentBackend) AddAttachment(_ context.Context, key string, d backend.AttachmentDraft) error {
	b.uploads = append(b.uploads, key+" "+d.Name)
	return nil
}

func (b *attachmentBackend) SaveAttachment(_ context.Context, key, id, dest string) error {
	return os.WriteFile(dest, []byte(key+" "+id), 0o600)
}

func seedAttachmentApp(t *testing.T) (*App, string, *attachmentBackend) {
	t.Helper()
	a := newTestApp(t)
	p := newTestProfile(t, a)
	b := &attachmentBackend{}
	a.backends[p.ID] = b
	rows := []backend.Issue{{Key: "PLAT-1", ID: "1", Project: "PLAT", Type: backend.TypeTask, Summary: "one", Status: "To Do", Updated: "2026-09-01T00:00:00Z"}}
	if err := a.repo.UpsertPage(context.Background(), p.ID, rows, time.Now(), false); err != nil {
		t.Fatalf("seed: %v", err)
	}
	return a, p.ID, b
}

func writeFile(t *testing.T, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestAttachFilesJournalsThemAndListsThemAfterJirasOwn(t *testing.T) {
	a, id, b := seedAttachmentApp(t)
	shot := writeFile(t, "shot.png", "12345")
	if err := a.AttachFiles(id, "PLAT-1", []string{shot}); err != nil {
		t.Fatalf("AttachFiles: %v", err)
	}
	if len(b.uploads) != 0 {
		t.Fatalf("uploads = %v, want nothing sent to Jira before Commit", b.uploads)
	}

	files, err := a.ListAttachments(id, "PLAT-1")
	if err != nil {
		t.Fatalf("ListAttachments: %v", err)
	}
	if len(files) != 2 {
		t.Fatalf("files = %+v, want the one Jira holds and the pending one", files)
	}
	if files[0].Name != "trace.log" || files[0].Pending {
		t.Errorf("first = %+v, want the file Jira holds", files[0])
	}
	if got := files[1]; got.Name != "shot.png" || got.Size != 5 || !got.Pending || got.PendingID == 0 || got.Path != shot {
		t.Errorf("second = %+v, want the pending file with its size, path and journal row", got)
	}
}

// Picking the same file twice is one upload, not two.
func TestAttachFilesKeepsOneRowForAFilePickedTwice(t *testing.T) {
	a, id, _ := seedAttachmentApp(t)
	shot := writeFile(t, "shot.png", "12345")
	for range 2 {
		if err := a.AttachFiles(id, "PLAT-1", []string{shot}); err != nil {
			t.Fatalf("AttachFiles: %v", err)
		}
	}
	files, _ := a.ListAttachments(id, "PLAT-1")
	if len(files) != 2 {
		t.Errorf("files = %+v, want the pending file once", files)
	}
}

// A list with one entry that is not a readable file journals none of it.
func TestAttachFilesRefusesAListWithAPathItCannotRead(t *testing.T) {
	a, id, _ := seedAttachmentApp(t)
	good := writeFile(t, "shot.png", "12345")
	for _, bad := range []string{filepath.Join(t.TempDir(), "gone.txt"), t.TempDir()} {
		err := a.AttachFiles(id, "PLAT-1", []string{good, bad})
		if err == nil || !strings.Contains(err.Error(), bad) {
			t.Errorf("AttachFiles(%s) = %v, want a refusal naming it", bad, err)
		}
	}
	files, _ := a.ListAttachments(id, "PLAT-1")
	if len(files) != 1 {
		t.Errorf("files = %+v, want only the one Jira holds", files)
	}
}

func TestSaveAttachmentWritesTheFileTheBackendAnswers(t *testing.T) {
	a, id, _ := seedAttachmentApp(t)
	dest := filepath.Join(t.TempDir(), "saved.log")
	if err := a.saveAttachmentTo(id, "PLAT-1", "10010", dest); err != nil {
		t.Fatalf("save: %v", err)
	}
	got, err := os.ReadFile(dest)
	if err != nil || string(got) != "PLAT-1 10010" {
		t.Errorf("saved %q, %v", got, err)
	}
}

func TestListAttachmentsOnADraftNeverAsksJira(t *testing.T) {
	a, id, b := seedAttachmentApp(t)
	key, err := a.repo.CreateDraft(context.Background(), id, "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "new one"})
	if err != nil {
		t.Fatalf("draft: %v", err)
	}
	if err := a.AttachFiles(id, key, []string{writeFile(t, "shot.png", "1")}); err != nil {
		t.Fatalf("AttachFiles: %v", err)
	}
	files, err := a.ListAttachments(id, key)
	if err != nil || len(files) != 1 || !files[0].Pending {
		t.Errorf("files = %+v, %v, want the journalled file alone", files, err)
	}
	if len(b.reads) != 0 {
		t.Errorf("reads = %v, want Jira left alone for a draft", b.reads)
	}
}
