package jira

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAttachmentsReadsTheIssuesFiles(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`{"key":"PLAT-1","fields":{"attachment":[
			{"id":"10010","filename":"trace.log","author":{"name":"jdoe","displayName":"Jane Doe"},"created":"2026-11-17T09:00:00.000+0700","size":2048,"mimeType":"text/plain","content":"http://jira/secure/attachment/10010/trace.log"}
		]}}`))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "tok").Attachments(context.Background(), "PLAT-1")
	if err != nil {
		t.Fatalf("Attachments: %v", err)
	}
	if asked != "/rest/api/2/issue/PLAT-1?fields=attachment" {
		t.Errorf("asked %s, want the issue narrowed to its attachment field", asked)
	}
	if len(got) != 1 || got[0].ID != "10010" || got[0].Filename != "trace.log" || got[0].Size != 2048 || got[0].Author.DisplayName != "Jane Doe" {
		t.Errorf("attachments = %+v", got)
	}
}

// Jira refuses an upload that does not carry X-Atlassian-Token: no-check, and
// reads the file from a multipart part named "file".
func TestAddAttachmentPostsTheFileAsMultipart(t *testing.T) {
	var token, auth, name, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue/PLAT-1/attachments" {
			t.Errorf("request = %s %s", r.Method, r.URL.Path)
		}
		token, auth = r.Header.Get("X-Atlassian-Token"), r.Header.Get("Authorization")
		f, h, err := r.FormFile("file")
		if err != nil {
			t.Errorf("no file part: %v", err)
			return
		}
		raw, _ := io.ReadAll(f)
		name, body = h.Filename, string(raw)
		_, _ = w.Write([]byte(`[{"id":"10011"}]`))
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "tok").AddAttachment(context.Background(), "PLAT-1", "trace.log", strings.NewReader("line one\n"))
	if err != nil {
		t.Fatalf("AddAttachment: %v", err)
	}
	if token != "no-check" || auth != "Bearer tok" {
		t.Errorf("headers: token %q, auth %q", token, auth)
	}
	if name != "trace.log" || body != "line one\n" {
		t.Errorf("uploaded %q with %q", name, body)
	}
}

func TestAddAttachmentReportsWhatJiraSaid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		_, _ = w.Write([]byte(`{"errorMessages":["The file is larger than the 10 MB this instance allows"]}`))
	}))
	defer srv.Close()

	err := NewClient(srv.URL, "tok").AddAttachment(context.Background(), "PLAT-1", "big.bin", strings.NewReader("x"))
	if err == nil || !strings.Contains(err.Error(), "larger than the 10 MB") {
		t.Fatalf("AddAttachment = %v, want the instance's own reason", err)
	}
}

// The token goes to the instance and nowhere else: a content link pointing at
// another host is refused before any request is made.
func TestDownloadAttachmentOnlyReadsFromTheInstance(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/secure/attachment/10010/trace.log" || r.Header.Get("Authorization") != "Bearer tok" {
			t.Errorf("request = %s, auth %q", r.URL.Path, r.Header.Get("Authorization"))
		}
		_, _ = w.Write([]byte("line one\n"))
	}))
	defer srv.Close()
	c := NewClient(srv.URL, "tok")

	var out bytes.Buffer
	if err := c.DownloadAttachment(context.Background(), srv.URL+"/secure/attachment/10010/trace.log", &out); err != nil {
		t.Fatalf("DownloadAttachment: %v", err)
	}
	if out.String() != "line one\n" {
		t.Errorf("downloaded %q", out.String())
	}
	if err := c.DownloadAttachment(context.Background(), "http://elsewhere.example/secure/attachment/1/x", &out); err == nil {
		t.Error("DownloadAttachment followed a link off the instance")
	}
}
