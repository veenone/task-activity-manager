// Package demo answers Jira searches from invented issues, so PIE can be shown
// and tested with no Jira and no token. It sits under core's client as an
// http.RoundTripper: query.Run pages and parses demo data with the same code
// it uses on a real instance, and nothing above the client knows the
// difference.
package demo

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	corejira "agile-suite/core/jira"
)

// Is reports whether a profile's Jira URL selects demo data.
func Is(jiraURL string) bool {
	return strings.EqualFold(strings.TrimSpace(jiraURL), "demo")
}

// Client is a Jira client whose every request is answered here. The base URL
// cannot resolve, so a request that somehow bypassed the transport would fail
// rather than reach a real server.
func Client() *corejira.Client {
	return corejira.NewClientWithHTTP("http://demo.invalid", "", &http.Client{Transport: transport{}})
}

// refusedFilter is the saved filter demo treats as one the account cannot see.
const refusedFilter = "99999"

var (
	keyIn       = regexp.MustCompile(`(?i)\bkey\s+in\s*\(([^)]*)\)`)
	refusedJQL  = regexp.MustCompile(`(?i)\bfilter\s*=\s*` + refusedFilter + `\b`)
	refusedBody = `{"errorMessages":["A value with ID '` + refusedFilter + `' does not exist for the field 'filter'."],"errors":{}}`
)

type transport struct{}

// RoundTrip answers /rest/api/2/search and the field list, and nothing else.
// The JQL is ignored apart from key in (...) and the refused filter: demo has
// no query engine.
func (transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Path == "/rest/api/2/field" {
		return reply(req, http.StatusOK, `[{"id":"`+baseField+`","name":"Product base","custom":true}]`), nil
	}
	if req.URL.Path != "/rest/api/2/search" {
		return reply(req, http.StatusNotFound, `{"errorMessages":["demo answers searches only"]}`), nil
	}
	q := req.URL.Query()
	jql := q.Get("jql")
	if refusedJQL.MatchString(jql) {
		return reply(req, http.StatusBadRequest, refusedBody), nil
	}
	issues := search(jql, time.Now())
	startAt, _ := strconv.Atoi(q.Get("startAt"))
	maxResults, err := strconv.Atoi(q.Get("maxResults"))
	if err != nil || maxResults <= 0 {
		maxResults = 50
	}
	startAt = min(max(startAt, 0), len(issues))
	end := min(startAt+maxResults, len(issues))
	body, err := json.Marshal(map[string]any{
		"startAt": startAt, "maxResults": maxResults, "total": len(issues), "issues": issues[startAt:end],
	})
	if err != nil {
		return nil, err
	}
	return reply(req, http.StatusOK, string(body)), nil
}

// search is the seed a query sees: the keys a key in (...) names, in seed
// order, or every issue the seed lists to a plain query.
func search(jql string, now time.Time) []corejira.RawIssue {
	all := render(now)
	m := keyIn.FindStringSubmatch(jql)
	var out []corejira.RawIssue
	if m == nil {
		for _, is := range all {
			if !linkOnly[is.Key] {
				out = append(out, is)
			}
		}
		return out
	}
	want := map[string]bool{}
	for k := range strings.SplitSeq(m[1], ",") {
		want[strings.ToUpper(strings.Trim(strings.TrimSpace(k), `"'`))] = true
	}
	for _, is := range all {
		if want[is.Key] {
			out = append(out, is)
		}
	}
	return out
}

func reply(req *http.Request, code int, body string) *http.Response {
	return &http.Response{
		StatusCode: code,
		Status:     strconv.Itoa(code) + " " + http.StatusText(code),
		Header:     http.Header{"Content-Type": []string{"application/json"}},
		Body:       io.NopCloser(bytes.NewBufferString(body)),
		Request:    req,
	}
}
