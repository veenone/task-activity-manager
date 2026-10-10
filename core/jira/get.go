// GET, the read half of the Jira client.
//
// Split out of client.go to keep that file under the 400-line limit (C2).
// The three exported forms differ only in what the caller wants back: a
// decoded object, the raw bytes, or the raw bytes plus the status for callers
// that treat a particular one as data. All three build the request and report
// failure through the same two helpers, so they cannot drift apart (#176).

package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Get performs an authenticated GET and decodes a JSON response into out. Any
// status other than 200 becomes an *HTTPError carrying Jira's message.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	resp, err := c.getRaw(ctx, path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return getError(path, resp, body)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// getRaw performs the authenticated GET both read paths share. It judges
// nothing: the status is the caller's to interpret, because GetBytesStatus
// exists for callers that treat one as data.
func (c *Client) getRaw(ctx context.Context, path string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	return c.Do(req)
}

// getError is how a failed GET is reported, from one place so the two read
// paths cannot drift apart again.
//
// It must be an *HTTPError rather than a formatted string: Humanize finds the
// status with errors.As, and a GET that reported a bare error was silently
// beyond its reach. GetBytes feeds the container, folder and precondition
// syncs, so that was most of the stages a user actually sees fail (#176).
func getError(path string, resp *http.Response, body []byte) *HTTPError {
	// Jira's own words, and nothing when it sent none. Leaving Message empty
	// is deliberate and tested: HTTPError.Error() then falls back to the
	// method/path/status form, which names the endpoint that failed. A
	// snippet of an unrecognisable body in its place would read worse and
	// lose the path (xtm TestHTTPErrorFallsBackWithoutJiraMessage).
	return &HTTPError{
		Method:  http.MethodGet,
		Path:    path,
		Code:    resp.StatusCode,
		Status:  resp.Status,
		Message: jiraErrorMessage(body),
	}
}

// GetBytes performs an authenticated GET and returns the raw body, for
// responses whose shape has to be sniffed before decoding.
func (c *Client) GetBytes(ctx context.Context, path string) ([]byte, error) {
	body, _, err := c.GetBytesStatus(ctx, path)
	return body, err
}

// GetBytesStatus is GetBytes plus the HTTP status code, for callers that treat
// a particular status as data rather than failure. A failure is reported the
// same way Get reports one, as an *HTTPError (#176).
func (c *Client) GetBytesStatus(ctx context.Context, path string) ([]byte, int, error) {
	resp, err := c.getRaw(ctx, path)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, getError(path, resp, body)
	}
	return body, resp.StatusCode, nil
}
