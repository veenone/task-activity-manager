package confluence

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func probeServer(t *testing.T, listing int, space string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/rest/api/content":
			if q := r.URL.Query(); q.Get("spaceKey") != "TEAM" || q.Get("limit") != "1" {
				t.Errorf("listing query = %s", r.URL.RawQuery)
			}
			w.WriteHeader(listing)
			if listing == http.StatusOK {
				_, _ = w.Write([]byte(`{"results":[{"id":"1"}]}`))
				return
			}
			_, _ = w.Write([]byte(`{"message":"nope"}`))
		case "/rest/api/space/TEAM":
			if r.URL.Query().Get("expand") != "operations" {
				t.Errorf("space query = %s", r.URL.RawQuery)
			}
			_, _ = w.Write([]byte(space))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
		}
	}))
}

func TestCanCreatePagesReadsTheSpaceThenItsOperations(t *testing.T) {
	for _, tc := range []struct {
		name    string
		listing int
		space   string
		want    Permission
	}{
		{"create listed", http.StatusOK, `{"key":"TEAM","operations":[{"operation":"read","targetType":"space"},{"operation":"create","targetType":"page"}]}`, PermissionYes},
		{"operations without create", http.StatusOK, `{"key":"TEAM","operations":[{"operation":"read","targetType":"space"}]}`, PermissionNo},
		{"operations not reported", http.StatusOK, `{"key":"TEAM"}`, PermissionUnknown},
		{"space forbidden", http.StatusForbidden, "", PermissionNo},
		{"space not found", http.StatusNotFound, "", PermissionNo},
		{"server trouble", http.StatusInternalServerError, "", PermissionUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			srv := probeServer(t, tc.listing, tc.space)
			defer srv.Close()
			if got := NewClient(srv.URL, "secret", "", false).CanCreatePages(context.Background(), "TEAM"); got != tc.want {
				t.Fatalf("got %s, want %s", got, tc.want)
			}
		})
	}
}
