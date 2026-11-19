package jira

import (
	"context"

	"agile-suite/tam/internal/backend"
)

var _ backend.PermissionBackend = (*Backend)(nil)

// Permissions reads what the token may do in the project.
func (b *Backend) Permissions(ctx context.Context, projectKey string) (map[string]bool, error) {
	return b.c.MyPermissions(ctx, projectKey)
}
