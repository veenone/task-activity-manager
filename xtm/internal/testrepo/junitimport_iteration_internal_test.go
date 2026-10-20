package testrepo

import (
	"strings"
	"testing"

	"agile-suite/xtm/internal/backend"
)

func iterationRun(parameters ...backend.IterationParameter) backend.ParameterizedRun {
	return backend.ParameterizedRun{Iterations: []backend.RunIteration{{ID: "iteration-1", Parameters: parameters}}}
}

func TestParseJUnitPropertiesDecodesAndPreservesOrder(t *testing.T) {
	cases, err := parseJUnitXML([]byte(`<testsuite><testcase name="login"><properties>` +
		`<property name=" USER " value=" Alice &amp; Bob "/><property name="REGION" value="東京"/>` +
		`</properties></testcase></testsuite>`))
	if err != nil || len(cases) != 1 {
		t.Fatalf("parse = %+v, err=%v", cases, err)
	}
	want := []JUnitProperty{{Name: "USER", Value: "Alice & Bob"}, {Name: "REGION", Value: "東京"}}
	if len(cases[0].Properties) != len(want) {
		t.Fatalf("properties = %+v, want %+v", cases[0].Properties, want)
	}
	for i := range want {
		if cases[0].Properties[i] != want[i] {
			t.Errorf("property %d = %+v, want %+v", i, cases[0].Properties[i], want[i])
		}
	}
}

func TestMatchIterationRequiresACompleteExactSignature(t *testing.T) {
	run := iterationRun(
		backend.IterationParameter{Name: "USER", Value: "alice"},
		backend.IterationParameter{Name: "REGION", Value: "eu"},
	)
	tests := []struct {
		name       string
		properties []JUnitProperty
		wantID     string
		wantReason string
	}{
		{name: "complete with unrelated property", properties: []JUnitProperty{{Name: "build", Value: "42"}, {Name: "USER", Value: "alice"}, {Name: "REGION", Value: "eu"}}, wantID: "iteration-1"},
		{name: "partial", properties: []JUnitProperty{{Name: "USER", Value: "alice"}}, wantReason: "missing"},
		{name: "unknown value", properties: []JUnitProperty{{Name: "USER", Value: "bob"}, {Name: "REGION", Value: "eu"}}, wantReason: "no iteration"},
		{name: "duplicate property", properties: []JUnitProperty{{Name: "USER", Value: "alice"}, {Name: "USER", Value: "alice"}, {Name: "REGION", Value: "eu"}}, wantReason: "duplicate"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			id, signature, reason := matchIteration(tc.properties, run)
			if id != tc.wantID || (tc.wantReason != "" && !strings.Contains(reason, tc.wantReason)) {
				t.Fatalf("match = (%q, %+v, %q), want id=%q reason containing %q", id, signature, reason, tc.wantID, tc.wantReason)
			}
			if id != "" && len(signature) != 2 {
				t.Fatalf("signature = %+v, want complete ordered signature", signature)
			}
		})
	}
}

func TestMatchIterationRejectsDuplicateIterationSignature(t *testing.T) {
	run := backend.ParameterizedRun{Iterations: []backend.RunIteration{
		{ID: "iteration-1", Parameters: []backend.IterationParameter{{Name: "USER", Value: "alice"}}},
		{ID: "iteration-2", Parameters: []backend.IterationParameter{{Name: "USER", Value: "alice"}}},
	}}
	id, _, reason := matchIteration([]JUnitProperty{{Name: "USER", Value: "alice"}}, run)
	if id != "" || !strings.Contains(reason, "multiple") {
		t.Fatalf("match = (%q, %q), want ambiguous duplicate signature", id, reason)
	}
}
