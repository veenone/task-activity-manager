package jira

import (
	"context"
	"errors"
)

var ErrParameterizedRunUnavailable = errors.New("parameterized run detail is unavailable")

type ParameterizedRunInput struct {
	Dataset Dataset
	Steps   []Step
}

// ParameterizedRunInput exposes the deterministic source material used by the
// Demo2 execution. Live Xray stays unavailable until its result transport is
// captured and tested.
func (c *Client) ParameterizedRunInput(ctx context.Context, execKey, testKey string) (ParameterizedRunInput, error) {
	if !isDemoURL(c.baseURL) || execKey != "DEMO2-STE-1" || testKey != "DEMO2-1" {
		return ParameterizedRunInput{}, ErrParameterizedRunUnavailable
	}
	return ParameterizedRunInput{
		Dataset: demoDataset(testKey),
		Steps:   demoStepsForKey(themeFor(c.baseURL), testKey),
	}, nil
}
