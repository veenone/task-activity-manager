package committer

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"net/url"
	"sort"
	"strings"
	"time"

	"agile-suite/core/journal"
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/issuerepo"
)

// A create POST that times out may still have made the issue, and Jira has
// no idempotency key, so posting the draft again can make a second copy.
// Commit records such a create on the draft (backend.CreateAttempt) and, on
// the next Commit, searches before posting: one match is adopted the way a
// successful create is, none is created as normal, and more than one holds
// the draft, because picking one would be a guess. A create Jira answered
// with an HTTP error made nothing and is retried as before.

// lostCreateMargin widens the search back past the attempt, for a clock on
// this machine that runs ahead of Jira's.
const lostCreateMargin = 5 * time.Minute

// noAnswer says the request failed before any HTTP status came back, so
// whether Jira acted on it is unknown. http.Client wraps every transport
// failure in *url.Error; the rest cover a body read cut short and a context
// that ended mid-request.
func noAnswer(err error) bool {
	var ue *url.Error
	var ne net.Error
	return errors.As(err, &ue) || errors.As(err, &ne) ||
		errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) ||
		errors.Is(err, io.ErrUnexpectedEOF)
}

// createFailed reports a create that did not return a key. One with no HTTP
// answer is recorded on the draft so the next Commit searches first; one
// Jira refused clears an earlier record, since the search that preceded it
// found nothing and Jira has now answered.
func (e *Engine) createFailed(ctx context.Context, r *commitRun, tempKey string, d backend.IssueDraft, err error) {
	if !noAnswer(err) {
		if d.LostCreate != nil {
			if cerr := e.repo.SetLostCreate(ctx, r.profileID, tempKey, nil); cerr != nil {
				log.Printf("tam: clear the lost create of %s: %v", tempKey, cerr)
			}
		}
		r.res.Failures = append(r.res.Failures, failure(tempKey, issuerepo.EntityIssueCreate, err.Error(), true))
		r.deps.block(tempKey, tempKey, "which Jira refused")
		return
	}
	msg := err.Error() + ". Jira did not answer, so it may have made the issue; the next Commit looks for it before creating it again."
	a := &backend.CreateAttempt{At: time.Now().UTC().Format(time.RFC3339), Summary: d.Summary, Type: d.Type}
	if serr := e.repo.SetLostCreate(ctx, r.profileID, tempKey, a); serr != nil {
		msg = err.Error() + ". Jira did not answer, so it may have made the issue, and TAM could not record that: check Jira for it before you Commit again (" + serr.Error() + ")."
	}
	r.res.Failures = append(r.res.Failures, failure(tempKey, issuerepo.EntityIssueCreate, msg, true))
	r.deps.block(tempKey, tempKey, "whose create got no answer from Jira")
}

// findLostCreate looks for the issue an earlier create with no answer may
// have made. It returns the key to adopt, or "" to create as normal; stop is
// true when it has already reported the draft as failed.
func (e *Engine) findLostCreate(ctx context.Context, r *commitRun, row journal.PendingChange, a backend.CreateAttempt) (key string, stop bool) {
	tempKey := row.EntityKey
	keys, err := e.lostCreateMatches(ctx, r.projectKey, a)
	if err != nil {
		r.res.Failures = append(r.res.Failures, failure(tempKey, issuerepo.EntityIssueCreate, "the last create of this draft got no answer from Jira, and the search for the issue it may have made failed, so it was not sent again: "+err.Error(), true))
		r.deps.block(tempKey, tempKey, "whose create got no answer from Jira")
		return "", true
	}
	switch len(keys) {
	case 0:
		return "", false
	case 1:
		return keys[0], false
	}
	r.res.Failures = append(r.res.Failures, failure(tempKey, issuerepo.EntityIssueCreate, fmt.Sprintf("the last create of this draft got no answer from Jira, and %d issues you reported since then match its summary and type: %s. TAM cannot tell which one it made; delete the extra ones in Jira and Commit again, or discard the draft.", len(keys), strings.Join(keys, ", ")), false))
	r.deps.block(tempKey, tempKey, "which may already exist in Jira more than once")
	return "", true
}

// lostCreateMatches lists the keys of the issues the current user reported
// in the project since the attempt whose summary and type are what it sent.
// The window is relative ("-Nm") because a JQL date literal is read in the
// Jira user's time zone, which TAM does not know. The summary is compared
// here and not in JQL, where "~" is a fuzzy text match.
func (e *Engine) lostCreateMatches(ctx context.Context, projectKey string, a backend.CreateAttempt) ([]string, error) {
	at, err := time.Parse(time.RFC3339, a.At)
	if err != nil {
		return nil, fmt.Errorf("the attempt's time %q: %w", a.At, err)
	}
	minutes := int(math.Ceil(time.Since(at).Minutes()))
	if minutes < 0 {
		minutes = 0
	}
	minutes += int(lostCreateMargin.Minutes())
	scope := fmt.Sprintf(`reporter = currentUser() AND created >= "-%dm"`, minutes)
	keys := []string{}
	for start := 0; ; {
		page, total, err := e.b.SearchIssuesPage(ctx, projectKey, scope, "", start, 100)
		if err != nil {
			return nil, err
		}
		for _, iss := range page {
			if strings.TrimSpace(iss.Summary) == strings.TrimSpace(a.Summary) && strings.EqualFold(iss.Type, a.Type) {
				keys = append(keys, iss.Key)
			}
		}
		start += len(page)
		if len(page) == 0 || start >= total {
			break
		}
	}
	sort.Strings(keys)
	return keys, nil
}
