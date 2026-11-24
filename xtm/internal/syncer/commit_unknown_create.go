package syncer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/url"
	"strings"
	"time"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/testrepo"
)

// createSkew is how far before a recorded attempt the duplicate check
// starts looking, to absorb the difference between this machine's clock and
// Jira's.
const createSkew = 5 * time.Minute

// createOnce sends one create for a pending row without risking a duplicate
// (#607). Jira has no idempotency key, so a create that got no answer may
// still have made the issue.
//
// prior is the attempt the row recorded on an earlier commit, or nil. When
// set, b is searched first: exactly one issue with the same summary is
// adopted and returned instead of creating, none lets the create go ahead,
// and more than one is an error naming them, because picking one would be a
// guess. A create that then fails with no HTTP response records next on the
// row for the following commit. One that got a status, 4xx or 5xx, records
// nothing: Jira answered, so it made nothing.
//
// A backend that cannot search for created issues keeps the old behaviour:
// the create is sent and nothing is recorded.
func (e *Engine) createOnce(ctx context.Context, profileID string, c testrepo.PendingChange, b backend.Backend, prior *testrepo.CreateAttempt, next testrepo.CreateAttempt, create func() (string, error)) (string, error) {
	finder, ok := b.(backend.CreatedIssueFinder)
	if !ok {
		return create()
	}
	if prior != nil {
		key, err := e.findEarlierCreate(ctx, finder, *prior)
		if err != nil || key != "" {
			return key, err
		}
	}
	next.At = time.Now().UTC().Format(time.RFC3339)
	key, err := create()
	if err != nil {
		if !noResponse(err) {
			return "", err
		}
		if mErr := e.repo.MarkCreateAttempt(profileID, c.ID, next); mErr != nil {
			log.Printf("xtm: record unanswered create for %s: %v", c.EntityKey, mErr)
		}
		return "", fmt.Errorf("no answer from Jira, so the next commit checks whether it was created before trying again: %w", err)
	}
	e.claimCreated(key)
	return key, nil
}

// findEarlierCreate searches for the issue an unanswered create may have
// made. It returns its key when exactly one matches, "" when none does.
func (e *Engine) findEarlierCreate(ctx context.Context, finder backend.CreatedIssueFinder, prior testrepo.CreateAttempt) (string, error) {
	at, err := time.Parse(time.RFC3339, prior.At)
	if err != nil {
		return "", fmt.Errorf("an earlier create got no answer and its recorded time %q is unreadable; not creating it again", prior.At)
	}
	found, err := finder.FindCreatedIssues(ctx, prior.Project, prior.Kind, prior.IssueType, at.Add(-createSkew))
	if err != nil {
		return "", fmt.Errorf("an earlier create got no answer and checking Jira for it failed, so it is not sent again yet: %w", err)
	}
	var matches []string
	for _, f := range found {
		// JQL's summary search is fuzzy, so the comparison is made here.
		if f.Summary == prior.Summary && !e.createdThisRun[f.Key] {
			matches = append(matches, f.Key)
		}
	}
	switch len(matches) {
	case 0:
		return "", nil
	case 1:
		e.claimCreated(matches[0])
		return matches[0], nil
	}
	return "", fmt.Errorf("an earlier create got no answer, and Jira now has %d issues it could have made (%s). Nothing was created: keep one, delete the others, then discard this change",
		len(matches), strings.Join(matches, ", "))
}

// claimCreated notes a key this commit created or adopted, so a later row
// with the same summary cannot adopt it too.
func (e *Engine) claimCreated(key string) {
	if key == "" {
		return
	}
	if e.createdThisRun == nil {
		e.createdThisRun = map[string]bool{}
	}
	e.createdThisRun[key] = true
}

// noResponse reports whether err means the request got no HTTP response, so
// the outcome is unknown: a timeout, a dropped connection, a cancelled
// context. An error that carries a status never matches.
func noResponse(err error) bool {
	var ue *url.Error
	var ne net.Error
	return errors.As(err, &ue) || errors.As(err, &ne) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}
