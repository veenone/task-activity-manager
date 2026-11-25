package demo

import (
	"context"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

var _ backend.CatalogBackend = (*Backend)(nil)

// ProjectVersions answers the dataset's fix versions under the profile's
// project key.
func (b *Backend) ProjectVersions(context.Context, string) ([]backend.Version, error) {
	return demo.Versions(b.project), nil
}

// ProjectComponents answers the dataset's components under the profile's
// project key.
func (b *Backend) ProjectComponents(context.Context, string) ([]backend.Component, error) {
	return demo.Components(b.project), nil
}
