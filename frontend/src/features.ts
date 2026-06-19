// Feature flags for capabilities that are built but intentionally hidden in the
// UI. Flip a flag to surface the feature again — no other code change needed.

// Test review / sign-off (verdict + reviewer + note). The backend, local store,
// commit path (a Jira comment, Phase 7), the Browse review filter and the
// requirement sign-off audit export all remain; this flag only gates the
// user-facing entry points. As a standalone tool XTM can't enforce a review
// workflow, so the surface is hidden until a team process makes it useful.
export const REVIEW_ENABLED = false;
