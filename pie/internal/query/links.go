package query

import (
	"context"
	"slices"
	"strings"

	corejira "agile-suite/core/jira"
)

// keysPerSearch keeps a key in (...) clause well inside the length of a URL.
const keysPerSearch = 100

// Set is a linked set: issues joined by links that cross projects, followed in
// either direction. It is out of step when at least one issue is in Jira's done
// category and at least one is not.
type Set struct {
	Keys      []string `json:"keys"`
	Projects  []string `json:"projects"`
	OutOfStep bool     `json:"outOfStep"`
}

// Explore runs the JQL, then fetches the issues its rows link to in other
// projects, and theirs, until no new key appears or MaxRows is reached. Unless
// confirmed, a query matching more than AskAbove issues fetches nothing and
// comes back with NeedsConfirm. extra names the profile's custom fields;
// linked issues read them too, because reach counts product bases.
func Explore(ctx context.Context, client *corejira.Client, jql string, confirmed bool, extra Fields) (Result, error) {
	if !confirmed && strings.TrimSpace(jql) != "" {
		page, err := client.SearchIssues(ctx, jql, []string{"key"}, nil, 0, 1)
		if err != nil {
			return Result{}, readable(err)
		}
		if page.Total > AskAbove {
			return Result{Total: page.Total, Issues: []Issue{}, NeedsConfirm: true}, nil
		}
	}
	res, err := run(ctx, client, jql, extra)
	if err != nil {
		return Result{}, err
	}
	return Follow(ctx, client, res, extra), nil
}

// Follow fetches the issues res links to in other projects, and theirs,
// until no new key appears or MaxRows is reached, and groups the result into
// sets. res holds the rows to start from; Refresh links passes a stored run's
// own rows, so only the linked issues are fetched again. A failed search
// stops the walk and is reported in LinkError, with the rows kept.
func Follow(ctx context.Context, client *corejira.Client, res Result, extra Fields) Result {
	res.LinkError, res.LinksTruncated = "", false
	have := map[string]bool{}
	for _, is := range res.Issues {
		have[is.Key] = true
	}
	asked := map[string]bool{}
	for {
		var pending []string
		for _, is := range res.Issues {
			for _, l := range is.Links {
				if crosses(is, l) && !have[l.Key] && !asked[l.Key] {
					asked[l.Key] = true
					pending = append(pending, l.Key)
				}
			}
		}
		if len(pending) == 0 {
			break
		}
		if room := MaxRows - len(res.Issues); len(pending) > room {
			pending, res.LinksTruncated = pending[:room], true
		}
		batches := slices.Collect(slices.Chunk(pending, keysPerSearch))
		got := make([][]Issue, len(batches))
		errs := make([]error, len(batches))
		_ = parallel(len(batches), func(i int) error {
			r, err := run(ctx, client, "key in ("+strings.Join(batches[i], ", ")+")", extra)
			got[i], errs[i] = r.Issues, err
			return err
		})
		// Batches before the first failure are kept, as a sequential walk
		// would have kept them, so a failure costs only what came after it.
		for i := range batches {
			if errs[i] != nil {
				res.LinkError = errs[i].Error()
				res.Sets = Sets(res.Issues)
				return res
			}
			for _, is := range got[i] {
				if !have[is.Key] {
					have[is.Key] = true
					is.Matched = false
					res.Issues = append(res.Issues, is)
				}
			}
		}
		if res.LinksTruncated {
			break
		}
	}
	res.Sets = Sets(res.Issues)
	return res
}

// Sets groups issues into linked sets, in the order their first issue appears.
// A link to an issue that was not fetched is left out.
func Sets(issues []Issue) []Set {
	index := map[string]int{}
	for i, is := range issues {
		index[is.Key] = i
	}
	parent := make([]int, len(issues))
	for i := range parent {
		parent[i] = i
	}
	var root func(int) int
	root = func(i int) int {
		for parent[i] != i {
			parent[i] = parent[parent[i]]
			i = parent[i]
		}
		return i
	}
	for i, is := range issues {
		for _, l := range is.Links {
			if j, ok := index[l.Key]; ok && crosses(is, l) {
				parent[root(j)] = root(i)
			}
		}
	}
	members := map[int][]int{}
	var order []int
	for i := range issues {
		r := root(i)
		if members[r] == nil {
			order = append(order, r)
		}
		members[r] = append(members[r], i)
	}
	var out []Set
	for _, r := range order {
		if len(members[r]) < 2 {
			continue
		}
		var s Set
		done, open := false, false
		for _, i := range members[r] {
			is := issues[i]
			s.Keys = append(s.Keys, is.Key)
			if !slices.Contains(s.Projects, is.Project) {
				s.Projects = append(s.Projects, is.Project)
			}
			if is.StatusCategory == "done" {
				done = true
			} else {
				open = true
			}
		}
		slices.Sort(s.Projects)
		s.OutOfStep = done && open
		out = append(out, s)
	}
	return out
}

// crosses reports whether a link leaves the issue's project. A key's project
// is everything before its last dash.
func crosses(is Issue, l Link) bool {
	i := strings.LastIndex(l.Key, "-")
	return i > 0 && l.Key[:i] != is.Project
}
