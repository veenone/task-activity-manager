package backend

import "fmt"

type DatasetSourceKind string

const (
	DatasetSourceTest      DatasetSourceKind = "test"
	DatasetSourcePlan      DatasetSourceKind = "plan"
	DatasetSourceExecution DatasetSourceKind = "execution"
)

type ParameterizedRun struct {
	ID                string            `json:"id"`
	ExecutionKey      string            `json:"executionKey"`
	TestKey           string            `json:"testKey"`
	DatasetSourceKind DatasetSourceKind `json:"datasetSourceKind"`
	DatasetSourceKey  string            `json:"datasetSourceKey"`
	Status            string            `json:"status"`
	Iterations        []RunIteration    `json:"iterations"`
}

type RunIteration struct {
	ID         string               `json:"id"`
	Rank       int                  `json:"rank"`
	Status     string               `json:"status"`
	Parameters []IterationParameter `json:"parameters"`
	Steps      []IterationStep      `json:"steps"`
}

type IterationParameter struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

type IterationStep struct {
	ID           string `json:"id"`
	DefinitionID string `json:"definitionId"`
	Rank         int    `json:"rank"`
	Status       string `json:"status"`
	Action       string `json:"action"`
	Data         string `json:"data"`
	Expected     string `json:"expected"`
}

// ExpandDataset deterministically expands a dataset for demo fixtures and
// unit tests. Live backends return the iterations and their stable ids.
func ExpandDataset(dataset Dataset, max int) ([]RunIteration, error) {
	if max < 1 {
		return nil, fmt.Errorf("iteration limit must be positive")
	}
	if len(dataset.Parameters) == 0 {
		return []RunIteration{}, nil
	}
	for rowIndex, row := range dataset.Rows {
		if len(row) != len(dataset.Parameters) {
			return nil, fmt.Errorf("dataset row %d has %d values, want %d", rowIndex+1, len(row), len(dataset.Parameters))
		}
	}

	combinationIndexes := make([]int, 0, len(dataset.Parameters))
	seedIndexes := make([]int, 0, len(dataset.Parameters))
	for index, parameter := range dataset.Parameters {
		if parameter.Combinatorial {
			combinationIndexes = append(combinationIndexes, index)
		} else {
			seedIndexes = append(seedIndexes, index)
		}
	}
	if len(combinationIndexes) == 0 {
		if len(dataset.Rows) > max {
			return nil, fmt.Errorf("dataset expands to %d iterations, limit is %d", len(dataset.Rows), max)
		}
		iterations := make([]RunIteration, 0, len(dataset.Rows))
		for _, row := range dataset.Rows {
			iterations = append(iterations, iterationFromValues(dataset.Parameters, row, len(iterations)))
		}
		return iterations, nil
	}

	seeds := distinctProjectedRows(dataset.Rows, seedIndexes)
	if len(seedIndexes) == 0 {
		seeds = [][]string{{}}
	}
	valueSets := make([][]string, len(combinationIndexes))
	for setIndex, columnIndex := range combinationIndexes {
		valueSets[setIndex] = distinctColumnValues(dataset.Rows, columnIndex)
	}

	total := len(seeds)
	for _, values := range valueSets {
		if len(values) == 0 {
			return []RunIteration{}, nil
		}
		if total > max/len(values) {
			return nil, fmt.Errorf("dataset expands above the %d iteration limit", max)
		}
		total *= len(values)
	}
	iterations := make([]RunIteration, 0, total)
	for _, seed := range seeds {
		values := make([]string, len(dataset.Parameters))
		for index, columnIndex := range seedIndexes {
			values[columnIndex] = seed[index]
		}
		appendCombinations(dataset.Parameters, combinationIndexes, valueSets, values, 0, &iterations)
	}
	return iterations, nil
}

func iterationFromValues(parameters []DatasetParameter, values []string, rank int) RunIteration {
	resolved := make([]IterationParameter, len(parameters))
	for index, parameter := range parameters {
		resolved[index] = IterationParameter{Name: parameter.Name, Value: values[index]}
	}
	return RunIteration{Rank: rank, Parameters: resolved, Steps: []IterationStep{}}
}

func appendCombinations(parameters []DatasetParameter, indexes []int, valueSets [][]string, values []string, depth int, iterations *[]RunIteration) {
	if depth == len(indexes) {
		*iterations = append(*iterations, iterationFromValues(parameters, values, len(*iterations)))
		return
	}
	for _, value := range valueSets[depth] {
		values[indexes[depth]] = value
		appendCombinations(parameters, indexes, valueSets, values, depth+1, iterations)
	}
}

func distinctProjectedRows(rows [][]string, indexes []int) [][]string {
	result := make([][]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		projected := make([]string, len(indexes))
		for index, columnIndex := range indexes {
			projected[index] = row[columnIndex]
		}
		key := fmt.Sprintf("%q", projected)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		result = append(result, projected)
	}
	return result
}

func distinctColumnValues(rows [][]string, columnIndex int) []string {
	values := make([]string, 0, len(rows))
	seen := make(map[string]struct{}, len(rows))
	for _, row := range rows {
		value := row[columnIndex]
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}
