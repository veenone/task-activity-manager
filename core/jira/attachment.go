package jira

import (
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"
)

// RawAttachment is one file on an issue as Jira answers it. Content is the
// link Jira serves the bytes from; it is on the instance itself.
type RawAttachment struct {
	ID       string `json:"id"`
	Filename string `json:"filename"`
	Author   struct {
		Name        string `json:"name"`
		DisplayName string `json:"displayName"`
	} `json:"author"`
	Created  string `json:"created"`
	Size     int64  `json:"size"`
	MimeType string `json:"mimeType"`
	Content  string `json:"content"`
}

// Attachments reads the files on an issue. There is no endpoint that lists
// them on their own, so this asks for the issue narrowed to the one field.
func (c *Client) Attachments(ctx context.Context, key string) ([]RawAttachment, error) {
	var issue struct {
		Fields struct {
			Attachment []RawAttachment `json:"attachment"`
		} `json:"fields"`
	}
	if err := c.Get(ctx, "/rest/api/2/issue/"+url.PathEscape(key)+"?fields=attachment", &issue); err != nil {
		return nil, err
	}
	if issue.Fields.Attachment == nil {
		return []RawAttachment{}, nil
	}
	return issue.Fields.Attachment, nil
}

// AddAttachment uploads one file to an issue under filename.
//
// The body is multipart with the file in a part named "file", and the request
// carries X-Atlassian-Token: no-check, without which Jira answers 403 to
// guard against a forged form post. The file is streamed through a pipe
// rather than read into memory first, because an attachment can be as large
// as the instance allows.
func (c *Client) AddAttachment(ctx context.Context, key, filename string, file io.Reader) error {
	path := "/rest/api/2/issue/" + url.PathEscape(key) + "/attachments"
	pr, pw := io.Pipe()
	form := multipart.NewWriter(pw)
	go func() {
		part, err := form.CreateFormFile("file", filename)
		if err == nil {
			_, err = io.Copy(part, file)
		}
		if err == nil {
			err = form.Close()
		}
		pw.CloseWithError(err)
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+path, pr)
	if err != nil {
		pr.Close()
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", form.FormDataContentType())
	req.Header.Set("X-Atlassian-Token", "no-check")
	req.Header.Set("Accept", "application/json")
	resp, err := c.Do(req)
	if err != nil {
		pr.Close()
		return err
	}
	defer resp.Body.Close()
	// Closing the read end stops the copy when Jira answers before the body
	// has been sent, which is what it does to a file over its size limit.
	defer pr.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, writeBodyLimit))
	if resp.StatusCode >= 300 {
		return writeStatusError(http.MethodPost, path, WriteResponse{Code: resp.StatusCode, Status: resp.Status, Body: body})
	}
	return nil
}

// DownloadAttachment copies an attachment's bytes into w. contentURL is the
// link the listing carried.
//
// A link that is not on this instance is refused. The request carries the
// user's token, and a listing is data from a server: following it to another
// host would hand the token to whoever that host is.
func (c *Client) DownloadAttachment(ctx context.Context, contentURL string, w io.Writer) error {
	if !strings.HasPrefix(contentURL, c.baseURL+"/") {
		return fmt.Errorf("the attachment's link is not on %s, so it was not followed", c.baseURL)
	}
	path := strings.TrimPrefix(contentURL, c.baseURL)
	resp, err := c.getRawAccepting(ctx, path, "*/*")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, writeBodyLimit))
		return getError(path, resp, body)
	}
	_, err = io.Copy(w, resp.Body)
	return err
}
