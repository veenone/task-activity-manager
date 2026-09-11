// Package sprintreport assembles what the Reports view puts on screen: one
// sprint's reconstructed series, the board's velocity table, and the reason
// there is neither when there is neither.
//
// It is the orchestration around internal/reports rather than part of it.
// internal/reports takes issues and a clock and touches no I/O, which is
// what makes its tests cheap; everything that costs something lives here:
// the board's own rule for what finished means, the paged changelog fetch,
// the stored series a closed sprint is served from, the velocity table
// assembled out of both, and the progress frames a fetch that takes minutes
// has to send. The binding in app_reports.go does what every other app*.go
// does and no more, which is to resolve a profile, take the lock, call this
// and adapt the error.
//
// Two rules run through the whole package.
//
// A closed sprint's series is stored and served from the store; a live
// sprint's is neither. A closed sprint cannot change, so reading yesterday's
// copy answers the same question today for none of the cost. A live sprint
// changed an hour ago, so the stored copy would be a confident wrong answer,
// and that is the case the guard in series.go is written about.
//
// A condition the view has to render beside the report travels in the
// result, not as a Go error. Wails fills in either a bound method's value or
// its error and never both, the same limit sprints.Completion is built
// around, so a sprint with no readable dates or a board that was never
// synced comes back as a Report carrying an Unavailable reason and nothing
// else. A failure of the call itself, a refused lock, a transport error
// partway through a fetch, a database that will not answer, stays a Go
// error. The reasons are constants rather than sentences because the
// frontend words them.
package sprintreport

import (
	"context"
	"time"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/reports"
)

// pageIssues is how many issues one page of a report's search asks for.
//
// Twenty five, against the sync's fifty, because the changelog expansion
// makes each issue's payload several times the size of the row the grid
// syncs: the same page count costs several times the bytes and several
// times the time. This is a considered default and not a measured one. The
// probe that would have timed 50 against 25 on a real instance, step 4 of
// this phase's wire probe, has not been run, so nobody here knows where the
// real knee is.
const pageIssues = 25

// Why a report has nothing to show. Each is a property of the data the user
// is looking at rather than a failure of the call, and the view renders each
// one differently, which is why they are told apart here instead of being
// flattened into one empty answer.
const (
	// ReasonBoardNotSynced is a board whose columns cannot say what
	// finished means: no columns cached at all, or a last column that
	// collects no status. It is answered before anything is fetched,
	// because a changelog TAM could not classify afterwards is minutes
	// spent on nothing.
	ReasonBoardNotSynced = "boardNotSynced"
	// ReasonSprintNotFound is a sprint id no cached sprint of this board
	// carries. A sprint deleted in Jira, and a board whose sprint list the
	// view is holding an older copy of, both read as this.
	ReasonSprintNotFound = "sprintNotFound"
	// ReasonNoDates is a sprint whose own start or end date cannot be
	// read, which is reports.ErrNoDates reaching here. There is nothing to
	// reconstruct: a series is a walk between two dates.
	ReasonNoDates = "sprintHasNoDates"
	// ReasonNoClosedSprint is a board that has never closed a sprint, and
	// so has neither a report to open on nor a velocity table. It is the
	// answer to a call that named no sprint of its own.
	ReasonNoClosedSprint = "noClosedSprint"
)

// The phases a progress frame belongs to.
const (
	// PhaseSprint is the fetch of the sprint the user asked for.
	PhaseSprint = "sprint"
	// PhaseVelocity is the fetch of one of the older sprints the velocity
	// table needs and the store did not already hold.
	PhaseVelocity = "velocity"
)

// Progress is one frame of a report's own progress, and it is deliberately
// not syncer.Progress on syncProgressEvent. The shell's banner reads that
// event, so emitting on it would have the whole app announce a sync while a
// read that is not one is running, which is the shape of the bug
// tam/CLAUDE.md's "One lock, both ends" section records.
//
// It carries no sentence. Which sprint is being read and how far along it is
// are facts; the wording is the view's, the same way the reasons above are.
type Progress struct {
	Phase      string `json:"phase"`
	SprintID   int    `json:"sprintId"`
	SprintName string `json:"sprintName"`
	// Fetched and Total are issues of this one sprint, not of the report:
	// a report covering six sprints sends each one's frames under its own
	// name rather than one total nobody could check.
	Fetched int `json:"fetched"`
	Total   int `json:"total"`
	// Done marks the last frame of one sprint's fetch.
	Done bool `json:"done"`
}

// Report is what the Reports view draws.
//
// Unavailable is empty for every report that has one. When it is set it
// holds one of the reasons above, Series and Velocity are empty, and the
// view renders that reason instead: there is no half a report here, and a
// series with no days in it would otherwise be indistinguishable from a
// sprint in which nothing happened.
type Report struct {
	Series   reports.Series        `json:"series"`
	Velocity []reports.VelocityRow `json:"velocity"`
	// BuiltAt is when the series was reconstructed, in RFC 3339, which for
	// a closed sprint served from the store is when it was first built and
	// not now. It is carried because a user reading a report cannot
	// otherwise tell a fresh answer from one this profile stored weeks ago.
	BuiltAt     string `json:"builtAt"`
	Unavailable string `json:"unavailable"`
}

// Store is what a report needs from the board cache: the columns that
// define finished, the board's sprints, and the two halves of the stored
// series. boardrepo.Repository satisfies it.
//
// It takes boardrepo's own types rather than translating them, because
// SavedReport is where BuiltAt comes from and the sprint rows are the only
// record TAM keeps of a closed sprint at all.
type Store interface {
	Columns(ctx context.Context, profileID string, boardID int) ([]backend.BoardColumn, error)
	ListSprints(ctx context.Context, profileID string, boardID int) ([]boardrepo.Sprint, error)
	SaveReport(ctx context.Context, profileID string, boardID int, series reports.Series) error
	Report(ctx context.Context, profileID string, boardID, sprintID int) (boardrepo.SavedReport, bool, error)
}

// Service builds reports for one profile's backend and board cache.
//
// Now and Loc are taken rather than reached for, the way syncer.Engine
// takes its clock and reports.Build takes both of these: a live sprint's
// walk stops at Now and its days bucket at midnight in Loc, so a service
// that read the machine's own would be untestable in the one case that
// changes by the hour. New fills in the machine's, which is what the
// binding wants.
type Service struct {
	b     backend.HistoryBackend
	store Store

	// PageSize is how many issues one page of the changelog search asks
	// for, pageIssues by default and a field of its own so a test can page
	// a fixture of three issues.
	PageSize int
	Now      func() time.Time
	Loc      *time.Location
	// Progress receives one frame per page fetched, for the sprint asked
	// for and for each older sprint the velocity table has to fetch. It is
	// nil in every test that does not look at it, which is why each emit
	// goes through emit below.
	Progress func(Progress)
}

// New builds a service with the default page size, the machine's clock and
// the machine's zone.
func New(b backend.HistoryBackend, store Store) *Service {
	return &Service{b: b, store: store, PageSize: pageIssues, Now: time.Now, Loc: time.Local}
}

// emit sends one frame if anyone is listening.
func (s *Service) emit(p Progress) {
	if s.Progress != nil {
		s.Progress(p)
	}
}

// unavailable is a report that is only its reason.
func unavailable(reason string) Report {
	return Report{Velocity: []reports.VelocityRow{}, Unavailable: reason}
}
