package confluence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestClientGetsPageWithBearerAndDecodesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" {
			t.Errorf("authorization = %q", r.Header.Get("Authorization"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"42","title":"Review","space":{"key":"PLAT"},"version":{"number":3},"body":{"storage":{"value":"<p>Review</p>"}}}`))
	}))
	defer srv.Close()
	p, err := NewClient(srv.URL, "secret", "", false).GetPage(context.Background(), "42")
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != "42" || p.Title != "Review" || p.Space.Key != "PLAT" || p.Body.Storage.Value != "<p>Review</p>" {
		t.Fatalf("decoded page = %+v", p)
	}
}

func TestClientMapsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"denied"}`))
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL, "secret", "", false).GetPage(context.Background(), "42")
	h, ok := err.(*HTTPError)
	if !ok || h.Code != http.StatusForbidden || h.Message != "denied" {
		t.Fatalf("error = %#v", err)
	}
}

func TestClientListsChildPagesWithPagination(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/rest/api/content/root/child/page" {
			t.Errorf("path = %q", r.URL.Path)
		}
		q, _ := url.ParseQuery(r.URL.RawQuery)
		if q.Get("start") != "20" || q.Get("limit") != "10" {
			t.Errorf("pagination = start %q limit %q", q.Get("start"), q.Get("limit"))
		}
		if q.Get("expand") != "version,_links" {
			t.Errorf("expand = %q", q.Get("expand"))
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"results":[{"id":"21","title":"Retro","type":"page","status":"current","_links":{"webui":"/pages/21"}}],"start":20,"limit":10,"size":1}`))
	}))
	defer srv.Close()
	r, err := NewClient(srv.URL+"/", "secret", "", false).ListChildPages(context.Background(), "root", 20, 10)
	if err != nil {
		t.Fatal(err)
	}
	if r.Start != 20 || r.Limit != 10 || len(r.Results) != 1 || r.Results[0].ID != "21" {
		t.Fatalf("result = %+v", r)
	}
}
