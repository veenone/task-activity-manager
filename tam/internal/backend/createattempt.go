package backend

// CreateAttempt is a create Commit sent that got no HTTP answer: a timeout, a
// reset connection, a cancelled context. Jira may or may not have made the
// issue, and Jira has no idempotency key to ask with, so the draft keeps what
// was sent and when. The next Commit searches for an issue matching it before
// posting again. At is RFC 3339 in UTC; Summary and Type are the values the
// create carried, which a later edit of the draft does not change.
type CreateAttempt struct {
	At      string `json:"at"`
	Summary string `json:"summary"`
	Type    string `json:"type"`
}
