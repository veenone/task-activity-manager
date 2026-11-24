package jira

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// throttled answers the first `limited` requests with status and the
// Retry-After value given, then 200, recording every body it was sent.
func throttled(t *testing.T, limited int, status int, retryAfter string) (*httptest.Server, *[]string) {
	t.Helper()
	var bodies []string
	var n int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if int(atomic.AddInt32(&n, 1)) <= limited {
			if retryAfter != "" {
				w.Header().Set("Retry-After", retryAfter)
			}
			w.WriteHeader(status)
			return
		}
		_, _ = w.Write([]byte(`{"key":"PLAT-1"}`))
	}))
	t.Cleanup(srv.Close)
	return srv, &bodies
}

// recordSleeps swaps the client's wait for one that returns at once and
// records each duration it was asked to wait.
func recordSleeps(c *Client) *[]time.Duration {
	var waits []time.Duration
	c.sleep = func(_ context.Context, d time.Duration) error {
		waits = append(waits, d)
		return nil
	}
	return &waits
}

func TestA429IsResentWithTheSameBodyAfterRetryAfterSeconds(t *testing.T) {
	srv, bodies := throttled(t, 1, http.StatusTooManyRequests, "7")
	c := NewClientWithHTTP(srv.URL, "tok", nil)
	waits := recordSleeps(c)

	var out struct{ Key string }
	err := c.WriteJSONReturning(context.Background(), http.MethodPost, "/rest/api/2/issue", map[string]string{"summary": "x"}, &out)
	if err != nil {
		t.Fatalf("write after one 429 failed: %v", err)
	}
	if out.Key != "PLAT-1" {
		t.Errorf("key = %q, want PLAT-1", out.Key)
	}
	want := `{"summary":"x"}`
	if len(*bodies) != 2 || (*bodies)[0] != want || (*bodies)[1] != want {
		t.Errorf("bodies sent = %q, want the payload twice", *bodies)
	}
	if len(*waits) != 1 || (*waits)[0] != 7*time.Second {
		t.Errorf("waits = %v, want [7s] from Retry-After", *waits)
	}
}

func TestRetryAfterAsAnHTTPDateIsHonoured(t *testing.T) {
	at := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	srv, _ := throttled(t, 1, http.StatusTooManyRequests, at)
	c := NewClientWithHTTP(srv.URL, "tok", nil)
	waits := recordSleeps(c)

	if _, err := c.GetBytes(context.Background(), "/x"); err != nil {
		t.Fatalf("get after one 429 failed: %v", err)
	}
	// The date has one-second precision, so the wait lands just under 10s.
	if len(*waits) != 1 || (*waits)[0] < 8*time.Second || (*waits)[0] > 10*time.Second {
		t.Errorf("waits = %v, want one of about 10s from the HTTP-date", *waits)
	}
}

func TestA503WithRetryAfterIsResentButOneWithoutIsNot(t *testing.T) {
	srv, bodies := throttled(t, 1, http.StatusServiceUnavailable, "1")
	c := NewClientWithHTTP(srv.URL, "tok", nil)
	recordSleeps(c)
	if _, err := c.GetBytes(context.Background(), "/x"); err != nil {
		t.Fatalf("503 with Retry-After was not resent: %v", err)
	}
	if len(*bodies) != 2 {
		t.Errorf("requests = %d, want 2", len(*bodies))
	}

	bare, bareBodies := throttled(t, 1, http.StatusServiceUnavailable, "")
	c2 := NewClientWithHTTP(bare.URL, "tok", nil)
	waits := recordSleeps(c2)
	if _, err := c2.GetBytes(context.Background(), "/x"); err == nil {
		t.Error("a bare 503 came back as success")
	}
	if len(*bareBodies) != 1 || len(*waits) != 0 {
		t.Errorf("bare 503: requests = %d, waits = %v; want 1 and none", len(*bareBodies), *waits)
	}
}

func TestA429WithoutRetryAfterBacksOffAndAttemptsAreBounded(t *testing.T) {
	srv, bodies := throttled(t, 100, http.StatusTooManyRequests, "")
	c := NewClientWithHTTP(srv.URL, "tok", nil)
	waits := recordSleeps(c)

	resp, err := c.WriteJSONRaw(context.Background(), http.MethodPost, "/x", map[string]int{"a": 1})
	if err != nil {
		t.Fatalf("WriteJSONRaw: %v", err)
	}
	if resp.Code != http.StatusTooManyRequests {
		t.Errorf("final code = %d, want the last 429 unchanged", resp.Code)
	}
	if len(*bodies) != 1+maxResends {
		t.Errorf("requests = %d, want %d", len(*bodies), 1+maxResends)
	}
	want := []time.Duration{time.Second, 2 * time.Second, 4 * time.Second}
	if len(*waits) != len(want) {
		t.Fatalf("waits = %v, want %v", *waits, want)
	}
	for i := range want {
		if (*waits)[i] != want[i] {
			t.Errorf("waits = %v, want %v", *waits, want)
			break
		}
	}
}

func TestAWaitPastTheTotalCapIsNotTaken(t *testing.T) {
	srv, bodies := throttled(t, 1, http.StatusTooManyRequests, "3600")
	c := NewClientWithHTTP(srv.URL, "tok", nil)
	waits := recordSleeps(c)

	resp, err := c.WriteJSONRaw(context.Background(), http.MethodPost, "/x", 1)
	if err != nil {
		t.Fatalf("WriteJSONRaw: %v", err)
	}
	if resp.Code != http.StatusTooManyRequests || len(*bodies) != 1 || len(*waits) != 0 {
		t.Errorf("code = %d, requests = %d, waits = %v; want the 429 back with no wait", resp.Code, len(*bodies), *waits)
	}
}

func TestCancellingTheContextStopsTheWait(t *testing.T) {
	srv, bodies := throttled(t, 1, http.StatusTooManyRequests, "20")
	c := NewClientWithHTTP(srv.URL, "tok", nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	c.sleep = func(ctx context.Context, d time.Duration) error {
		cancel()
		return sleepCtx(ctx, d)
	}

	start := time.Now()
	_, err := c.GetBytes(ctx, "/x")
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
	if time.Since(start) > 5*time.Second || len(*bodies) != 1 {
		t.Errorf("took %v with %d requests; the wait should end on cancel", time.Since(start), len(*bodies))
	}
}

func TestABodyThatCannotBeReplayedIsNotResent(t *testing.T) {
	srv, bodies := throttled(t, 1, http.StatusTooManyRequests, "1")
	c := NewClientWithHTTP(srv.URL, "tok", nil)
	waits := recordSleeps(c)

	// An io.NopCloser hides the reader's type, so NewRequest sets no GetBody.
	req, err := http.NewRequest(http.MethodPost, srv.URL+"/x", io.NopCloser(strings.NewReader("once")))
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatalf("Do: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests || len(*bodies) != 1 || len(*waits) != 0 {
		t.Errorf("code = %d, requests = %d, waits = %v; want the 429 back untouched", resp.StatusCode, len(*bodies), *waits)
	}
}
