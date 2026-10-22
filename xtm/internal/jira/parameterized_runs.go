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

// ParameterizedRunInput exposes the deterministic source material the Demo2
// execution builds its run from.
//
// Demo only, and nothing but demo reaches it: a live profile is answered by
// xray.liveParameterizedRun, which reads the run off the instance instead of
// generating one (#334). The demo path still needs the dataset and the step
// definitions because it has no run to read.
func (c *Client) ParameterizedRunInput(ctx context.Context, execKey, testKey string) (ParameterizedRunInput, error) {
	if !isDemoURL(c.baseURL) || execKey != "DEMO2-STE-1" || testKey != "DEMO2-1" {
		return ParameterizedRunInput{}, ErrParameterizedRunUnavailable
	}
	return ParameterizedRunInput{
		Dataset: demoDataset(testKey),
		Steps:   demoStepsForKey(themeFor(c.baseURL), testKey),
	}, nil
}
