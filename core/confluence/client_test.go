package confluence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientGetsPageWithBearerAndDecodesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer secret" { t.Errorf("authorization = %q", r.Header.Get("Authorization")) }
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"42","title":"Review","space":{"key":"PLAT"},"version":{"number":3},"body":{"storage":{"value":"<p>Review</p>"}}}`))
	}))
	defer srv.Close()
	p, err := NewClient(srv.URL, "secret", "", false).GetPage(context.Background(), "42")
	if err != nil { t.Fatal(err) }
	if p.ID != "42" || p.Title != "Review" || p.Space.Key != "PLAT" || p.Body.Storage.Value != "<p>Review</p>" { t.Fatalf("decoded page = %+v", p) }
}

func TestClientMapsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusForbidden); _, _ = w.Write([]byte(`{"message":"denied"}`)) }))
	defer srv.Close()
	_, err := NewClient(srv.URL, "secret", "", false).GetPage(context.Background(), "42")
	h, ok := err.(*HTTPError)
	if !ok || h.Code != http.StatusForbidden || h.Message != "denied" { t.Fatalf("error = %#v", err) }
}
