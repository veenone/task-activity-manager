package confluence

import (
	"context"
	"errors"
	"net/http"
	"net/url"
)

var (
	// ErrNotFound matches a 404 from any call through errors.Is. The ritual
	// sync reads it as a page deleted or moved out of reach in Confluence.
	ErrNotFound = errors.New("confluence: not found")
	// ErrVersionConflict matches a 409, Confluence's answer to an update whose
	// version number is not the page's current version plus one.
	ErrVersionConflict = errors.New("confluence: version conflict")
)

// Is lets errors.Is match an HTTP answer against the two sentinels by code,
// so a caller never compares status numbers itself.
func (e *HTTPError) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return e.Code == http.StatusNotFound
	case ErrVersionConflict:
		return e.Code == http.StatusConflict
	}
	return false
}

// StoredPage is a page the way the ritual sync reads and writes it: the
// storage body, the version an update must build on, and the ancestor ids an
// adoption checks against the rituals root. Ancestors run root first.
type StoredPage struct {
	ID          string
	Title       string
	Version     int
	Body        string
	AncestorIDs []string
}

// Pages is what the ritual sync needs from Confluence. The HTTP client below
// satisfies it, and so does TAM's in-memory demo space.
type Pages interface {
	GetPageStorage(ctx context.Context, id string) (StoredPage, error)
	FindPageByTitle(ctx context.Context, spaceKey, title string) (StoredPage, bool, error)
	CreatePage(ctx context.Context, spaceKey, parentID, title, body string) (StoredPage, error)
	UpdatePage(ctx context.Context, id, title, body string, version int) (StoredPage, error)
}

var _ Pages = (*Client)(nil)

const storedExpand = "body.storage,version,ancestors"

type rawStoredPage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Version struct {
		Number int `json:"number"`
	} `json:"version"`
	Body struct {
		Storage struct {
			Value string `json:"value"`
		} `json:"storage"`
	} `json:"body"`
	Ancestors []struct {
		ID string `json:"id"`
	} `json:"ancestors"`
}

func (r rawStoredPage) stored() StoredPage {
	p := StoredPage{ID: r.ID, Title: r.Title, Version: r.Version.Number, Body: r.Body.Storage.Value, AncestorIDs: []string{}}
	for _, a := range r.Ancestors {
		p.AncestorIDs = append(p.AncestorIDs, a.ID)
	}
	return p
}

type storageValue struct {
	Value          string `json:"value"`
	Representation string `json:"representation"`
}

func storageBody(body string) map[string]storageValue {
	return map[string]storageValue{"storage": {Value: body, Representation: "storage"}}
}

// GetPageStorage reads one page's storage body, version and ancestors.
func (c *Client) GetPageStorage(ctx context.Context, id string) (StoredPage, error) {
	var r rawStoredPage
	if err := c.get(ctx, "/rest/api/content/"+url.PathEscape(id)+"?expand="+storedExpand).Decode(&r); err != nil {
		return StoredPage{}, err
	}
	return r.stored(), nil
}

// FindPageByTitle looks for a page anywhere in the space. Confluence Data
// Center keeps titles unique within a space, so there is at most one answer,
// and it may sit outside the rituals root: the caller checks AncestorIDs.
func (c *Client) FindPageByTitle(ctx context.Context, spaceKey, title string) (StoredPage, bool, error) {
	q := url.Values{}
	q.Set("spaceKey", spaceKey)
	q.Set("title", title)
	q.Set("type", "page")
	q.Set("expand", storedExpand)
	var r struct {
		Results []rawStoredPage `json:"results"`
	}
	if err := c.get(ctx, "/rest/api/content?"+q.Encode()).Decode(&r); err != nil {
		return StoredPage{}, false, err
	}
	if len(r.Results) == 0 {
		return StoredPage{}, false, nil
	}
	return r.Results[0].stored(), true, nil
}

// CreatePage creates a page under parentID. A response that leaves the body
// out answers with the body that was sent, since that is what now exists.
func (c *Client) CreatePage(ctx context.Context, spaceKey, parentID, title, body string) (StoredPage, error) {
	payload := map[string]any{
		"type":      "page",
		"title":     title,
		"space":     map[string]string{"key": spaceKey},
		"ancestors": []map[string]string{{"id": parentID}},
		"body":      storageBody(body),
	}
	var r rawStoredPage
	if err := c.send(ctx, http.MethodPost, "/rest/api/content?expand="+storedExpand, payload).Decode(&r); err != nil {
		return StoredPage{}, err
	}
	p := r.stored()
	if p.Body == "" {
		p.Body = body
	}
	return p, nil
}

// UpdatePage replaces a page's title and body. version is sent exactly as
// given, and must be the page's current version plus one: Confluence refuses
// anything else with 409, which is the optimistic concurrency the sync relies
// on instead of inventing its own.
func (c *Client) UpdatePage(ctx context.Context, id, title, body string, version int) (StoredPage, error) {
	payload := map[string]any{
		"id":      id,
		"type":    "page",
		"title":   title,
		"version": map[string]int{"number": version},
		"body":    storageBody(body),
	}
	var r rawStoredPage
	if err := c.send(ctx, http.MethodPut, "/rest/api/content/"+url.PathEscape(id)+"?expand="+storedExpand, payload).Decode(&r); err != nil {
		return StoredPage{}, err
	}
	p := r.stored()
	if p.Body == "" {
		p.Body = body
	}
	return p, nil
}
