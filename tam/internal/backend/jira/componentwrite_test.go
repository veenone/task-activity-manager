package jira_test

import (
	"context"
	"net/http"
	"testing"

	"agile-suite/tam/internal/backend"
)

func TestComponentWritesConvertIDs(t *testing.T) {
	b, got, _ := versionBackend(t, func(r *http.Request) (int, string) {
		switch r.Method {
		case http.MethodPost:
			return http.StatusCreated, `{"id":"10300","name":"Reports","assigneeType":"COMPONENT_LEAD","lead":{"name":"alee","displayName":"A. Lee"}}`
		case http.MethodDelete:
			return http.StatusNoContent, ""
		}
		return http.StatusOK, `{"id":"10000","name":"Login","assigneeType":"PROJECT_DEFAULT"}`
	})
	var cb backend.ComponentBackend = b
	ctx := context.Background()
	made, err := cb.CreateComponent(ctx, "PLAT", backend.ComponentInput{Name: "Reports", LeadName: "alee", AssigneeType: "COMPONENT_LEAD"})
	if err != nil {
		t.Fatal(err)
	}
	if made.ID != 10300 || made.Project != "PLAT" || made.LeadDisplayName != "A. Lee" || made.AssigneeType != "COMPONENT_LEAD" {
		t.Fatalf("made %+v", made)
	}
	if _, err := cb.UpdateComponent(ctx, 10000, backend.ComponentInput{Name: "Login", AssigneeType: "PROJECT_DEFAULT"}); err != nil {
		t.Fatal(err)
	}
	if err := cb.DeleteComponent(ctx, 10000, 10300); err != nil {
		t.Fatal(err)
	}
	if err := cb.DeleteComponent(ctx, 10001, 0); err != nil {
		t.Fatal(err)
	}
	calls := *got
	if len(calls) != 4 {
		t.Fatalf("calls %+v", calls)
	}
	if calls[0].path != "/rest/api/2/component" || calls[0].body["project"] != "PLAT" || calls[0].body["leadUserName"] != "alee" {
		t.Errorf("create %+v", calls[0])
	}
	if calls[1].method != http.MethodPut || calls[1].path != "/rest/api/2/component/10000" {
		t.Errorf("update %+v", calls[1])
	}
	if calls[2].path != "/rest/api/2/component/10000" || calls[2].query != "moveIssuesTo=10300" {
		t.Errorf("delete with move %+v", calls[2])
	}
	if calls[3].query != "" {
		t.Errorf("delete without move %+v", calls[3])
	}
}
