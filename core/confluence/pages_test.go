package confluence

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

const storedJSON = `{"id":"42","title":"Sprint 14 · Planning","version":{"number":3},
	"body":{"storage":{"value":"<p>plan</p>"}},"ancestors":[{"id":"1"},{"id":"7"}]}`

func TestGetPageStorageReadsBodyVersionAndAncestors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/content/42" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.URL.Query().Get("expand"); got != "body.storage,version,ancestors" {
			t.Errorf("expand = %q", got)
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte(storedJSON))
	}))
	defer srv.Close()

	p, err := NewClient(srv.URL, "secret", "", false).GetPageStorage(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "42" || p.Version != 3 || p.Body != "<p>plan</p>" || len(p.AncestorIDs) != 2 || p.AncestorIDs[1] != "7" {
		t.Fatalf("page = %+v", p)
	}
}

func TestFindPageByTitleSearchesTheSpace(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q, _ := url.ParseQuery(r.URL.RawQuery)
		if r.URL.Path != "/rest/api/content" || q.Get("spaceKey") != "PLAT" || q.Get("type") != "page" {
			t.Errorf("request = %s ? %s", r.URL.Path, r.URL.RawQuery)
		}
		if q.Get("title") == "Missing" {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		if q.Get("title") != "Sprint 14 · Planning" {
			t.Errorf("title = %q", q.Get("title"))
		}
		_, _ = w.Write([]byte(`{"results":[` + storedJSON + `]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "secret", "", false)

	p, ok, err := c.FindPageByTitle(context.Background(), "PLAT", "Sprint 14 · Planning")
	if err != nil || !ok || p.ID != "42" {
		t.Fatalf("found = %+v, %v, %v", p, ok, err)
	}
	_, ok, err = c.FindPageByTitle(context.Background(), "PLAT", "Missing")
	if err != nil || ok {
		t.Fatalf("missing = %v, %v", ok, err)
	}
	if calls != 2 {
		t.Fatalf("calls = %d", calls)
	}
}

func TestCreatePageSendsSpaceParentAndStorageBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/content" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("content type = %q", r.Header.Get("Content-Type"))
		}
		raw, _ := io.ReadAll(r.Body)
		var got struct {
			Type      string `json:"type"`
			Title     string `json:"title"`
			Space     struct{ Key string } `json:"space"`
			Ancestors []struct{ ID string } `json:"ancestors"`
			Body      struct {
				Storage struct{ Value, Representation string } `json:"storage"`
			} `json:"body"`
		}
		if err := json.Unmarshal(raw, &got); err != nil {
			t.Fatalf("payload %s: %v", raw, err)
		}
		if got.Type != "page" || got.Title != "Sprint 14" || got.Space.Key != "PLAT" ||
			len(got.Ancestors) != 1 || got.Ancestors[0].ID != "1" ||
			got.Body.Storage.Value != "<p>overview</p>" || got.Body.Storage.Representation != "storage" {
			t.Errorf("payload = %s", raw)
		}
		// Confluence answers a create without the body when expand is ignored.
		_, _ = w.Write([]byte(`{"id":"99","title":"Sprint 14","version":{"number":1}}`))
	}))
	defer srv.Close()

	p, err := NewClient(srv.URL, "secret", "", false).CreatePage(context.Background(), "PLAT", "1", "Sprint 14", "<p>overview</p>")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "99" || p.Version != 1 || p.Body != "<p>overview</p>" {
		t.Fatalf("created = %+v", p)
	}
}

func TestUpdatePageSendsTheVersionItIsGiven(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/rest/api/content/42" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		raw, _ := io.ReadAll(r.Body)
		var got struct {
			ID      string `json:"id"`
			Version struct{ Number int } `json:"version"`
		}
		_ = json.Unmarshal(raw, &got)
		if got.ID != "42" || got.Version.Number != 4 {
			t.Errorf("payload = %s", raw)
		}
		_, _ = w.Write([]byte(`{"id":"42","title":"T","version":{"number":4},"body":{"storage":{"value":"<p>v4</p>"}}}`))
	}))
	defer srv.Close()

	p, err := NewClient(srv.URL, "secret", "", false).UpdatePage(context.Background(), "42", "T", "<p>v4</p>", 4)
	if err != nil || p.Version != 4 {
		t.Fatalf("updated = %+v, %v", p, err)
	}
}

func TestConflictAndNotFoundAreMatchable(t *testing.T) {
	for _, tc := range []struct {
		code      int
		want, not error
	}{
		{http.StatusConflict, ErrVersionConflict, ErrNotFound},
		{http.StatusNotFound, ErrNotFound, ErrVersionConflict},
	} {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tc.code)
			_, _ = w.Write([]byte(`{"message":"nope"}`))
		}))
		_, err := NewClient(srv.URL, "secret", "", false).UpdatePage(context.Background(), "42", "T", "<p/>", 2)
		srv.Close()
		if !errors.Is(err, tc.want) || errors.Is(err, tc.not) {
			t.Errorf("code %d: err = %v", tc.code, err)
		}
	}
}
