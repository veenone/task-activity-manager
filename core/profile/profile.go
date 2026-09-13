// Package profile manages connection profiles and their credentials.
//
// A profile binds the application to one Jira project and connection (FR-5).
// Each profile owns an isolated local dataset; the user switches the active
// profile to switch projects.
package profile

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"agile-suite/core/connection"
)

// ErrNotFound is returned when a profile id does not exist.
var ErrNotFound = errors.New("profile not found")

// Profile is one Jira project connection. Credentials are stored separately in
// the OS credential manager (see CredentialStore) — never in this struct or the
// database.
type Profile struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	JiraURL    string `json:"jiraUrl"`
	ProjectKey string `json:"projectKey"`
	ScopeJQL   string `json:"scopeJql"`
	// CrossProjectSources is a comma-separated list of project keys this profile
	// may link preconditions, test calls, and cloned steps from (across
	// projects, RND_P_4TFINT_05-322). Empty disables cross-project linking.
	CrossProjectSources string `json:"crossProjectSources"`
	BugIssueType        string `json:"bugIssueType"`
	// BugProjectMode is where a filed defect lands: "test" (the test's project,
	// the default), "execution" (the Test Execution's project), or "dedicated"
	// (BugProjectKey).
	BugProjectMode string `json:"bugProjectMode"`
	BugProjectKey  string `json:"bugProjectKey"`
	// CACert is the PEM-encoded CA certificate to add to the TLS trust pool
	// when connecting to this profile's Jira instance (optional). When set,
	// the system roots are extended with this certificate. Takes effect on
	// the next client creation.
	CACert string `json:"caCert"`
	// AllowUntrustedTLS disables TLS certificate verification for this
	// profile's Jira connection. Only use this for trusted internal servers
	// where no CA certificate is available.
	AllowUntrustedTLS bool `json:"allowUntrustedTls"`
	// Backend selects which system this profile talks to: "xray" (default,
	// Jira Data Center + Xray Server/DC) or "kiwi" (Kiwi TCMS). A blank value
	// reads as "xray" (back-compat for rows written before this column
	// existed). For a "kiwi" profile, the credential stored in the OS
	// credential manager holds "username:password" (Kiwi session-login),
	// not a PAT.
	Backend   string    `json:"backend"`
	CreatedAt time.Time `json:"createdAt"`
}

// ConfluenceConfig is the non-secret Confluence connection configuration for a profile.
type ConfluenceConfig struct {
	BaseURL    string `json:"baseURL"`
	SpaceKey   string `json:"spaceKey"`
	RootPageID string `json:"rootPageID"`
}

type RitualAssociation struct {
	BoardID    int    `json:"boardID"`
	SprintID   int    `json:"sprintID"`
	RitualType string `json:"ritualType"`
	PageID     string `json:"pageID"`
	PageTitle  string `json:"pageTitle"`
}

func (m *Manager) CacheConfluencePage(profileID, pageID, payload, fetchedAt string) error {
	_, err := m.db.Exec(`INSERT INTO confluence_page_cache(profile_id, page_id, fetched_at, payload) VALUES(?, ?, ?, ?) ON CONFLICT(profile_id, page_id) DO UPDATE SET fetched_at=excluded.fetched_at, payload=excluded.payload`, profileID, pageID, fetchedAt, payload)
	if err != nil {
		return fmt.Errorf("cache Confluence page: %w", err)
	}
	return nil
}

func (m *Manager) CachedConfluencePage(profileID, pageID string) (payload, fetchedAt string, err error) {
	err = m.db.QueryRow(`SELECT payload, fetched_at FROM confluence_page_cache WHERE profile_id = ? AND page_id = ?`, profileID, pageID).Scan(&payload, &fetchedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return "", "", nil
	}
	if err != nil {
		return "", "", fmt.Errorf("read cached Confluence page: %w", err)
	}
	return payload, fetchedAt, nil
}

