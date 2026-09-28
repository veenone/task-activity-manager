package confluence

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

var png = []byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a, 7, 7, 7}

// filePart reads the one part a multipart request should carry and answers
// its name, filename, declared type and bytes.
func filePart(t *testing.T, r *http.Request) (name, filename, partType string, body []byte) {
	t.Helper()
	media, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "multipart/form-data" {
		t.Fatalf("content type = %q: %v", r.Header.Get("Content-Type"), err)
	}
	part, err := multipart.NewReader(r.Body, params["boundary"]).NextPart()
	if err != nil {
		t.Fatalf("first part: %v", err)
	}
	body, err = io.ReadAll(part)
	if err != nil {
		t.Fatalf("part body: %v", err)
	}
	return part.FormName(), part.FileName(), part.Header.Get("Content-Type"), body
}

func TestAttachFilePostsTheFilePartWithTheXSRFHeader(t *testing.T) {
	var posts int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			if r.URL.Path != "/rest/api/content/42/child/attachment" {
				t.Errorf("lookup path = %s", r.URL.Path)
			}
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		posts++
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/content/42/child/attachment" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("X-Atlassian-Token"); got != "nocheck" {
			t.Errorf("X-Atlassian-Token = %q", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer secret" {
			t.Errorf("authorization = %q", got)
		}
		name, filename, partType, body := filePart(t, r)
		if name != "file" || filename != "burndown.png" || partType != "image/png" || string(body) != string(png) {
			t.Errorf("part = %q %q %q %x", name, filename, partType, body)
		}
		_, _ = w.Write([]byte(`{"results":[{"id":"att1","title":"burndown.png"}]}`))
	}))
	defer srv.Close()

	a, err := NewClient(srv.URL, "secret", "", false).AttachFile(context.Background(), "42", "burndown.png", "image/png", png)
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "att1" || a.Filename != "burndown.png" {
		t.Fatalf("attached = %+v", a)
	}
	if posts != 1 {
		t.Fatalf("posts = %d", posts)
	}
}

// A sprint report is published again every time the sprint is re-reported, so
// the second attach of burndown.png must reach the existing attachment's data
// endpoint and leave the page with one attachment, not two.
func TestAttachFileTwiceReplacesTheFileInsteadOfAddingASecond(t *testing.T) {
	var created, replaced int
	var lastBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet:
			if got := r.URL.Query().Get("filename"); got != "burndown.png" {
				t.Errorf("lookup filename = %q", got)
			}
			if created == 0 {
				_, _ = w.Write([]byte(`{"results":[]}`))
				return
			}
			_, _ = w.Write([]byte(`{"results":[{"id":"att1","title":"burndown.png"}]}`))
		case r.URL.Path == "/rest/api/content/42/child/attachment":
			created++
			if r.Method != http.MethodPost {
				t.Errorf("create method = %s", r.Method)
			}
			_, _, _, body := filePart(t, r)
			lastBody = string(body)
			_, _ = w.Write([]byte(`{"results":[{"id":"att1","title":"burndown.png"}]}`))
		case r.URL.Path == "/rest/api/content/42/child/attachment/att1/data":
			replaced++
			if r.Method != http.MethodPost {
				t.Errorf("replace method = %s", r.Method)
			}
			if got := r.Header.Get("X-Atlassian-Token"); got != "nocheck" {
				t.Errorf("X-Atlassian-Token = %q", got)
			}
			name, filename, _, body := filePart(t, r)
			if name != "file" || filename != "burndown.png" {
				t.Errorf("part = %q %q", name, filename)
			}
			lastBody = string(body)
			// A replace answers the one attachment, and without its title.
			_, _ = w.Write([]byte(`{"id":"att1"}`))
		default:
			t.Errorf("unexpected request = %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusBadRequest)
		}
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "secret", "", false)

	if _, err := c.AttachFile(context.Background(), "42", "burndown.png", "image/png", png); err != nil {
		t.Fatal(err)
	}
	a, err := c.AttachFile(context.Background(), "42", "burndown.png", "image/png", []byte("second"))
	if err != nil {
		t.Fatal(err)
	}
	if a.ID != "att1" || a.Filename != "burndown.png" {
		t.Fatalf("attached = %+v", a)
	}
	if created != 1 || replaced != 1 {
		t.Fatalf("created = %d, replaced = %d", created, replaced)
	}
	if lastBody != "second" {
		t.Fatalf("stored body = %q", lastBody)
	}
}

// The filename lands in a Content-Disposition header, and the caller's own
// string put it there, so a name carrying a line break or a path never
// reaches the wire.
func TestAttachFileRefusesAFilenameThatWouldForgeAHeader(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_, _ = w.Write([]byte(`{"results":[]}`))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "secret", "", false)

	for _, name := range []string{"", "burn\r\nX-Evil: yes.png", `burn"down.png`, "charts/burndown.png", `charts\burndown.png`, ".."} {
		if _, err := c.AttachFile(context.Background(), "42", name, "image/png", png); err == nil {
			t.Errorf("filename %q was accepted", name)
		}
	}
	if calls != 0 {
		t.Fatalf("calls = %d, a refused filename still reached Confluence", calls)
	}
}

func TestAttachFileFailureNamesThePageAndTheFileAndKeepsTheTokenOut(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			_, _ = w.Write([]byte(`{"results":[]}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte(`{"message":"No content found with id"}`))
	}))
	defer srv.Close()

	_, err := NewClient(srv.URL, "secret", "", false).AttachFile(context.Background(), "42", "burndown.png", "image/png", png)
	if err == nil {
		t.Fatal("a 404 attached the file")
	}
	got := err.Error()
	if !strings.Contains(got, "burndown.png") || !strings.Contains(got, "page 42") {
		t.Errorf("error = %q, it names neither the file nor the page", got)
	}
	if strings.Contains(got, "secret") {
		t.Errorf("error = %q, it carries the token", got)
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("error = %q, it no longer matches ErrNotFound", got)
	}
}
