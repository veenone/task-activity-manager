package jira

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestMyPermissionsReadsEachKeyForTheProject(t *testing.T) {
	var asked string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		asked = r.URL.Path + "?" + r.URL.RawQuery
		_, _ = w.Write([]byte(`{"permissions":{
			"EDIT_ISSUES":{"id":"12","key":"EDIT_ISSUES","name":"Edit Issues","havePermission":true},
			"MANAGE_SPRINTS_PERMISSION":{"id":"x","key":"MANAGE_SPRINTS_PERMISSION","name":"Manage Sprints","havePermission":false}
		}}`))
	}))
	defer srv.Close()

	got, err := NewClient(srv.URL, "tok").MyPermissions(context.Background(), "PLAT")
	if err != nil {
		t.Fatalf("MyPermissions: %v", err)
	}
	if asked != "/rest/api/2/mypermissions?projectKey=PLAT" {
		t.Errorf("asked %s, want the project's permissions", asked)
	}
	if len(got) != 2 || !got["EDIT_ISSUES"] || got["MANAGE_SPRINTS_PERMISSION"] {
		t.Errorf("permissions = %v, want edit held and manage sprints not", got)
	}
}

func TestMyPermissionsReturnsJirasRefusal(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	if got, err := NewClient(srv.URL, "tok").MyPermissions(context.Background(), "NOPE"); err == nil {
		t.Errorf("MyPermissions = %v with no error, want Jira's 404", got)
	}
}
