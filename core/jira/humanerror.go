package jira

import (
	"errors"
	"net/http"
)

// Humanize turns a Jira HTTP failure into a sentence a user can act on, or
// returns "" when it has nothing better to say than the error already does.
//
// The raw text is built for a log, not a person. A failed sync read came out
// as:
//
//	Sync failed: list custom fields: jira: 404 : Issue Does Not Exist
//
// which names an internal step, leaks a status code with stray punctuation,
// and ends with Jira's generic 404 body. That last part is the worst of it:
// nothing in that call asked about an issue, so the reader goes looking for a
// missing test instead of a wrong URL (#170).
//
// Each sentence says what Jira did and what to do about it, in that order,
// and talks to the reader rather than about the app: "the address XTM asked
// for" is not how a person describes a wrong URL (#313).
//
// Only the status is used. Jira's own message is deliberately dropped,
// because the cases where it is specific enough to help are the cases where
// the status already says the same thing, and the cases where it is wrong
// are the ones that mislead. An error that is not an HTTP failure is left
// alone: inventing a sentence for something this does not understand would
// hide the only description of it there is.
//
// This lives beside HTTPError rather than in one app because both apps show
// the same failures. It was XTM's until #315; TAM, which cannot import
// xtm/internal, showed the raw text all along. No sentence here names an app,
// which TestHumanizeNamesNoApp holds.
func Humanize(err error) string {
	if err == nil {
		return ""
	}
	var he *HTTPError
	if !errors.As(err, &he) {
		return ""
	}
	switch {
	case he.Code == http.StatusUnauthorized:
		return "Jira rejected this profile's token. It may have expired or been revoked, so re-enter it in the profile's connection settings."
	case he.Code == http.StatusForbidden:
		return "The token works, but this account has no permission for that project in Jira. A Jira administrator can grant it."
	case he.Code == http.StatusNotFound:
		return "Jira found nothing at that address. Check the Jira URL and the project key in the profile's connection settings."
	case he.Code == http.StatusRequestTimeout, he.Code == http.StatusTooManyRequests:
		return "Jira is busy and is turning requests away. Wait a moment and sync again."
	case he.Code >= 500:
		return "Jira failed on its own side. Nothing in the profile needs changing. Sync again, and if it keeps failing the instance may be down."
	}
	return ""
}
