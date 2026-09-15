// Package confluence is the Confluence Data Center transport behind TAM's
// Rituals view: reading a page's storage body, finding a page by title, and
// creating and updating pages. TAM's ritualsync decides when each is called;
// nothing here keeps state between calls.
package confluence

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
	"time"
)

type HTTPError struct {
	Code    int
	Status  string
	Message string
}

func (e *HTTPError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("confluence: %s: %s", e.Status, e.Message)
	}
	return "confluence: " + e.Status
}

type Client struct {
	baseURL, token string
	http           *http.Client
}

func NewClient(baseURL, token string, caCert string, insecure bool) *Client {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if caCert != "" || insecure {
		pool, _ := x509.SystemCertPool()
		if pool == nil {
			pool = x509.NewCertPool()
		}
		if caCert != "" {
			pool.AppendCertsFromPEM([]byte(caCert))
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, InsecureSkipVerify: insecure}
	}
	return &Client{baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"), token: token, http: &http.Client{Timeout: 30 * time.Second, Transport: transport}}
}

type responseDecoder struct {
	body io.ReadCloser
	err  error
}

func (r responseDecoder) Decode(out any) error {
	if r.err != nil {
		return r.err
	}
	defer r.body.Close()
	return json.NewDecoder(r.body).Decode(out)
}

func (c *Client) get(ctx context.Context, path string) responseDecoder {
	return c.send(ctx, http.MethodGet, path, nil)
}

// send is every request this client makes. A nil payload sends no body; any
// other payload is JSON. A non-2xx answer becomes *HTTPError carrying
// Confluence's own message, which is what errors.Is matches ErrNotFound and
// ErrVersionConflict against.
func (c *Client) send(ctx context.Context, method, path string, payload any) responseDecoder {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return responseDecoder{err: err}
		}
		body = bytes.NewReader(encoded)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return responseDecoder{err: err}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	if payload != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return responseDecoder{err: err}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(b, &e)
		return responseDecoder{err: &HTTPError{Code: resp.StatusCode, Status: resp.Status, Message: e.Message}}
	}
	return responseDecoder{body: resp.Body}
}
