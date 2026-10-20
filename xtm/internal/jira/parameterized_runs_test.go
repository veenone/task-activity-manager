package jira

import (
	"context"
	"strings"
	"testing"
)

func TestDemo2ParameterizedRunInputCarriesDatasetMacrosAcrossStepFields(t *testing.T) {
	input, err := NewClient("demo", "").ParameterizedRunInput(context.Background(), "DEMO2-STE-1", "DEMO2-1")
	if err != nil {
		t.Fatalf("ParameterizedRunInput: %v", err)
	}
	if len(input.Dataset.Parameters) != 3 || len(input.Dataset.Rows) != 3 {
		t.Fatalf("dataset = %+v, want USERNAME, REGION and PRIORITY rows", input.Dataset)
	}
	joined := ""
	for _, step := range input.Steps {
		joined += step.Action + "\n" + step.Data + "\n" + step.Expected + "\n"
	}
	for _, macro := range []string{"${USERNAME}", "${REGION}", "${PRIORITY}"} {
		if !strings.Contains(joined, macro) {
			t.Errorf("steps are missing %s: %s", macro, joined)
		}
	}
}

func TestParameterizedRunInputRefusesUnknownOrLiveContexts(t *testing.T) {
	for _, tc := range []struct {
		client *Client
		exec   string
		test   string
	}{
		{NewClient("demo", ""), "DEMO2-STE-2", "DEMO2-1"},
		{NewClient("demo", ""), "DEMO2-STE-1", "DEMO2-2"},
		{NewClient("https://jira.example.com", "token"), "DEMO2-STE-1", "DEMO2-1"},
	} {
		if _, err := tc.client.ParameterizedRunInput(context.Background(), tc.exec, tc.test); err == nil {
			t.Errorf("ParameterizedRunInput(%q, %q) succeeded", tc.exec, tc.test)
		}
	}
}
