package demo

import (
	"context"
	"slices"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

var _ backend.CatalogBackend = (*Backend)(nil)

// ProjectVersions answers the dataset's fix versions under the profile's
// project key, with this run's version writes on them.
func (b *Backend) ProjectVersions(context.Context, string) ([]backend.Version, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return slices.Clone(b.versionList()), nil
}

// ProjectComponents answers the dataset's components under the profile's
// project key.
func (b *Backend) ProjectComponents(context.Context, string) ([]backend.Component, error) {
	return demo.Components(b.project), nil
}
