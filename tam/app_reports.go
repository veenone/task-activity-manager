package main

import (
	"context"
	"errors"
	"log"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/boardrepo"
	"agile-suite/tam/internal/errtext"
	"agile-suite/tam/internal/sprintreport"
)

// The sprint report: one bound read, laid out the way the sprint writes in
// app_sprintmanage.go are, because it takes the same guards they do. What
// it does not share with them is where the work lives. A report is paging,
// a done rule, a cache and a velocity table assembled out of two sources,
// which is internal/sprintreport's job; this file resolves the profile,
// checks the backend can answer at all, takes the lock, hands over a
// context the view can cancel, and reduces whatever came back to one line.
//
// reportProgressEvent is its own event and not syncProgressEvent. The
// shell's banner listens on the sync's event, so a report emitting there
// would have the whole app announce a sync while a read that is not one is
// running, on a profile whose sync has not started. That is the same class
// of bug tam/CLAUDE.md's "One lock, both ends" section was written about,
// where two sides of one invariant disagreed about what was happening.
const reportProgressEvent = "tam:report-progress"

// The board cache is what a report reads its columns, sprints and stored
// series through. This is the one file that imports both packages, so it is
// where the check that they still fit belongs, the same way app.go holds
// the check for boardrepo's own read seam.
var _ sprintreport.Store = (*boardrepo.Repository)(nil)

// GetSprintReport is the sprint's reconstructed series and its board's
// velocity table, together, in one call.
//
// Together because the per-profile lock refuses rather than waits: the two
// calls a view makes on mount would have raced for it, and one of them
// would have been refused every single time the view opened.
//
// sprintID may be zero, which asks for the board's most recent closed
// sprint, the report the morning after a sprint closes starts from. A
// negative one is answered as a sprint that does not exist rather than as
// that default, for the reason choose in internal/sprintreport gives.
//
// refresh builds the report again from Jira instead of from whatever is
// stored, and writes what it built back. It is what a user presses when a
// report is wrong: a stored series is only ever as good as the fetch that
// made it, and nothing else in TAM can replace one that reports.AlgoVersion
// still considers current. It costs the whole table's fetch, so it belongs
// on a control the user reaches for and not on the view's mount.
//
// A board that was never synced, a sprint the cache does not hold and a
// sprint whose dates cannot be read all come back as a Report naming that
// reason with no error, because Wails fills in either a bound method's
// value or its error and never both, and each of those is a fact about the
// data on screen that the view has to render. An error here means the call
// itself failed: no profile, a connection that cannot read a changelog, a
// lock held by something else, a transport failure partway through, or a
// database that would not answer. The view keeps its last report and names
// the failure.
//
// It takes the lock under its own name, "report". acquire reports the name
// that is HELD rather than the one refused, which is what the five sprint
// bindings share theirs to exploit; a report is a read and not one of those
// writes, so it gets its own, and what that buys is a sync or a commit
// refused during a report reading "a report is already running for this
// profile" rather than naming an operation nobody started.
func (a *App) GetSprintReport(profileID string, boardID, sprintID int, refresh bool) (sprintreport.Report, error) {
	p, b, err := a.backendForProfile(profileID)
	if err != nil {
		return sprintreport.Report{}, err
	}
	// The changelog is a capability, reached by assertion the way SyncBoards
	// reaches BoardBackend, so a backend that cannot expand one is refused
	// with a sentence here rather than carrying a stub method for it.
	history, ok := b.(backend.HistoryBackend)
	if !ok {
		return sprintreport.Report{}, errors.New("this connection cannot read an issue's history, so there is nothing to build a sprint report from")
	}
	if err := a.acquire(p.ID, "report"); err != nil {
		log.Printf("tam: report for sprint %d on board %d refused for %s: %v", sprintID, boardID, p.Name, err)
		return sprintreport.Report{}, err
	}
	defer a.release(p.ID)

	ctx, finished := a.beginReport(p.ID)
	defer finished()

	// Logged on the way in as well as out, the rule every long call in this
	// app follows: this is the slowest read TAM makes, and one that never
	// returns would otherwise leave no trace that it had started.
	log.Printf("tam: report for sprint %d on board %d started for %s (%s)", sprintID, boardID, p.Name, p.ProjectKey)
	service := sprintreport.New(history, a.boards)
	service.Progress = a.emitReportProgress
	report, err := service.Build(ctx, p.ID, boardID, sprintID, refresh)
	if err != nil {
		log.Printf("tam: report for sprint %d on board %d for %s failed: %v", sprintID, boardID, p.Name, err)
		return sprintreport.Report{}, errors.New(errtext.Line(err))
	}
	if report.Unavailable != "" {
		log.Printf("tam: no report for sprint %d on board %d for %s: %s", sprintID, boardID, p.Name, report.Unavailable)
		return report, nil
	}
	log.Printf("tam: report for sprint %d on board %d for %s built, with %d velocity row(s)", report.Series.SprintID, boardID, p.Name, len(report.Velocity))
	return report, nil
}

// CancelSprintReport stops the report running for this profile, if one is.
//
// Wails hands a bound method no per-call context, so the report is given one
// built here and this is the other end of it. It exists because the lock
// refuses rather than waits: a report still fetching six sprints of
// changelog after the user has left the view would hold the profile against
// their next sync or commit for minutes, for a screen nobody is looking at.
// The view calls it when it unmounts.
//
// Calling it when no report is running does nothing.
func (a *App) CancelSprintReport(profileID string) {
	a.backendMu.Lock()
	cancel := a.reportCancels[profileID]
	delete(a.reportCancels, profileID)
	a.backendMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// beginReport builds the context one report runs under and records its
// cancel func where CancelSprintReport can find it. The second return both
// clears that record and releases the context's own resources, and the
// caller defers it.
//
// The map is guarded by backendMu, the mutex the busy guard is already
// under, and it is made on the first report rather than in initStore
// because nothing but a report ever touches it.
//
// Clearing the entry rather than only cancelling is what keeps a later
// report from being cancelled by an earlier call's func. There is at most
// one entry per profile to begin with, since the lock this runs under
// refuses a second report, and a cancel that has already fired removed its
// own entry on the way through.
//
// a.ctx is nil until Wails calls startup, which is the same thing
// emitReportProgress below guards for and the same thing app.go's menu
// helpers guard for. context.WithCancel panics on a nil parent rather than
// returning an error, so the report gets a background context instead: a
// report with no Wails runtime behind it cannot emit a frame, but it can
// still be run and still be cancelled, and that is what a unit test holds.
func (a *App) beginReport(profileID string) (context.Context, func()) {
	parent := a.ctx
	if parent == nil {
		parent = context.Background()
	}
	ctx, cancel := context.WithCancel(parent)
	a.backendMu.Lock()
	if a.reportCancels == nil {
		a.reportCancels = map[string]context.CancelFunc{}
	}
	a.reportCancels[profileID] = cancel
	a.backendMu.Unlock()
	return ctx, func() {
		a.backendMu.Lock()
		delete(a.reportCancels, profileID)
		a.backendMu.Unlock()
		cancel()
	}
}

// emitReportProgress forwards one frame to the frontend on the report's own
// event. The nil check is emitProgress's, for the reason given there: a
// context with no Wails runtime behind it, which is what a unit test holds,
// makes Wails call log.Fatal rather than return an error, so the frame is
// dropped instead.
func (a *App) emitReportProgress(p sprintreport.Progress) {
	if a.ctx == nil || a.ctx.Value("events") == nil {
		return
	}
	runtime.EventsEmit(a.ctx, reportProgressEvent, p)
}
