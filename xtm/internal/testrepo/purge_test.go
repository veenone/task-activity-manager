package testrepo_test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"

	"agile-suite/xtm/internal/testrepo"
)

func TestPurgeProfileRemovesOnlyThatProfilesData(t *testing.T) {
	repo := newRepo(t)
	for _, p := range []string{"p1", "p2"} {
		if err := repo.UpsertTests(p, []testrepo.TestCase{
			{Key: "QA-1", ID: "1", Summary: "t"},
		}); err != nil {
			t.Fatalf("seed %s: %v", p, err)
		}
		if err := repo.UpsertContainers(p, []testrepo.Container{
			{Key: "QA-TP-1", Kind: "testplan", Summary: "plan"},
		}); err != nil {
			t.Fatalf("seed container %s: %v", p, err)
		}
		if err := repo.SetTestReview(p, "QA-1", "approved", "x", ""); err != nil {
			t.Fatalf("seed review %s: %v", p, err)
		}
	}

	if err := repo.PurgeProfile("p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}

	p1, _ := repo.ListTests("p1", testrepo.Query{})
	if p1.Total != 0 {
		t.Errorf("p1 tests = %d after purge, want 0", p1.Total)
	}
	if c, _ := repo.ListContainers("p1", "testplan"); len(c) != 0 {
		t.Errorf("p1 containers remain after purge: %v", c)
	}
	if rv, _ := repo.GetTestReview("p1", "QA-1"); rv.Verdict != "" {
		t.Errorf("p1 review remains after purge: %+v", rv)
	}
	if ch, _ := repo.ListPendingChanges("p1"); len(ch) != 0 {
		t.Errorf("p1 pending changes remain after purge: %v", ch)
	}

	// p2 must be untouched.
	p2, _ := repo.ListTests("p2", testrepo.Query{})
	if p2.Total != 1 {
		t.Errorf("p2 tests = %d after purging p1, want 1", p2.Total)
	}
}

// TestPurgeProfileLeavesNoProfileRowBehind seeds one row per profile in every
// table the schema keys by profile, then purges one of them. The table list is
// read from the open database rather than from profileScopedTables, so a table
// missing from that list fails here instead of agreeing with itself.
func TestPurgeProfileLeavesNoProfileRowBehind(t *testing.T) {
	repo, st := newRepoAndStore(t)
	db := st.DB()
	tables := profileKeyedTables(t, db)
	for _, want := range []string{
		"dataset_catalog_state",
		"dataset_catalog_test",
		"dataset_catalog_definition",
		"dataset_catalog_usage",
		"dataset_catalog_exec_plan",
	} {
		if !containsTable(tables, want) {
			t.Errorf("profile-keyed tables missing %s", want)
		}
	}
	if len(tables) < 30 {
		t.Fatalf("found %d profile-keyed tables, want the whole schema", len(tables))
	}
	for _, table := range tables {
		seedProfileRow(t, db, table, "p1", 1)
		seedProfileRow(t, db, table, "p2", 2)
	}

	if err := repo.PurgeProfile("p1"); err != nil {
		t.Fatalf("purge: %v", err)
	}

	var left []string
	for _, table := range tables {
		if countRows(t, db, table, "p1") != 0 {
			left = append(left, table)
		}
		if n := countRows(t, db, table, "p2"); n != 1 {
			t.Errorf("%s holds %d rows for the profile that was not purged, want 1", table, n)
		}
	}
	if len(left) > 0 {
		t.Errorf("purge left rows in %d of %d tables: %s",
			len(left), len(tables), strings.Join(left, ", "))
	}
}

func containsTable(tables []string, want string) bool {
	for _, table := range tables {
		if table == want {
			return true
		}
	}
	return false
}

// profileKeyedTables names every table in the open database that carries a
// profile_id column.
func profileKeyedTables(t *testing.T, db *sql.DB) []string {
	t.Helper()
	rows, err := db.Query(`SELECT m.name FROM sqlite_master m
		WHERE m.type = 'table' AND m.name NOT LIKE 'sqlite_%'
		AND EXISTS (SELECT 1 FROM pragma_table_info(m.name) ti WHERE ti.name = 'profile_id')
		ORDER BY m.name`)
	if err != nil {
		t.Fatalf("list profile-keyed tables: %v", err)
	}
	defer func() { _ = rows.Close() }()
	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			t.Fatalf("scan table name: %v", err)
		}
		names = append(names, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read table names: %v", err)
	}
	return names
}

// seedProfileRow writes one row into table for profileID, giving every column a
// value its declared type accepts. n keeps the two profiles' rows apart in any
// table whose primary key does not include profile_id.
func seedProfileRow(t *testing.T, db *sql.DB, table, profileID string, n int) {
	t.Helper()
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		t.Fatalf("describe %s: %v", table, err)
	}
	defer func() { _ = rows.Close() }()
	var cols []string
	var vals []any
	for rows.Next() {
		var cid, notNull, pk int
		var name, declType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &declType, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan %s column: %v", table, err)
		}
		cols = append(cols, name)
		upper := strings.ToUpper(declType)
		switch {
		case name == "profile_id":
			vals = append(vals, profileID)
		case strings.Contains(upper, "INT"):
			vals = append(vals, n)
		case strings.Contains(upper, "REAL"):
			vals = append(vals, float64(n))
		default:
			vals = append(vals, profileID+"-"+name)
		}
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("read %s columns: %v", table, err)
	}
	placeholders := strings.TrimSuffix(strings.Repeat("?, ", len(cols)), ", ")
	stmt := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		table, strings.Join(cols, ", "), placeholders)
	if _, err := db.Exec(stmt, vals...); err != nil {
		t.Fatalf("seed %s for %s: %v", table, profileID, err)
	}
}

func countRows(t *testing.T, db *sql.DB, table, profileID string) int {
	t.Helper()
	var n int
	if err := db.QueryRow(
		fmt.Sprintf("SELECT COUNT(*) FROM %s WHERE profile_id = ?", table), profileID,
	).Scan(&n); err != nil {
		t.Fatalf("count %s rows: %v", table, err)
	}
	return n
}