func (m *Manager) ListRitualAssociations(profileID string, boardID, sprintID int) ([]RitualAssociation, error) {
	rows, err := m.db.Query(`SELECT board_id, sprint_id, ritual_type, page_id, page_title FROM confluence_association WHERE profile_id = ? AND board_id = ? AND sprint_id = ? ORDER BY ritual_type, page_title`, profileID, boardID, sprintID)
	if err != nil {
		return nil, fmt.Errorf("list ritual associations: %w", err)
	}
	defer rows.Close()
	out := []RitualAssociation{}
	for rows.Next() {
		var a RitualAssociation
		if err := rows.Scan(&a.BoardID, &a.SprintID, &a.RitualType, &a.PageID, &a.PageTitle); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

func (m *Manager) SetRitualAssociation(profileID string, a RitualAssociation) error {
	_, err := m.db.Exec(`INSERT INTO confluence_association(profile_id, board_id, sprint_id, ritual_type, page_id, page_title) VALUES(?, ?, ?, ?, ?, ?) ON CONFLICT(profile_id, board_id, sprint_id, ritual_type, page_id) DO UPDATE SET page_title=excluded.page_title`, profileID, a.BoardID, a.SprintID, strings.TrimSpace(a.RitualType), strings.TrimSpace(a.PageID), strings.TrimSpace(a.PageTitle))
	if err != nil {
		return fmt.Errorf("set ritual association: %w", err)
	}
	return nil
}

func (m *Manager) DeleteRitualAssociation(profileID string, a RitualAssociation) error {
	_, err := m.db.Exec(`DELETE FROM confluence_association WHERE profile_id = ? AND board_id = ? AND sprint_id = ? AND ritual_type = ? AND page_id = ?`, profileID, a.BoardID, a.SprintID, strings.TrimSpace(a.RitualType), strings.TrimSpace(a.PageID))
	if err != nil {
		return fmt.Errorf("delete ritual association: %w", err)
	}
	return nil
}

func (m *Manager) ConfluenceConfig(profileID string) (ConfluenceConfig, error) {
	var c ConfluenceConfig
	err := m.db.QueryRow(`SELECT base_url, space_key, root_page_id FROM confluence_profile WHERE profile_id = ?`, profileID).Scan(&c.BaseURL, &c.SpaceKey, &c.RootPageID)
	if errors.Is(err, sql.ErrNoRows) {
		return ConfluenceConfig{}, nil
	}
	if err != nil {
		return ConfluenceConfig{}, fmt.Errorf("get Confluence settings: %w", err)
	}
	return c, nil
}

func (m *Manager) SetConfluenceConfig(profileID string, c ConfluenceConfig) error {
	_, err := m.db.Exec(`INSERT INTO confluence_profile(profile_id, base_url, space_key, root_page_id) VALUES(?, ?, ?, ?) ON CONFLICT(profile_id) DO UPDATE SET base_url=excluded.base_url, space_key=excluded.space_key, root_page_id=excluded.root_page_id`, profileID, strings.TrimRight(strings.TrimSpace(c.BaseURL), "/"), strings.TrimSpace(c.SpaceKey), strings.TrimSpace(c.RootPageID))
	if err != nil {
		return fmt.Errorf("set Confluence settings: %w", err)
	}
	return nil
}

// Manager is the profile CRUD service backed by the local store (FR-5.1).
//
// Manager also keeps each profile's primary connection row in sync (Phase 6
// bridge task B1's shim seam): the connection table exists so a workspace can
// eventually hold multiple backend connections, but today there is exactly
// one per profile, with id == the profile's id. profiles remains the read
// source of truth for every existing flow — the connection row is a mirror,
// not (yet) consulted by anything, so keeping it in sync here is purely
// additive and behaviour-preserving.
type Manager struct {
	db    *sql.DB
	conns *connection.Manager
}

// NewManager returns a profile manager over the given database, which must
// hold the shareddb tables.
func NewManager(db *sql.DB) *Manager {
	return &Manager{db: db, conns: connection.NewManager(db)}
}

// syncConnection upserts the profile's primary connection row (id == p.ID) so
// it mirrors the profile's current backend fields with role "both". Errors
// are logged-shaped (returned) but callers treat a sync failure as
// non-fatal to the profile write it accompanies, since profiles remains the
// read source of truth.
func (m *Manager) syncConnection(p Profile) error {
	_, err := m.conns.Put(
		p.ID, p.ID, p.Name, p.Backend, p.JiraURL, p.ProjectKey, p.ScopeJQL,
		p.BugIssueType, p.BugProjectMode, p.BugProjectKey, p.CACert, p.AllowUntrustedTLS,
		"both", p.CreatedAt)
	return err
}

// List returns all profiles, ordered by name.
func (m *Manager) List() ([]Profile, error) {
	rows, err := m.db.Query(
		`SELECT id, name, jira_url, project_key, scope_jql, cross_project_sources, bug_issue_type, bug_project_mode, bug_project_key, ca_cert, allow_untrusted_tls, backend, created_at FROM profiles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("list profiles: %w", err)
	}
	defer rows.Close()

	out := []Profile{}
	for rows.Next() {
		p, err := scanProfile(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}

// Get returns one profile by id, or ErrNotFound.
func (m *Manager) Get(id string) (Profile, error) {
	row := m.db.QueryRow(
		`SELECT id, name, jira_url, project_key, scope_jql, cross_project_sources, bug_issue_type, bug_project_mode, bug_project_key, ca_cert, allow_untrusted_tls, backend, created_at FROM profiles WHERE id = ?`, id)
	p, err := scanProfile(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Profile{}, ErrNotFound
	}
	return p, err
}

// Create persists a new profile, generating its id and creation timestamp.
// scopeJQL is an optional JQL fragment that narrows which Tests sync (FR-5.4).
// bugIssueType is the Jira issuetype used when filing a defect (blank defaults
// to "Bug"); bugProjectMode / bugProjectKey choose which project a filed defect
// lands in (blank mode defaults to "test"). caCert is an optional PEM-encoded CA
// certificate to trust when connecting; allowUntrustedTLS disables certificate
// verification (RND_P_4TFINT_05-243). backendType selects the connection
// backend ("xray" default | "kiwi"); blank normalizes to "xray".
func (m *Manager) Create(name, jiraURL, projectKey, scopeJQL, bugIssueType, bugProjectMode, bugProjectKey, caCert string, allowUntrustedTLS bool, backendType string) (Profile, error) {
	p := Profile{
		ID:                uuid.NewString(),
		Name:              name,
		JiraURL:           jiraURL,
		ProjectKey:        projectKey,
		ScopeJQL:          scopeJQL,
		BugIssueType:      bugIssueTypeOrDefault(bugIssueType),
		BugProjectMode:    bugProjectModeOrDefault(bugProjectMode),
		BugProjectKey:     strings.TrimSpace(bugProjectKey),
		CACert:            strings.TrimSpace(caCert),
		AllowUntrustedTLS: allowUntrustedTLS,
		Backend:           backendOrDefault(backendType),
		CreatedAt:         time.Now().UTC(),
	}
	_, err := m.db.Exec(
		`INSERT INTO profiles (id, name, jira_url, project_key, scope_jql, bug_issue_type, bug_project_mode, bug_project_key, ca_cert, allow_untrusted_tls, backend, created_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		p.ID, p.Name, p.JiraURL, p.ProjectKey, p.ScopeJQL, p.BugIssueType, p.BugProjectMode, p.BugProjectKey, p.CACert, boolToInt(p.AllowUntrustedTLS), p.Backend, p.CreatedAt.Format(time.RFC3339))
	if err != nil {
		return Profile{}, fmt.Errorf("create profile: %w", err)
	}
	if err := m.syncConnection(p); err != nil {
		return Profile{}, fmt.Errorf("sync connection for new profile: %w", err)
	}
	return p, nil
}

// bugIssueTypeOrDefault falls back to "Bug" when no issue type is configured, so
// the field is never stored empty.
func bugIssueTypeOrDefault(t string) string {
	if s := strings.TrimSpace(t); s != "" {
		return s
	}
	return "Bug"
}

// bugProjectModeOrDefault normalises the bug-project mode, falling back to "test"
// for blank or unknown values.
func bugProjectModeOrDefault(mode string) string {
	switch strings.TrimSpace(mode) {
	case "execution", "dedicated", "test":
		return strings.TrimSpace(mode)
	default:
		return "test"
	}
}

// backendOrDefault normalises the connection backend, falling back to "xray"
// for blank or unrecognized values (back-compat for rows written before the
// backend column existed).
func backendOrDefault(backendType string) string {
	if strings.TrimSpace(backendType) == "kiwi" {
		return "kiwi"
	}
	return "xray"
}

// Update changes a profile's editable fields — name, Jira URL, project key, JQL
// scope, bug issue type, bug project, and TLS settings (FR-5) — e.g. to correct
// a wrong project key. Returns ErrNotFound if the id doesn't exist. Credentials
// are managed separately. A blank bugIssueType defaults to "Bug"; a blank
// bugProjectMode defaults to "test". caCert and allowUntrustedTLS update the
// TLS trust settings (RND_P_4TFINT_05-243). backendType selects the connection
// backend ("xray" default | "kiwi"); blank normalizes to "xray".
func (m *Manager) Update(id, name, jiraURL, projectKey, scopeJQL, bugIssueType, bugProjectMode, bugProjectKey, caCert string, allowUntrustedTLS bool, backendType string) error {
	res, err := m.db.Exec(
		`UPDATE profiles SET name = ?, jira_url = ?, project_key = ?, scope_jql = ?, bug_issue_type = ?, bug_project_mode = ?, bug_project_key = ?, ca_cert = ?, allow_untrusted_tls = ?, backend = ? WHERE id = ?`,
		name, jiraURL, projectKey, scopeJQL, bugIssueTypeOrDefault(bugIssueType),
		bugProjectModeOrDefault(bugProjectMode), strings.TrimSpace(bugProjectKey),
		strings.TrimSpace(caCert), boolToInt(allowUntrustedTLS), backendOrDefault(backendType), id)
	if err != nil {
		return fmt.Errorf("update profile: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	p, err := m.Get(id)
	if err != nil {
		return fmt.Errorf("reread profile after update: %w", err)
	}
	if err := m.syncConnection(p); err != nil {
		return fmt.Errorf("sync connection after update: %w", err)
	}
	return nil
}

// UpdateScope changes a profile's JQL scope override (FR-5.4).
func (m *Manager) UpdateScope(id, scopeJQL string) error {
	res, err := m.db.Exec(
		`UPDATE profiles SET scope_jql = ? WHERE id = ?`, scopeJQL, id)
	if err != nil {
		return fmt.Errorf("update profile scope: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	p, err := m.Get(id)
	if err != nil {
		return fmt.Errorf("reread profile after scope update: %w", err)
	}
	if err := m.syncConnection(p); err != nil {
		return fmt.Errorf("sync connection after scope update: %w", err)
	}
	return nil
}

// UpdateCrossProjectSources sets a profile's comma-separated list of source
// project keys for cross-project linking (RND_P_4TFINT_05-322).
func (m *Manager) UpdateCrossProjectSources(id, sources string) error {
	res, err := m.db.Exec(
		`UPDATE profiles SET cross_project_sources = ? WHERE id = ?`,
		strings.TrimSpace(sources), id)
	if err != nil {
		return fmt.Errorf("update cross-project sources: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	return nil
}

// Delete removes a profile, or returns ErrNotFound. Its primary connection
// row is removed alongside it; an already-missing connection (e.g. a profile
// created before the connection table existed on a database that has not
// yet reopened to run the v43 backfill) is tolerated, not an error.
func (m *Manager) Delete(id string) error {
	res, err := m.db.Exec(`DELETE FROM profiles WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete profile: %w", err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotFound
	}
	if err := m.conns.Delete(id); err != nil && !errors.Is(err, connection.ErrNotFound) {
		return fmt.Errorf("delete connection for profile: %w", err)
	}
	// TODO(xtm): cascade-delete this profile's test_case / sync_state rows (FR-5.3).
	return nil
}

// scanner abstracts *sql.Row and *sql.Rows so scanProfile serves both Get and List.
type scanner interface {
	Scan(dest ...any) error
}

func scanProfile(s scanner) (Profile, error) {
	var (
		p                 Profile
		allowUntrustedInt int
		created           string
	)
	if err := s.Scan(&p.ID, &p.Name, &p.JiraURL, &p.ProjectKey, &p.ScopeJQL, &p.CrossProjectSources, &p.BugIssueType, &p.BugProjectMode, &p.BugProjectKey, &p.CACert, &allowUntrustedInt, &p.Backend, &created); err != nil {
		return Profile{}, err
	}
	p.AllowUntrustedTLS = allowUntrustedInt != 0
	p.Backend = backendOrDefault(p.Backend)
	p.CreatedAt, _ = time.Parse(time.RFC3339, created)
	return p, nil
}

// boolToInt converts a bool to a SQLite-compatible integer (1 / 0).
func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
