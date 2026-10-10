package jira

import (
	"errors"
	"net/http"

	corejira "agile-suite/core/jira"
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
// Only the status is used. Jira's own message is deliberately dropped,
// because the cases where it is specific enough to help are the cases where
// the status already says the same thing, and the cases where it is wrong
// are the ones that mislead. An error that is not an HTTP failure is left
// alone: inventing a sentence for something this does not understand would
// hide the only description of it there is.
func Humanize(err error) string {
	if err == nil {
		return ""
	}
	var he *corejira.HTTPError
	if !errors.As(err, &he) {
		return ""
	}
	switch {
	case he.Code == http.StatusUnauthorized:
		return "Jira did not accept this profile's token. Check the token in the profile's connection settings; it may have expired or been revoked."
	case he.Code == http.StatusForbidden:
		return "Jira accepted the token but refused the request. The account needs permission for this project in Jira."
	case he.Code == http.StatusNotFound:
		return "Jira has nothing at the address XTM asked for. Check the Jira URL and the project key in the profile's connection settings."
	case he.Code == http.StatusRequestTimeout, he.Code == http.StatusTooManyRequests:
		return "Jira is busy and asked XTM to slow down. Try again in a moment."
	case he.Code >= 500:
		return "Jira reported a problem on its own side. Nothing here needs changing; try again, and if it keeps happening the instance may be down."
	}
	return ""
}
