package syncer

import (
	"context"
	"log"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/permission"
)

// syncPermissions reads what the token may do in the project and stores the
// permissions TAM gates an action on. A backend that cannot say, and a read
// or a write that fails, leave the stored answer as it was and never fail
// the sync: what the app may offer is not a reason to lose the issues.
func (e *Engine) syncPermissions(ctx context.Context, profileID, projectKey string) {
	pb, ok := e.b.(backend.PermissionBackend)
	if !ok {
		return
	}
	all, err := pb.Permissions(ctx, projectKey)
	if err != nil {
		log.Printf("tam: read the permissions of %s for %s: %v", projectKey, profileID, err)
		return
	}
	if err := e.repo.PutPermissions(ctx, profileID, projectKey, permission.Gated(all)); err != nil {
		log.Printf("tam: store the permissions of %s for %s: %v", projectKey, profileID, err)
	}
}
