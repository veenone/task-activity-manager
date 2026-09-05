// Package jira is the transport the suite's apps share for talking to Jira
// Data Center: personal access token auth, TLS options, paced requests, the
// JSON helpers, and the handful of generic issue reads both apps need. It
// targets Jira DC 8.14+ and /rest/api/2/. Everything Xray-specific lives in
// XTM, which embeds this client.
package jira

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// Request pacing: a token-bucket limiter governs every outbound call (see
// Client.Do), so concurrent per-item fetches stay within a safe rate.
const (
	reqPerSec = 20 // sustained requests per second across all calls on one client
	burst     = 10 // allowed burst above the sustained rate
)

// Client talks to a single Jira Data Center instance.
type Client struct {
	baseURL string
	token   string
	http    *http.Client
	// limiter paces every outbound request. Shared across goroutines so
	// concurrent fetches respect one global rate.
	limiter *rate.Limiter

	// fieldMu guards fieldIDs, the per-instance cache of custom field ids by
	// lower-cased name, filled from one /rest/api/2/field fetch.
	fieldMu      sync.Mutex
	fieldIDs     map[string]string
	fieldsLoaded bool
}

// User is the subset of /rest/api/2/myself the apps need to confirm a
// connection.
type User struct {
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
	Email       string `json:"emailAddress"`
}

// HTTPError carries the HTTP status of a failed request so callers can treat
// specific statuses as soft failures.
type HTTPError struct {
	Method  string
	Path    string
	Code    int
	Status  string
	Message string // readable Jira error, parsed from the response body
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("jira: %s: %s", e.Status, e.Message)
	}
	return fmt.Sprintf("jira: %s %s -> %s", e.Method, e.Path, e.Status)
}

// clientConfig holds optional TLS settings.
type clientConfig struct {
	caCertPEM string
	insecure  bool
}

// Option is a functional option for NewClient.
type Option func(*clientConfig)

// WithCACert adds a PEM-encoded CA certificate to the TLS trust pool. The
// system pool is the base, so existing roots are preserved. An empty or
// unparseable PEM adds nothing.
func WithCACert(pem string) Option {
	return func(c *clientConfig) { c.caCertPEM = pem }
}

// WithInsecureTLS disables TLS certificate verification, hostname checks
// included. Only for trusted internal servers with no CA certificate.
func WithInsecureTLS(b bool) Option {
	return func(c *clientConfig) { c.insecure = b }
}

// buildHTTPClient returns the plain 30 second client when no TLS option is
// set, otherwise a clone of the default transport with a custom TLS config so
// pooling and keep-alives are preserved.
func buildHTTPClient(cfg clientConfig) *http.Client {
	if cfg.caCertPEM == "" && !cfg.insecure {
		return &http.Client{Timeout: 30 * time.Second}
	}

	pool, err := x509.SystemCertPool()
	if err != nil || pool == nil {
		pool = x509.NewCertPool()
	}
	if cfg.caCertPEM != "" {
		pool.AppendCertsFromPEM([]byte(cfg.caCertPEM))
	}

	base := http.DefaultTransport.(*http.Transport).Clone()
	base.TLSClientConfig = &tls.Config{
		RootCAs:            pool,
		InsecureSkipVerify: cfg.insecure, //nolint:gosec // user-controlled escape hatch
	}
	return &http.Client{Timeout: 30 * time.Second, Transport: base}
}

// NewClient builds a client for baseURL (the instance root, for example
// https://jira.example.com) authenticated with a personal access token.
func NewClient(baseURL, token string, opts ...Option) *Client {
	var cfg clientConfig
	for _, o := range opts {
		o(&cfg)
	}
	return NewClientWithHTTP(baseURL, token, buildHTTPClient(cfg))
}

// NewClientWithHTTP builds a client on a caller-supplied http.Client, which
// tests use to point at an httptest server. A nil h uses the default client.
func NewClientWithHTTP(baseURL, token string, h *http.Client) *Client {
	if h == nil {
		h = buildHTTPClient(clientConfig{})
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    h,
		limiter: rate.NewLimiter(reqPerSec, burst),
	}
}

// BaseURL is the instance root with any trailing slash removed.
func (c *Client) BaseURL() string { return c.baseURL }

// Do sends a request after waiting for a slot from the rate limiter. It is
// the single throttle point every helper goes through. A nil limiter is a
// no-op so hand-built clients still work.
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	if c.limiter != nil {
		if err := c.limiter.Wait(req.Context()); err != nil {
			return nil, err
		}
	}
	return c.http.Do(req)
}

// Myself fetches the authenticated user, which is the connection test.
func (c *Client) Myself(ctx context.Context) (User, error) {
	var u User
	if err := c.Get(ctx, "/rest/api/2/myself", &u); err != nil {
		return User{}, err
	}
	return u, nil
}

// Get performs an authenticated GET and decodes a JSON response into out. Any
// status other than 200 becomes an *HTTPError carrying Jira's message.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		msg := jiraErrorMessage(body)
		return &HTTPError{Method: http.MethodGet, Path: path, Code: resp.StatusCode, Status: resp.Status, Message: msg}
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// GetBytes performs an authenticated GET and returns the raw body, for
// responses whose shape has to be sniffed before decoding.
func (c *Client) GetBytes(ctx context.Context, path string) ([]byte, error) {
	body, _, err := c.GetBytesStatus(ctx, path)
	return body, err
}

// GetBytesStatus is GetBytes plus the HTTP status code, for callers that treat
// a particular status as data rather than failure. The error text stays what
// GetBytes has always produced.
func (c *Client) GetBytesStatus(ctx context.Context, path string) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("jira: GET %s -> %s: %s", path, resp.Status, snippet(body, 1024))
	}
	return body, resp.StatusCode, nil
}

// Put performs an authenticated JSON PUT.
func (c *Client) Put(ctx context.Context, path string, body any) error {
	return c.WriteJSON(ctx, http.MethodPut, path, body)
}

// Post performs an authenticated JSON POST.
func (c *Client) Post(ctx context.Context, path string, body any) error {
	return c.WriteJSON(ctx, http.MethodPost, path, body)
}

// Delete performs an authenticated DELETE with no body. Any 2xx is success.
func (c *Client) Delete(ctx context.Context, path string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf(
			"jira: DELETE %s -> %s: %s",
			path, resp.Status, strings.TrimSpace(string(respBody)),
		)
	}
	return nil
}

// WriteJSON marshals body and sends it with method, discarding the response.
func (c *Client) WriteJSON(ctx context.Context, method, path string, body any) error {
	return c.WriteJSONReturning(ctx, method, path, body, nil)
}

// WriteJSONReturning marshals body, sends it with method, and decodes a 2xx
// response into out when out is non-nil and a body is present. A non-2xx
// status returns an error carrying a short slice of the response body.
func (c *Client) WriteJSONReturning(ctx context.Context, method, path string, body, out any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return fmt.Errorf("marshal request: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx, method, c.baseURL+path, bytes.NewReader(payload),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return fmt.Errorf(
			"jira: %s %s -> %s: %s",
			method, path, resp.Status, strings.TrimSpace(string(respBody)),
		)
	}
	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil && err != io.EOF {
		return fmt.Errorf("decode response: %w", err)
	}
	return nil
}
