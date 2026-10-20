package jira

import (
	"strings"
	"sync"
)

// demoDatasets holds the datasets a demo profile has committed, keyed by demo
// URL and test key (#307). XTM builds a fresh client for every call, so a
// write kept on the client would be gone by the next read; a real Xray keeps
// the write on the server, and this is the demo's stand-in for that server,
// for as long as the app runs. Keying by URL keeps two demo profiles apart.
var demoDatasets sync.Map

func demoDatasetKey(baseURL, testKey string) string {
	return strings.ToLower(strings.TrimSpace(baseURL)) + "\x00" + testKey
}

// demoCurrentDataset is the dataset a demo profile committed for testKey, or
// the fixture's when it committed none; an emptied dataset is a commit too.
func demoCurrentDataset(baseURL, testKey string) Dataset {
	if v, ok := demoDatasets.Load(demoDatasetKey(baseURL, testKey)); ok {
		return v.(Dataset)
	}
	return demoDataset(testKey)
}

func storeDemoDataset(baseURL, testKey string, d Dataset) {
	demoDatasets.Store(demoDatasetKey(baseURL, testKey), d)
}

// resetDemoDatasets forgets every demo commit, for tests.
func resetDemoDatasets() {
	demoDatasets.Range(func(k, _ any) bool {
		demoDatasets.Delete(k)
		return true
	})
}
