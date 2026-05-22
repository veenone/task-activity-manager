package profile

import (
	"fmt"

	"github.com/danieljoos/wincred"
)

// credentialPrefix namespaces this app's entries in Windows Credential Manager.
const credentialPrefix = "xray-test-manager:"

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

// windowsCredentialStore backs CredentialStore with Windows Credential Manager,
// whose entries are DPAPI-protected per user account.
type windowsCredentialStore struct{}

// NewCredentialStore returns the OS-native credential store. Windows is the
// only supported platform (Q5 / NFR-6).
func NewCredentialStore() CredentialStore {
	return &windowsCredentialStore{}
}

func (w *windowsCredentialStore) Save(profileID, secret string) error {
	cred := wincred.NewGenericCredential(credentialPrefix + profileID)
	cred.CredentialBlob = []byte(secret)
	if err := cred.Write(); err != nil {
		return fmt.Errorf("store credential: %w", err)
	}
	return nil
}

func (w *windowsCredentialStore) Load(profileID string) (string, error) {
	cred, err := wincred.GetGenericCredential(credentialPrefix + profileID)
	if err != nil {
		return "", fmt.Errorf("load credential: %w", err)
	}
	return string(cred.CredentialBlob), nil
}

func (w *windowsCredentialStore) Delete(profileID string) error {
	cred, err := wincred.GetGenericCredential(credentialPrefix + profileID)
	if err != nil {
		return fmt.Errorf("find credential to delete: %w", err)
	}
	if err := cred.Delete(); err != nil {
		return fmt.Errorf("delete credential: %w", err)
	}
	return nil
}
