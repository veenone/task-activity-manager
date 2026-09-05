package sharedmigrate

import (
	"database/sql"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"agile-suite/core/shareddb"
	xtmstore "agile-suite/xtm/internal/store"
)

// The shared database was lifted from XTM's own tables, and XTM upstream keeps
// changing them. This test fails as soon as XTM's copy of a table has a column
// the shared schema lacks, or the import stops naming every column, so the
// drift shows up in CI instead of as a setting that silently never reaches the
// shared file.
func TestSharedSchemaTracksXTMTables(t *testing.T) {
	dir := t.TempDir()
	src, err := xtmstore.Open(filepath.Join(dir, "xtm.db"))
	if err != nil {
		t.Fatalf("open xtm store: %v", err)
	}
	defer src.Close()
	dst, err := shareddb.Open(filepath.Join(dir, "profiles.db"))
	if err != nil {
		t.Fatalf("open shared db: %v", err)
	}
	defer dst.Close()

	for _, table := range Tables {
		xtmCols := tableColumns(t, src.DB(), table)
		sharedCols := tableColumns(t, dst.DB(), table)
		for _, c := range xtmCols {
			if !contains(sharedCols, c) {
				t.Errorf("%s: XTM has column %q but core/shareddb does not; add it to shareddb.Schema and to sharedmigrate.Columns", table, c)
			}
		}
		imported := append([]string(nil), Columns[table]...)
		sort.Strings(imported)
		want := append([]string(nil), xtmCols...)
		sort.Strings(want)
		if strings.Join(imported, ",") != strings.Join(want, ",") {
			t.Errorf("%s: import copies %v but XTM's table has %v", table, imported, want)
		}
	}
}

func tableColumns(t *testing.T, db *sql.DB, table string) []string {
	t.Helper()
	rows, err := db.Query(`PRAGMA table_info(` + table + `)`)
	if err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}
	defer rows.Close()
	var cols []string
	for rows.Next() {
		var (
			cid     int
			name    string
			typ     string
			notNull int
			dflt    sql.NullString
			pk      int
		)
		if err := rows.Scan(&cid, &name, &typ, &notNull, &dflt, &pk); err != nil {
			t.Fatalf("scan table_info %s: %v", table, err)
		}
		cols = append(cols, name)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("table_info %s: %v", table, err)
	}
	return cols
}

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}
