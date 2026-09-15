package confluence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestClientMapsHTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"message":"denied"}`))
	}))
	defer srv.Close()
	_, err := NewClient(srv.URL, "secret", "", false).GetPageStorage(context.Background(), "42")
	h, ok := err.(*HTTPError)
	if !ok || h.Code != http.StatusForbidden || h.Message != "denied" {
		t.Fatalf("error = %#v", err)
	}
}
