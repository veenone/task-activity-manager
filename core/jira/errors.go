package jira

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// jiraErrorMessage pulls the readable parts out of a Jira error body:
// errorMessages, then the errors map in key order, then error and message.
func jiraErrorMessage(body []byte) string {
	msg, _, _ := parseJiraError(body)
	return msg
}

// parseJiraError reads a Jira error body three ways at once: the flattened
// sentence jiraErrorMessage has always produced, the free-standing messages
// on their own, and the errors map keyed by field id. The last two are what
// a caller holding the instance's field names needs to say which field a
// refusal is about; this package has the ids and not the names, so it keeps
// them apart rather than guessing.
func parseJiraError(body []byte) (msg string, messages []string, fields map[string]string) {
	var e struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
		Error         string            `json:"error"`
		Message       string            `json:"message"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return "", nil, nil
	}
	messages = append([]string{}, e.ErrorMessages...)
	if e.Error != "" {
		messages = append(messages, e.Error)
	}
	if e.Message != "" {
		messages = append(messages, e.Message)
	}
	parts := append([]string{}, e.ErrorMessages...)
	keys := make([]string, 0, len(e.Errors))
	for k := range e.Errors {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		parts = append(parts, fmt.Sprintf("%s: %s", k, e.Errors[k]))
	}
	if e.Error != "" {
		parts = append(parts, e.Error)
	}
	if e.Message != "" {
		parts = append(parts, e.Message)
	}
	return strings.Join(parts, "; "), messages, e.Errors
}

// snippet trims a response body for an error message.
func snippet(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}
