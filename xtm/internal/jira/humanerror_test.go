package jira

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	corejira "agile-suite/core/jira"
)

// The message this was raised for (#170):
//
//	Sync failed: list custom fields: jira: 404 : Issue Does Not Exist
//
// Three things wrong with it. "list custom fields" is an internal step name.
// "jira: 404 :" leaks a status code with stray punctuation. And "Issue Does
// Not Exist" is Jira's generic 404 body, which is about issues when nothing
// in that call asked about an issue, so it sends the reader looking for a
// missing test.
func TestHumanizeReplacesTheGeneric404(t *testing.T) {
	err := fmt.Errorf("list custom fields: %w", &corejira.HTTPError{
		Method: "GET", Path: "/rest/api/2/field", Code: 404,
		Status: "404 ", Message: "Issue Does Not Exist",
	})
	got := Humanize(err)
	if got == "" {
		t.Fatal("no human message for a 404")
	}
	for _, leak := range []string{"list custom fields", "Issue Does Not Exist", "jira:", "404"} {
		if strings.Contains(got, leak) {
			t.Errorf("message still carries %q: %s", leak, got)
		}
	}
	if !strings.Contains(strings.ToLower(got), "jira url") {
		t.Errorf("a 404 should point at the Jira URL or project; got: %s", got)
	}
}

// Credentials are the other common cause and need a different instruction:
// re-entering a token fixes one of these and not the other.
func TestHumanizeSeparatesAuthFromPermission(t *testing.T) {
	unauth := Humanize(&corejira.HTTPError{Code: 401, Status: "401", Message: "Unauthorized"})
	forbidden := Humanize(&corejira.HTTPError{Code: 403, Status: "403", Message: "Forbidden"})
	if unauth == "" || forbidden == "" {
		t.Fatalf("missing message: 401=%q 403=%q", unauth, forbidden)
	}
	if unauth == forbidden {
		t.Errorf("401 and 403 read the same, but only one is fixed by a new token: %s", unauth)
	}
	if !strings.Contains(strings.ToLower(unauth), "token") {
		t.Errorf("a 401 should mention the token; got: %s", unauth)
	}
	if !strings.Contains(strings.ToLower(forbidden), "permission") {
		t.Errorf("a 403 should mention permission; got: %s", forbidden)
	}
}

// A server-side fault is worth telling apart from a mistake in the profile,
// because there is nothing for the user to fix.
func TestHumanizeSaysWhenJiraItselfFailed(t *testing.T) {
	got := Humanize(&corejira.HTTPError{Code: 503, Status: "503", Message: "Service Unavailable"})
	if !strings.Contains(strings.ToLower(got), "jira") {
		t.Fatalf("no mention of Jira: %s", got)
	}
	if strings.Contains(strings.ToLower(got), "check the jira url") {
		t.Errorf("a 503 is not a misconfigured URL, and should not say so: %s", got)
	}
}

// Anything that is not an HTTP failure is left alone. Inventing a sentence
// for an error this does not understand would hide the only description of
// it that exists.
func TestHumanizeLeavesOtherErrorsAlone(t *testing.T) {
	if got := Humanize(errors.New("context canceled")); got != "" {
		t.Errorf("Humanize(%q) = %q, want empty so the caller keeps the original", "context canceled", got)
	}
	if got := Humanize(nil); got != "" {
		t.Errorf("Humanize(nil) = %q, want empty", got)
	}
}
