package jira

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Rate-limit resends: a 429, or a 503 that says when to come back, is sent
// again after the wait Jira asks for. The bounds keep one call from holding a
// commit for minutes; past them the last response goes back unchanged.
const (
	maxResends   = 3                // resends after the first attempt
	maxRetryWait = 30 * time.Second // total wait across one call's resends
	firstBackoff = time.Second      // wait before the first resend when Jira names none; doubles each time
)

// resendAfter reports whether resp is a rate-limit refusal worth sending req
// again for, and how long to wait first. A 429 always is: Jira did not
// process the request, so even a POST is safe to repeat. A 503 is only when
// it carries Retry-After; a bare 503 is an outage, not a throttle. A request
// whose body cannot be rebuilt is never resent.
func resendAfter(req *http.Request, resp *http.Response, attempt int) (time.Duration, bool) {
	header := resp.Header.Get("Retry-After")
	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
	case resp.StatusCode == http.StatusServiceUnavailable && header != "":
	default:
		return 0, false
	}
	if req.Body != nil && req.Body != http.NoBody && req.GetBody == nil {
		return 0, false
	}
	if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil {
		return max(time.Duration(secs)*time.Second, 0), true
	}
	if at, err := http.ParseTime(header); err == nil {
		return max(time.Until(at), 0), true
	}
	return firstBackoff << attempt, true
}

// waitToResend discards resp, waits d (or until req's context ends), and
// returns a copy of req with a fresh body ready to send again.
func (c *Client) waitToResend(req *http.Request, resp *http.Response, d time.Duration) (*http.Request, error) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, writeBodyLimit))
	resp.Body.Close()
	sleep := c.sleep
	if sleep == nil {
		sleep = sleepCtx
	}
	if err := sleep(req.Context(), d); err != nil {
		return nil, err
	}
	next := req.Clone(req.Context())
	if req.GetBody != nil {
		body, err := req.GetBody()
		if err != nil {
			return nil, err
		}
		next.Body = body
	}
	return next, nil
}

// sleepFunc waits d or until ctx ends, returning ctx's error in that case.
type sleepFunc func(ctx context.Context, d time.Duration) error

// sleepCtx is the real wait.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
