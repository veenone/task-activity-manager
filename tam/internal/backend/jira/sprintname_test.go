package jira

import (
	"encoding/json"
	"testing"
)

// A sprint is routinely named for its team in brackets. Stopping the name at
// the first "]" cut "[SGRS] Sprint 12" to "[SGRS", which is what the grid
// showed for RND_P_4TFINT_05-107.
func TestParseSprintKeepsBracketsInTheName(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want string
		id   string
	}{
		{
			name: "bracketed team prefix",
			raw: `["com.atlassian.greenhopper.service.sprint.Sprint@1e0a4[id=1234,rapidViewId=45,` +
				`state=ACTIVE,name=[SGRS] Sprint 12,startDate=2026-01-01T00:00:00.000Z,` +
				`endDate=2026-01-15T00:00:00.000Z,completeDate=<null>,sequence=1234,goal=]"]`,
			want: "[SGRS] Sprint 12",
			id:   "1234",
		},
		{
			name: "name last, closing bracket only",
			raw:  `["Sprint@1[id=7,state=CLOSED,name=[OPS] Hardening]"]`,
			want: "[OPS] Hardening",
			id:   "7",
		},
		{
			name: "a comma inside the name that is not a key",
			raw:  `["Sprint@1[id=9,state=ACTIVE,name=Sprint 3, second half,startDate=2026-01-01]"]`,
			want: "Sprint 3, second half",
			id:   "9",
		},
		{
			name: "plain name still reads as before",
			raw:  `["Sprint@1[id=12,state=ACTIVE,name=Sprint 12,startDate=2026-01-01]"]`,
			want: "Sprint 12",
			id:   "12",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			id, name := parseSprint(json.RawMessage(c.raw))
			if name != c.want {
				t.Errorf("name = %q, want %q", name, c.want)
			}
			if id != c.id {
				t.Errorf("id = %q, want %q", id, c.id)
			}
		})
	}
}

// The modern object shape is untouched by any of this.
func TestParseSprintStillReadsTheObjectShape(t *testing.T) {
	id, name := parseSprint(json.RawMessage(`[{"id":42,"name":"[SGRS] Sprint 12"}]`))
	if id != "42" || name != "[SGRS] Sprint 12" {
		t.Errorf("id = %q name = %q", id, name)
	}
}
