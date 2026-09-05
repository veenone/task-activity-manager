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
	var e struct {
		ErrorMessages []string          `json:"errorMessages"`
		Errors        map[string]string `json:"errors"`
		Error         string            `json:"error"`
		Message       string            `json:"message"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return ""
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
	return strings.Join(parts, "; ")
}

// snippet trims a response body for an error message.
func snippet(b []byte, n int) string {
	s := strings.TrimSpace(string(b))
	if len(s) > n {
		s = s[:n] + "…"
	}
	return s
}
