package demo

import (
	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

// withDescription fills in the description the dataset holds for a row, so a
// demo profile's Backlog carries the field a real sync now caches on it. A
// row the dataset has no detail for keeps a nil description, which is the
// same "nobody has read this" a real profile shows before its next sync
// rather than a fabricated empty one. Callers hold b.mu.
func (b *Backend) withDescription(iss backend.Issue) backend.Issue {
	if text, ok := b.desc[iss.Key]; ok {
		iss.Description = &text
		return iss
	}
	if d, ok := demo.Detail(b.project, iss.Key); ok {
		text := d.Description
		iss.Description = &text
	}
	return iss
}
