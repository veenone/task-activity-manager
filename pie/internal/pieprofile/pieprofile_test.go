package pieprofile

import (
	"path/filepath"
	"testing"

	"agile-suite/core/shareddb"
)

func TestValidateNew(t *testing.T) {
	cases := []struct {
		name, label, url, token string
		ok                      bool
	}{
		{"complete", "Quality", "https://jira.example.com/", "tok", true},
		{"no project key is needed", "Q", "http://jira.example.com", "tok", true},
		{"blank name", "  ", "https://jira.example.com", "tok", false},
		{"blank url", "Q", " ", "tok", false},
		{"url without a scheme", "Q", "jira.example.com", "tok", false},
		{"unsupported scheme", "Q", "ftp://jira.example.com", "tok", false},
		{"blank token", "Q", "https://jira.example.com", " ", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if err := ValidateNew(c.label, c.url, c.token); (err == nil) != c.ok {
				t.Fatalf("err = %v, want ok = %v", err, c.ok)
			}
		})
	}
}

// PIE's profiles have no project key, and TAM and XTM read every row of the
// shared database, so PIE must never open that file.
func TestDefaultPathIsNotTheSharedDatabase(t *testing.T) {
	t.Setenv("AppData", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	shared, err := shareddb.DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	mine, err := DefaultPath()
	if err != nil {
		t.Fatal(err)
	}
	if mine == shared {
		t.Fatalf("PIE opens the shared database at %s", shared)
	}
	if filepath.Dir(mine) != filepath.Dir(shared) {
		t.Fatalf("%s is outside the suite's folder %s", mine, filepath.Dir(shared))
	}
}
