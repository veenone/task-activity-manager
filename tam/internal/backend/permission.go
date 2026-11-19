package backend

import "context"

// PermissionBackend is a backend that can say what the signed-in account may
// do in a project. It is optional: the demo backend has no accounts and
// leaves it out, which reads as nothing known and so everything allowed.
type PermissionBackend interface {
	// Permissions answers every permission the backend reports for the
	// project, by key, with whether the account holds it.
	Permissions(ctx context.Context, projectKey string) (map[string]bool, error)
}
