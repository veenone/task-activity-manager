package committer_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/committer"
	"agile-suite/tam/internal/issuerepo"
)

// attachJira is the fake Jira with the attachment calls on it. It records
// "KEY name size" for every upload and refuses the names in refuse.
type attachJira struct {
	*fake
	uploaded []string
	refuse   map[string]error
}

func (a *attachJira) Attachments(context.Context, string) ([]backend.Attachment, error) {
	return nil, errors.New("not used")
}

func (a *attachJira) AddAttachment(_ context.Context, key string, d backend.AttachmentDraft) error {
	if err := a.refuse[d.Name]; err != nil {
		return err
	}
	data, err := os.ReadFile(d.Path)
	if err != nil {
		return err
	}
	a.uploaded = append(a.uploaded, key+" "+d.Name+" "+string(data))
	return nil
}

func (a *attachJira) SaveAttachment(context.Context, string, string, string) error {
	return errors.New("not used")
}

func attachSetup(t *testing.T) (*committer.Engine, *issuerepo.Repository, *attachJira) {
	t.Helper()
	h := newHarness(t)
	j := &attachJira{fake: h.jira, refuse: map[string]error{}}
	return committer.New(j, h.repo, h.order), h.repo, j
}

// attach writes a file and journals it against key.
func attach(t *testing.T, repo *issuerepo.Repository, key, name, body string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	d, err := backend.NewAttachmentDraft(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.AttachFile(context.Background(), "p1", key, d); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestCommitUploadsJournalledAttachments(t *testing.T) {
	eng, repo, j := attachSetup(t)
	ctx := context.Background()
	attach(t, repo, "PLAT-1", "trace.log", "line one")

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(j.uploaded) != 1 || j.uploaded[0] != "PLAT-1 trace.log line one" {
		t.Fatalf("uploaded = %v, want the file under its own name", j.uploaded)
	}
	if len(res.Attached) != 1 || res.Attached[0].Key != "PLAT-1" || res.Attached[0].Name != "trace.log" {
		t.Errorf("result = %+v, want the file named", res.Attached)
	}
	if len(res.Failures) != 0 || res.Remaining != 0 {
		t.Errorf("failures = %+v, remaining = %d", res.Failures, res.Remaining)
	}
	// An attachment is not a field edit: nothing may have been sent as one.
	if len(j.updates) != 0 {
		t.Errorf("updates = %v, want none", j.updates)
	}
	left, _ := repo.PendingAttachments(ctx, "p1", "PLAT-1")
	if len(left) != 0 {
		t.Errorf("journal still holds %+v after the upload", left)
	}
}

// One refused file keeps its row and names what Jira said, and the others
// still go (I2).
func TestCommitKeepsTheOtherFilesWhenOneAttachmentFails(t *testing.T) {
	eng, repo, j := attachSetup(t)
	ctx := context.Background()
	attach(t, repo, "PLAT-1", "big.bin", "x")
	attach(t, repo, "PLAT-2", "notes.txt", "y")
	j.refuse["big.bin"] = errors.New("the file is larger than this instance allows")

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(j.uploaded) != 1 || !strings.HasPrefix(j.uploaded[0], "PLAT-2 notes.txt") {
		t.Errorf("uploaded = %v, want the file that was not refused", j.uploaded)
	}
	if len(res.Failures) != 1 || res.Failures[0].Key != "PLAT-1" || !res.Failures[0].Retryable ||
		res.Failures[0].EntityType != issuerepo.EntityAttachment || !strings.Contains(res.Failures[0].Error, "larger than this instance allows") {
		t.Fatalf("failures = %+v, want the refused file with the reason", res.Failures)
	}
	left, _ := repo.PendingAttachments(ctx, "p1", "PLAT-1")
	if len(left) != 1 || left[0].PendingID != res.Failures[0].RowID {
		t.Errorf("journal = %+v, want the refused file kept under the row the failure names", left)
	}
}

// The journal holds a path. A file that has gone by Commit is said to be
// gone, is not offered for a retry that cannot work, and is never sent.
func TestCommitSaysWhenAnAttachedFileHasGone(t *testing.T) {
	eng, repo, j := attachSetup(t)
	path := attach(t, repo, "PLAT-1", "trace.log", "line one")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}

	res, err := eng.Commit(context.Background(), "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(j.uploaded) != 0 {
		t.Errorf("uploaded = %v, want nothing", j.uploaded)
	}
	if len(res.Failures) != 1 || res.Failures[0].Retryable || !strings.Contains(res.Failures[0].Error, "no longer at "+path) {
		t.Fatalf("failures = %+v, want the missing file named and not retryable", res.Failures)
	}
}

// A file attached to a draft goes up under the key the create gave it.
func TestCommitAttachesToTheKeyACreateJustGave(t *testing.T) {
	eng, repo, j := attachSetup(t)
	ctx := context.Background()
	draft, err := repo.CreateDraft(ctx, "p1", "PLAT", backend.IssueDraft{Type: backend.TypeTask, Summary: "new one"})
	if err != nil {
		t.Fatal(err)
	}
	attach(t, repo, draft, "shot.png", "png")

	res, err := eng.Commit(ctx, "p1", "PLAT")
	if err != nil {
		t.Fatalf("commit: %v", err)
	}
	if len(res.Created) != 1 || len(res.Attached) != 1 || res.Attached[0].Key != res.Created[0].Key {
		t.Fatalf("created = %+v, attached = %+v, want the file on the new key", res.Created, res.Attached)
	}
	if len(j.uploaded) != 1 || !strings.HasPrefix(j.uploaded[0], res.Created[0].Key+" shot.png") {
		t.Errorf("uploaded = %v", j.uploaded)
	}
}
