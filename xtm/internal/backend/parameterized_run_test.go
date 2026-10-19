package backend

import (
	"reflect"
	"strings"
	"testing"
)

func TestExpandDatasetAppliesOrderedDistinctCombinatorialValuesToSeedRows(t *testing.T) {
	dataset := Dataset{
		Parameters: []DatasetParameter{
			{Name: "USER"},
			{Name: "REGION", Combinatorial: true},
			{Name: "PRIORITY", Combinatorial: true},
		},
		Rows: [][]string{
			{"alice", "eu", "high"},
			{"bob", "us", "low"},
			{"alice", "eu", "low"},
			{"bob", "us", "high"},
			{"alice", "eu", "high"},
		},
	}

	got, err := ExpandDataset(dataset, 1000)
	if err != nil {
		t.Fatalf("ExpandDataset: %v", err)
	}
	want := [][]IterationParameter{
		{{Name: "USER", Value: "alice"}, {Name: "REGION", Value: "eu"}, {Name: "PRIORITY", Value: "high"}},
		{{Name: "USER", Value: "alice"}, {Name: "REGION", Value: "eu"}, {Name: "PRIORITY", Value: "low"}},
		{{Name: "USER", Value: "alice"}, {Name: "REGION", Value: "us"}, {Name: "PRIORITY", Value: "high"}},
		{{Name: "USER", Value: "alice"}, {Name: "REGION", Value: "us"}, {Name: "PRIORITY", Value: "low"}},
		{{Name: "USER", Value: "bob"}, {Name: "REGION", Value: "eu"}, {Name: "PRIORITY", Value: "high"}},
		{{Name: "USER", Value: "bob"}, {Name: "REGION", Value: "eu"}, {Name: "PRIORITY", Value: "low"}},
		{{Name: "USER", Value: "bob"}, {Name: "REGION", Value: "us"}, {Name: "PRIORITY", Value: "high"}},
		{{Name: "USER", Value: "bob"}, {Name: "REGION", Value: "us"}, {Name: "PRIORITY", Value: "low"}},
	}
	if len(got) != len(want) {
		t.Fatalf("iterations = %d, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Rank != i || !reflect.DeepEqual(got[i].Parameters, want[i]) {
			t.Errorf("iteration %d = rank %d %+v, want rank %d %+v", i, got[i].Rank, got[i].Parameters, i, want[i])
		}
		if got[i].Steps == nil {
			t.Errorf("iteration %d Steps is nil", i)
		}
	}
}

func TestExpandDatasetUsesOneEmptySeedForCombinatorialOnlyDataset(t *testing.T) {
	got, err := ExpandDataset(Dataset{
		Parameters: []DatasetParameter{{Name: "BROWSER", Combinatorial: true}},
		Rows:       [][]string{{"chrome"}, {"firefox"}, {"chrome"}},
	}, 1000)
	if err != nil {
		t.Fatalf("ExpandDataset: %v", err)
	}
	want := []RunIteration{
		{Rank: 0, Parameters: []IterationParameter{{Name: "BROWSER", Value: "chrome"}}, Steps: []IterationStep{}},
		{Rank: 1, Parameters: []IterationParameter{{Name: "BROWSER", Value: "firefox"}}, Steps: []IterationStep{}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("iterations = %+v, want %+v", got, want)
	}
}

func TestExpandDatasetRejectsNonRectangularRows(t *testing.T) {
	_, err := ExpandDataset(Dataset{
		Parameters: []DatasetParameter{{Name: "USER"}, {Name: "REGION"}},
		Rows:       [][]string{{"alice"}},
	}, 1000)
	if err == nil || !strings.Contains(err.Error(), "row 1") {
		t.Fatalf("error = %v, want row 1 validation", err)
	}
}

func TestExpandDatasetEnforcesLimitBeforeBuildingIterations(t *testing.T) {
	values := make([][]string, 1001)
	for i := range values {
		values[i] = []string{string(rune(i + 1))}
	}
	dataset := Dataset{
		Parameters: []DatasetParameter{{Name: "VALUE", Combinatorial: true}},
		Rows:       values,
	}
	if got, err := ExpandDataset(dataset, 1000); err == nil || got != nil {
		t.Fatalf("1,001 expansion = (%v, %v), want nil validation error", got, err)
	}

	dataset.Rows = dataset.Rows[:1000]
	got, err := ExpandDataset(dataset, 1000)
	if err != nil || len(got) != 1000 {
		t.Fatalf("1,000 expansion = (%d, %v), want success", len(got), err)
	}
}

func TestExpandDatasetReturnsNonNilEmptySlices(t *testing.T) {
	got, err := ExpandDataset(Dataset{}, 1000)
	if err != nil {
		t.Fatalf("ExpandDataset: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Fatalf("iterations = %#v, want non-nil empty slice", got)
	}
}
