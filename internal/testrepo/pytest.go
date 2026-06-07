package testrepo

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
)

// GeneratePytest builds a pytest scaffold from a Test Set / Plan / Execution
// (FR-7.2): one test function per member Test, named from its key, with the
// Test's summary and steps in the docstring and an @pytest.mark.xray marker
// linking it back to the Xray key (FR-7.6). Step bodies come from the cached
// steps — open or refresh a Test to populate them.
func (r *Repository) GeneratePytest(profileID, containerKey string) (string, error) {
	var kind, summary string
	err := r.db.QueryRow(
		`SELECT kind, summary FROM test_container WHERE profile_id = ? AND jira_key = ?`,
		profileID, containerKey,
	).Scan(&kind, &summary)
	if errors.Is(err, sql.ErrNoRows) {
		return "", fmt.Errorf("container %s not found", containerKey)
	}
	if err != nil {
		return "", fmt.Errorf("read container: %w", err)
	}

	members, err := r.db.Query(
		`SELECT t.jira_key, t.summary
		 FROM test_container_test l
		 JOIN test_case t ON t.profile_id = l.profile_id AND t.jira_key = l.test_key
		 WHERE l.profile_id = ? AND l.container_key = ?
		 ORDER BY t.jira_key`,
		profileID, containerKey)
	if err != nil {
		return "", fmt.Errorf("read members: %w", err)
	}
	defer members.Close()

	var b strings.Builder
	fmt.Fprintf(&b, "\"\"\"pytest scaffold generated from %s — %s.\n\n", containerKey, summary)
	fmt.Fprintf(&b, "One test per Xray Test in this %s. Fill in the bodies and run with pytest.\n\"\"\"\n",
		containerLabel(kind))
	b.WriteString("import pytest\n\n")

	used := map[string]int{}
	count := 0
	for members.Next() {
		var key, testSummary string
		if err := members.Scan(&key, &testSummary); err != nil {
			return "", err
		}
		count++
		fn := uniquePyName(used, key)
		steps, err := r.ListTestSteps(profileID, key)
		if err != nil {
			return "", err
		}
		writePyTest(&b, key, fn, testSummary, steps)
	}
	if err := members.Err(); err != nil {
		return "", err
	}
	if count == 0 {
		b.WriteString("\n# (no tests in this container yet)\n")
	}
	return b.String(), nil
}

func writePyTest(b *strings.Builder, key, fn, summary string, steps []Step) {
	fmt.Fprintf(b, "\n@pytest.mark.xray(%q)\n", key)
	fmt.Fprintf(b, "def %s():\n", fn)
	fmt.Fprintf(b, "    \"\"\"%s\n", pyDocLine(summary))
	if len(steps) > 0 {
		b.WriteString("\n    Steps:\n")
		for i, s := range steps {
			fmt.Fprintf(b, "    %d. %s\n", i+1, pyDocLine(s.Action))
			if strings.TrimSpace(s.Data) != "" {
				fmt.Fprintf(b, "       Data: %s\n", pyDocLine(s.Data))
			}
			if strings.TrimSpace(s.Expected) != "" {
				fmt.Fprintf(b, "       Expected: %s\n", pyDocLine(s.Expected))
			}
		}
	}
	b.WriteString("    \"\"\"\n")
	b.WriteString("    pytest.skip(\"scaffold — implement me\")\n")
}

// uniquePyName builds a valid, unique Python function name from a Test key.
func uniquePyName(used map[string]int, key string) string {
	var sb strings.Builder
	sb.WriteString("test_")
	for _, r := range strings.ToLower(key) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			sb.WriteRune(r)
		} else {
			sb.WriteByte('_')
		}
	}
	base := sb.String()
	used[base]++
	if n := used[base]; n > 1 {
		return fmt.Sprintf("%s_%d", base, n)
	}
	return base
}

// pyDocLine collapses a value to a single docstring-safe line.
func pyDocLine(s string) string {
	s = strings.ReplaceAll(s, "\"\"\"", "'''")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

func containerLabel(kind string) string {
	switch kind {
	case "testset":
		return "Test Set"
	case "testplan":
		return "Test Plan"
	case "testexec":
		return "Test Execution"
	}
	return "container"
}
