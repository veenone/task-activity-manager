package datasetcatalog

import (
	"context"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/jira"
	"agile-suite/xtm/internal/testrepo"
)

// TestADemoCommittedDatasetIsInTheNextScan is #307 as the user met it on
// DEMO2: a dataset added to a test and committed shows up when Scan datasets
// runs, though every call builds a fresh demo backend.
func TestADemoCommittedDatasetIsInTheNextScan(t *testing.T) {
	ctx := context.Background()
	commit := xray.New(jira.NewClient("demo", ""))
	added := backend.Dataset{Parameters: []backend.DatasetParameter{{Name: "BROWSER"}}, Rows: [][]string{{"firefox"}}}
	if err := commit.SetDataset(ctx, "DEMO2-7", added); err != nil {
		t.Fatalf("commit: %v", err)
	}
	repo := catalogRepo(t)
	if err := New(xray.New(jira.NewClient("demo", "")), repo).Sync(ctx, "p1", "DEMO2", nil); err != nil {
		t.Fatalf("scan: %v", err)
	}
	page, err := repo.ListDatasetCatalog("p1", testrepo.DatasetCatalogQuery{Search: "DEMO2-7", Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	for _, tst := range page.Tests {
		if tst.TestKey == "DEMO2-7" {
			return
		}
	}
	t.Fatalf("DEMO2-7 is not in the catalog after its dataset was committed; page = %+v", page.Tests)
}
