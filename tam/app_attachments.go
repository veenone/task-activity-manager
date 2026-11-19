package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
	"agile-suite/tam/internal/permission"
)

// ListAttachments is what the Attachments section shows: the files Jira
// holds for the issue, then the ones the journal holds. Like the work log it
// is read when the section is expanded and nothing is cached.
//
// A draft has no issue in Jira to read, so only its journalled files come
// back.
func (a *App) ListAttachments(profileID, key string) ([]backend.Attachment, error) {
	key = strings.TrimSpace(key)
	if key == "" {
		return nil, errors.New("issue key is empty")
	}
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return nil, err
	}
	files := []backend.Attachment{}
	if !strings.HasPrefix(key, issuerepo.DraftPrefix) {
		ab, ok := b.(backend.AttachmentBackend)
		if !ok {
			return nil, errors.New("this connection cannot read attachments")
		}
		remote, err := ab.Attachments(a.ctx, key)
		if err != nil {
			return nil, err
		}
		files = append(files, remote...)
	}
	pending, err := a.repo.PendingAttachments(a.ctx, p.ID, key)
	if err != nil {
		return nil, err
	}
	return append(files, pending...), nil
}

// PickAttachmentFiles opens the system file dialog and answers the paths
// chosen, none when it was cancelled. It is its own call so that AttachFiles
// takes paths and can be exercised without a window.
func (a *App) PickAttachmentFiles() ([]string, error) {
	paths, err := runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{Title: "Attach files"})
	if err != nil {
		return nil, fmt.Errorf("file dialog: %w", err)
	}
	if paths == nil {
		paths = []string{}
	}
	return paths, nil
}

// AttachFiles journals each file against the issue. Nothing reaches Jira
// here; Commit uploads them. Every path is checked before any is journalled,
// so a list with one bad entry journals none of it and the user picks again
// knowing what was refused.
func (a *App) AttachFiles(profileID, key string, paths []string) error {
	p, err := a.requireProfile(profileID)
	if err != nil {
		return err
	}
	if err := a.allowOnIssue(p.ID, key, permission.CreateAttachments); err != nil {
		return err
	}
	drafts := make([]backend.AttachmentDraft, 0, len(paths))
	for _, path := range paths {
		d, err := backend.NewAttachmentDraft(path)
		if err != nil {
			return err
		}
		drafts = append(drafts, d)
	}
	for _, d := range drafts {
		if err := a.repo.AttachFile(a.ctx, p.ID, strings.TrimSpace(key), d); err != nil {
			return err
		}
	}
	return nil
}

// SaveAttachment asks where to put one of the issue's attachments and
// downloads it there. It answers the path written, or "" when the dialog was
// cancelled. name only seeds the dialog; the file is found by its id.
func (a *App) SaveAttachment(profileID, key, id, name string) (string, error) {
	dest, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Save attachment",
		DefaultFilename: filepath.Base(name),
	})
	if err != nil {
		return "", fmt.Errorf("save dialog: %w", err)
	}
	if dest == "" {
		return "", nil
	}
	return dest, a.saveAttachmentTo(profileID, key, id, dest)
}

// saveAttachmentTo is SaveAttachment past the dialog.
func (a *App) saveAttachmentTo(profileID, key, id, dest string) error {
	_, b, err := a.backendForProfile(profileID)
	if err != nil {
		return err
	}
	ab, ok := b.(backend.AttachmentBackend)
	if !ok {
		return errors.New("this connection cannot read attachments")
	}
	return ab.SaveAttachment(a.ctx, strings.TrimSpace(key), id, dest)
}
