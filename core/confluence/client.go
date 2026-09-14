// Package confluence provides the read-only Confluence Data Center transport
// used by TAM's Rituals view.
package confluence

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Page struct {
	ID      string      `json:"id"`
	Title   string      `json:"title"`
	Space   PageSpace   `json:"space"`
	Version PageVersion `json:"version"`
	Links   PageLinks   `json:"_links"`
	Body    PageBody    `json:"body"`
}
type PageSpace struct {
	Key string `json:"key"`
}
type PageVersion struct {
	Number int    `json:"number"`
	When   string `json:"when"`
}
type PageLinks struct {
	WebUI string `json:"webui"`
}
type PageBody struct {
	Storage PageStorage `json:"storage"`
	View    PageView    `json:"view"`
}
type PageStorage struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}
type PageView struct {
	Value string `json:"value"`
}

type ChildPage struct {
	ID     string    `json:"id"`
	Title  string    `json:"title"`
	Type   string    `json:"type"`
	Status string    `json:"status"`
	Links  PageLinks `json:"_links"`
}
type ChildPageResult struct {
	Results []ChildPage `json:"results"`
	Start   int         `json:"start"`
	Limit   int         `json:"limit"`
	Size    int         `json:"size"`
}

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

func (c *Client) GetPage(ctx context.Context, id string) (Page, error) {
	var p Page
	err := c.get(ctx, "/rest/api/content/"+url.PathEscape(id)+"?expand=body.storage,body.view,version,space,_links").Decode(&p)
	return p, err
}
func (c *Client) ListChildPages(ctx context.Context, parentID string, start, limit int) (ChildPageResult, error) {
	if limit <= 0 {
		limit = 50
	}
	path := "/rest/api/content/" + url.PathEscape(parentID) + "/child/page?start=" + strconv.Itoa(start) + "&limit=" + strconv.Itoa(limit) + "&expand=version,_links"
	var r ChildPageResult
	err := c.get(ctx, path).Decode(&r)
	return r, err
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return responseDecoder{err: err}
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
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
