package profile

// CredentialStore persists per-profile secrets (a PAT or password) in the
// operating system credential manager — never in the database, never in
// plaintext, never in logs (FR-8.3, NFR-3).
type CredentialStore interface {
	// Save stores the secret for a profile.
	Save(profileID, secret string) error
	// Load retrieves the secret for a profile.
	Load(profileID string) (string, error)
	// Delete removes the secret for a profile.
	Delete(profileID string) error
}

// windowsCredentialStore backs CredentialStore with Windows Credential Manager
// (DPAPI-protected entries).
//
// TODO(xtm): implement using github.com/danieljoos/wincred (FR-8.3).
type windowsCredentialStore struct{}

// NewCredentialStore returns the OS-native credential store. Windows is the
// only supported platform (Q5 / NFR-6).
func NewCredentialStore() CredentialStore {
	return &windowsCredentialStore{}
}

func (w *windowsCredentialStore) Save(profileID, secret string) error {
	return ErrNotImplemented
}

func (w *windowsCredentialStore) Load(profileID string) (string, error) {
	return "", ErrNotImplemented
}

func (w *windowsCredentialStore) Delete(profileID string) error {
	return ErrNotImplemented
}
