// Package zephyr adapts a Zephyr Essential instance to the neutral
// backend.Backend interface, as internal/backend/xray and
// internal/backend/kiwi do for their own systems.
//
// Zephyr Essential is SmartBear's former Zephyr Squad, renamed in April 2025.
// Its Data Center API is the ZAPI-style one at {JIRA}/rest/zapi/latest. The
// other product, now called plain Zephyr, is the former Zephyr Scale and has a
// different API; agents/project/glossary.md records the trap, because planning
// against the wrong one is the first mistake available here.
//
// Only Data Center is in scope. Essential Cloud is a different transport, with
// a JWT per endpoint and its own credential type.
//
// This file is the transport. A Zephyr test is a Jira issue, so the adapter
// also holds an ordinary Jira client and most of its reads go through that
// rather than through here; see adapter.go.
//
// Design and gap analysis:
// docs/superpowers/specs/2026-10-30-zephyr-essential-bridge-design.md
// Plan: docs/superpowers/plans/2026-10-30-zephyr-essential-bridge.md (#371)
package zephyr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"

	corejira "agile-suite/core/jira"
)

// apiPath is the Zephyr Essential Data Center API root. Every path this
// package asks for is relative to it.
const apiPath = "/rest/zapi/latest"

// Client talks to one instance's Zephyr endpoints. The shared Jira transport
// carries it: the credential is the same Jira personal access token, sent as a
// bearer, and the rate limiter and TLS settings are the ones every other call
// to this instance already goes through.
type Client struct {
	jira *corejira.Client

	issueType    cache[string]
	execStatuses cache[*StatusVocabulary]
	stepStatuses cache[*StatusVocabulary]
	issueIDs     keyedCache
}

// NewClient builds a client for the Jira instance at baseURL, authenticated
// with a Jira personal access token. The options are the shared transport's,
// so a profile's CA certificate and its allow-untrusted setting apply here
// exactly as they do to its Jira calls.
func NewClient(baseURL, token string, opts ...corejira.Option) *Client {
	return &Client{jira: corejira.NewClient(baseURL, token, opts...)}
}

// NewClientWithHTTP builds a client on a caller-supplied http.Client, which
// tests use to point at an httptest server.
func NewClientWithHTTP(baseURL, token string, h *http.Client) *Client {
	return &Client{jira: corejira.NewClientWithHTTP(baseURL, token, h)}
}

// getJSON performs an authenticated GET under the Zephyr API root and decodes
// the response into out, which may be nil to discard it.
func (c *Client) getJSON(ctx context.Context, path string, out any) error {
	body, err := c.jira.GetBytes(ctx, apiPath+path)
	if err != nil {
		return fmt.Errorf("zephyr: GET %s: %w", path, err)
	}
	if err := bodyError(body); err != nil {
		return fmt.Errorf("zephyr: GET %s: %w", path, err)
	}
	if out == nil {
		return nil
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("zephyr: decode %s: %w", path, err)
	}
	return nil
}

// bodyError reads Zephyr's error envelope out of a response that arrived with
// a successful status. The Data Center documentation states that an invalid
// authentication can answer 200 OK carrying an errorId of "ERROR", so trusting
// the status code reads a rejected token as success and reports an empty
// project rather than a credential problem (I1).
//
// A body that is not a JSON object cannot be an envelope: several endpoints
// answer with a bare array, and /cycle answers with an object keyed by cycle
// id, which carries no errorId either way. Anything that fails to decode as
// the envelope is left to the caller's own decode, which reports it with the
// endpoint named.
func bodyError(body []byte) error {
	trimmed := bytes.TrimLeft(body, " \t\r\n")
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return nil
	}
	var env struct {
		ErrorID   json.RawMessage `json:"errorId"`
		ErrorDesc string          `json:"errorDesc"`
		ClientMsg string          `json:"clientMessage"`
	}
	if json.Unmarshal(trimmed, &env) != nil {
		return nil
	}
	id := strings.Trim(string(env.ErrorID), `"`)
	if id == "" || id == "null" {
		return nil
	}
	for _, msg := range []string{env.ErrorDesc, env.ClientMsg} {
		if strings.TrimSpace(msg) != "" {
			return errors.New(strings.TrimSpace(msg))
		}
	}
	return fmt.Errorf("error %s", id)
}

// TestIssueTypeID is the Jira issue type id Zephyr uses for a test on this
// instance, read from /util/zephyrTestIssueType. It is per instance, so it is
// asked for rather than assumed, and it doubles as the proof that the Zephyr
// plugin is installed at all.
//
// The id arrived quoted on the probed instance. It is only ever used as a
// string, in JQL and in an issue create, so an unquoted number is accepted
// rather than being a decode failure on some other instance.
func (c *Client) TestIssueTypeID(ctx context.Context) (string, error) {
	return c.issueType.get(func() (string, error) {
		var resp struct {
			ID json.RawMessage `json:"testcaseIssueTypeId"`
		}
		if err := c.getJSON(ctx, "/util/zephyrTestIssueType", &resp); err != nil {
			return "", fmt.Errorf("zephyr is not installed or not reachable on this instance: %w", err)
		}
		id := strings.Trim(string(resp.ID), `"`)
		if id == "" || id == "null" {
			return "", errors.New("zephyr reported no Test issue type for this instance")
		}
		return id, nil
	})
}

// cache holds a value read from the instance once per client.
//
// A failure is deliberately not cached. A cancelled context or one 503 would
// otherwise poison every later call on this client, which is the mistake
// Kiwi's plugin detection records in its "confirmed" flag. A client lives for
// one operation here, so this is a cheap guarantee rather than a profile-wide
// one.
type cache[T any] struct {
	mu   sync.Mutex
	val  T
	done bool
}

func (c *cache[T]) get(load func() (T, error)) (T, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.done {
		return c.val, nil
	}
	v, err := load()
	if err != nil {
		var zero T
		return zero, err
	}
	c.val, c.done = v, true
	return v, nil
}
