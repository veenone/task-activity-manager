package query

import (
	"context"
	"sync"
)

// Stages a run moves through, as the status bar names them (#452).
const (
	StageIssues = "Fetching issues"
	StageLinks  = "Following links"
)

// Progress is how far a run is: Done of Total in Stage.
type Progress struct {
	Stage string `json:"stage"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

type progressKey struct{}

// WithProgress returns ctx with a reporter that Run, Explore and Follow call
// as pages arrive, one call at a time.
func WithProgress(ctx context.Context, report func(Progress)) context.Context {
	return context.WithValue(ctx, progressKey{}, report)
}

// quiet drops the reporter, for searches whose pages are part of a larger
// stage: a link batch's own pages are not issue pages.
func quiet(ctx context.Context) context.Context {
	return context.WithValue(ctx, progressKey{}, (func(Progress))(nil))
}

// counter adds to Done and reports, under one lock so Done only grows.
type counter struct {
	mu          sync.Mutex
	ctx         context.Context
	stage       string
	done, total int
}

func (c *counter) add(n int) {
	report, _ := c.ctx.Value(progressKey{}).(func(Progress))
	if report == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.done += n
	report(Progress{Stage: c.stage, Done: min(c.done, c.total), Total: c.total})
}
