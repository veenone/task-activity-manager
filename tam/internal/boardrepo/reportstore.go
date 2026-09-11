package boardrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"agile-suite/tam/internal/reports"
)

// SavedReport is one sprint's report as tam.db holds it. Unit rides beside
// Series rather than only inside it, because a caller listing several
// sprints' reports at once, the way a velocity table does, wants each row's
// unit without decoding every series_json first.
type SavedReport struct {
	Series  reports.Series `json:"series"`
	Unit    string         `json:"unit"`
	BuiltAt string         `json:"builtAt"`
}

const upsertReportSQL = `
	INSERT INTO sprint_report (profile_id, board_id, sprint_id, unit, algo_version, built_at, series_json)
	VALUES (?, ?, ?, ?, ?, ?, ?)
	ON CONFLICT(profile_id, board_id, sprint_id) DO UPDATE SET
		unit = excluded.unit, algo_version = excluded.algo_version,
		built_at = excluded.built_at, series_json = excluded.series_json`

const selectReportSQL = `
	SELECT unit, algo_version, built_at, series_json FROM sprint_report
	WHERE profile_id = ? AND board_id = ? AND sprint_id = ?`

// SaveReport stores series as one board's copy of a sprint's report, keyed
// by board as well as sprint id for the reason sprint_report's own table
// comment gives: Jira hands one sprint to every board whose filter reaches
// it, and a report's done rule comes from its own board's last column, so a
// key without the board would let this write serve board B a series built
// for board A.
//
// It stamps the row with reports.AlgoVersion, the package's own constant,
// rather than a version the caller supplies, so a build made under an older
// binary can never be mistaken for a current one: Report only ever has to
// compare what is stored against what this package currently is.
func (r *Repository) SaveReport(ctx context.Context, profileID string, boardID int, series reports.Series) error {
	blob, err := json.Marshal(series)
	if err != nil {
		return fmt.Errorf("encode board %d sprint %d report: %w", boardID, series.SprintID, err)
	}
	stamp := time.Now().UTC().Format(time.RFC3339)
	if _, err := r.db.ExecContext(ctx, upsertReportSQL,
		profileID, boardID, series.SprintID, series.Unit, reports.AlgoVersion, stamp, string(blob),
	); err != nil {
		return fmt.Errorf("save board %d sprint %d report: %w", boardID, series.SprintID, err)
	}
	return nil
}

// Report reads back one board's copy of a sprint's report. The second
// return is false both when nothing has been stored yet and when the row
// that is there was written by an older reports.AlgoVersion: either way the
// caller's answer is the same, rebuild the series and call SaveReport
// again, which replaces the stale row with a current one under the same
// key. A closed sprint's series never changes on its own, so nothing here
// needs to tell those two cases apart.
func (r *Repository) Report(ctx context.Context, profileID string, boardID, sprintID int) (SavedReport, bool, error) {
	var (
		unit    string
		version int
		builtAt string
		blob    string
	)
	err := r.db.QueryRowContext(ctx, selectReportSQL, profileID, boardID, sprintID).
		Scan(&unit, &version, &builtAt, &blob)
	if errors.Is(err, sql.ErrNoRows) {
		return SavedReport{}, false, nil
	}
	if err != nil {
		return SavedReport{}, false, fmt.Errorf("read board %d sprint %d report: %w", boardID, sprintID, err)
	}
	if version != reports.AlgoVersion {
		return SavedReport{}, false, nil
	}
	var series reports.Series
	if err := json.Unmarshal([]byte(blob), &series); err != nil {
		return SavedReport{}, false, fmt.Errorf("decode board %d sprint %d report: %w", boardID, sprintID, err)
	}
	return SavedReport{Series: series, Unit: unit, BuiltAt: builtAt}, true, nil
}
