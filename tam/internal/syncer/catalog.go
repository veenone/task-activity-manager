package syncer

import (
	"context"
	"log"

	"agile-suite/tam/internal/backend"
)

// syncCatalog refreshes the project's fix versions and components. It has
// syncPermissions' contract: a backend that cannot answer, and a read or a
// write that fails, leave the stored rows as they were and never fail the
// sync. The two halves are independent, so one failing still refreshes the
// other.
func (e *Engine) syncCatalog(ctx context.Context, profileID, projectKey string) {
	cb, ok := e.b.(backend.CatalogBackend)
	if !ok {
		return
	}
	if vs, err := cb.ProjectVersions(ctx, projectKey); err != nil {
		log.Printf("tam: read the versions of %s for %s: %v", projectKey, profileID, err)
	} else if err := e.repo.ReplaceVersions(ctx, profileID, projectKey, vs); err != nil {
		log.Printf("tam: store the versions of %s for %s: %v", projectKey, profileID, err)
	}
	if cs, err := cb.ProjectComponents(ctx, projectKey); err != nil {
		log.Printf("tam: read the components of %s for %s: %v", projectKey, profileID, err)
	} else if err := e.repo.ReplaceComponents(ctx, profileID, projectKey, cs); err != nil {
		log.Printf("tam: store the components of %s for %s: %v", projectKey, profileID, err)
	}
}
