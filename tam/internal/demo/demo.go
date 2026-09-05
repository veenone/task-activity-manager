// Package demo is the deterministic Acme Platform dataset behind the demo
// profile: a dozen hand-written issues that match the design mockup plus
// generated filler, so the grid has pages to turn and the filters have
// something to hide. Keys, ids, and timestamps are fixed so tests can assert
// on them.
package demo

import (
	"fmt"
	"math/rand"
	"strings"
	"time"

	"agile-suite/tam/internal/backend"
)

// ProjectKey is the project the curated issues are written for. Issues and
// Detail substitute the profile's real key so a demo profile with any key
// works.
const ProjectKey = "PLAT"

const seed = 20260905

var base = time.Date(2026, 9, 1, 9, 0, 0, 0, time.UTC)

func pts(v float64) *float64 { return &v }

// curated are the issues the mockup shows, in backlog order. Keys use the
// PLAT prefix; Issues rewrites them for the profile's project key.
var curated = []backend.Issue{
	{Key: "PLAT-412", Type: backend.TypeStory, Summary: "Checkout: apply promo code at payment step", Status: "In Progress", Assignee: "R. Anand", Reporter: "PO", Priority: "High", Labels: []string{"checkout", "promo"}, SprintID: "12", SprintName: "Sprint 12", ParentKey: "PLAT-350", StoryPoints: pts(5)},
	{Key: "PLAT-409", Type: backend.TypeTask, Summary: "Rotate payment gateway API keys", Status: "To Do", Reporter: "M. Ortiz", Priority: "Medium", Labels: []string{"security"}, SprintID: "12", SprintName: "Sprint 12", ParentKey: "PLAT-310", StoryPoints: pts(2)},
	{Key: "PLAT-401", Type: backend.TypeBug, Summary: "Order total wrong when VAT changes mid-session", Status: "In Progress", Assignee: "M. Ortiz", Reporter: "S. Kim", Priority: "Highest", Labels: []string{"checkout"}, SprintID: "12", SprintName: "Sprint 12", ParentKey: "PLAT-320", StoryPoints: pts(3)},
	{Key: "PLAT-388", Type: backend.TypeRequirement, Summary: "Promo codes must be single-use per customer", Status: "Approved", Assignee: "PO", Reporter: "PO", Priority: "High", Labels: []string{"promo"}, ParentKey: "PLAT-350"},
	{Key: "PLAT-350", Type: backend.TypeEpic, Summary: "Promotions and discounts", Status: "In Progress", Assignee: "PO", Reporter: "PO", Priority: "High", Labels: []string{"promo"}, StoryPoints: pts(21)},
	{Key: "PLAT-347", Type: backend.TypeTask, Summary: "Write retro notes template", Status: "To Do", Assignee: "S. Kim", Reporter: "S. Kim", Priority: "Low", Labels: []string{"process"}, SprintID: "13", SprintName: "Sprint 13", ParentKey: "PLAT-305", StoryPoints: pts(1)},
	{Key: "PLAT-331", Type: backend.TypeStory, Summary: "Guest checkout without an account", Status: "Done", Assignee: "R. Anand", Reporter: "PO", Priority: "Medium", Labels: []string{"checkout"}, SprintID: "11", SprintName: "Sprint 11", ParentKey: "PLAT-320", StoryPoints: pts(8)},
	{Key: "PLAT-398", Type: backend.TypeStory, Summary: "Show the discount breakdown on the order summary", Status: "To Do", Reporter: "PO", Priority: "Medium", Labels: []string{"promo", "checkout"}, SprintID: "13", SprintName: "Sprint 13", ParentKey: "PLAT-350", StoryPoints: pts(3)},
	{Key: "PLAT-395", Type: backend.TypeRequirement, Summary: "Every payment request must carry an idempotency key", Status: "Approved", Assignee: "PO", Reporter: "M. Ortiz", Priority: "Highest", Labels: []string{"payments"}, ParentKey: "PLAT-310"},
	{Key: "PLAT-390", Type: backend.TypeBug, Summary: "Promo code field accepts whitespace-only input", Status: "To Do", Reporter: "J. Park", Priority: "Low", Labels: []string{"promo"}, SprintID: "13", SprintName: "Sprint 13", ParentKey: "PLAT-350", StoryPoints: pts(1)},
	{Key: "PLAT-385", Type: backend.TypeTask, Summary: "Add the promo code analytics event", Status: "Done", Assignee: "M. Ortiz", Reporter: "PO", Priority: "Medium", Labels: []string{"promo", "analytics"}, SprintID: "11", SprintName: "Sprint 11", ParentKey: "PLAT-350", StoryPoints: pts(2)},
	{Key: "PLAT-320", Type: backend.TypeEpic, Summary: "Checkout experience", Status: "In Progress", Assignee: "PO", Reporter: "PO", Priority: "High", Labels: []string{"checkout"}, StoryPoints: pts(34)},
	{Key: "PLAT-310", Type: backend.TypeEpic, Summary: "Payments platform hardening", Status: "To Do", Assignee: "PO", Reporter: "M. Ortiz", Priority: "Medium", Labels: []string{"payments", "security"}, StoryPoints: pts(13)},
	{Key: "PLAT-305", Type: backend.TypeEpic, Summary: "Team rituals and process", Status: "Done", Assignee: "S. Kim", Reporter: "S. Kim", Priority: "Low", Labels: []string{"process"}, StoryPoints: pts(5)},
}

// descriptions and links for the curated issues. Test keys use the XT
// prefix, the project XTM's own demo dataset uses, so the seam reads the
// way it will against a real suite.
var curatedDetails = map[string]backend.IssueDetail{
	"PLAT-412": {
		Description: "As a shopper I can enter a promo code on the payment step and see the discount before I pay.\n\nAcceptance: the code is validated against the promotions service; an invalid or expired code shows an inline message and leaves the total unchanged.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-1018", Summary: "Promo code applies discount", IssueType: "Test"},
			{Direction: "inward", Type: "Tested By", Key: "XT-1019", Summary: "Expired promo code rejected", IssueType: "Test"},
			{Direction: "outward", Type: "Relates", Key: "PLAT-388", Summary: "Promo codes must be single-use per customer", IssueType: "Requirement"},
		},
	},
	"PLAT-388": {
		Description: "A promo code may be redeemed once per customer account. A second attempt is rejected with a message that names the earlier order.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-1020", Summary: "Promo code single-use enforced", IssueType: "Test"},
			{Direction: "inward", Type: "Relates", Key: "PLAT-412", Summary: "Checkout: apply promo code at payment step", IssueType: "Story"},
		},
	},
	"PLAT-331": {
		Description: "As a shopper without an account I can complete checkout with an email address only.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-990", Summary: "Guest checkout completes", IssueType: "Test"},
		},
	},
	"PLAT-395": {
		Description: "Every request to the payment gateway carries an idempotency key derived from the order id, so a retried request never charges twice.",
		Links: []backend.Link{
			{Direction: "inward", Type: "Tested By", Key: "XT-1031", Summary: "Retried payment is not charged twice", IssueType: "Test"},
		},
	},
	"PLAT-401": {
		Description: "Steps: add an item, change the shipping country to one with a different VAT rate, return to the summary. The total still shows the old VAT amount.",
		Links: []backend.Link{
			{Direction: "outward", Type: "Relates", Key: "PLAT-331", Summary: "Guest checkout without an account", IssueType: "Story"},
		},
	},
}

var (
	fillerTypes     = []string{backend.TypeTask, backend.TypeStory, backend.TypeBug, backend.TypeRequirement, backend.TypeTask, backend.TypeStory}
	fillerAreas     = []string{"cart", "search", "account", "shipping", "returns", "catalogue", "notifications", "invoicing"}
	fillerAssignees = []string{"R. Anand", "M. Ortiz", "S. Kim", "J. Park", ""}
	fillerStatuses  = []string{"To Do", "To Do", "In Progress", "Done"}
	fillerPriority  = []string{"Low", "Medium", "Medium", "High"}
	fillerSprints   = []string{"", "11", "12", "13"}
	fillerPoints    = []float64{1, 2, 3, 5, 8}
	fillerEpics     = []string{"PLAT-320", "PLAT-310", "PLAT-350", ""}
)

func fillerSummary(typ, area string, n int) string {
	switch typ {
	case backend.TypeStory:
		return fmt.Sprintf("As a shopper I can use the %s page on a phone (%d)", area, n)
	case backend.TypeBug:
		return fmt.Sprintf("The %s page loses its state after a refresh (%d)", area, n)
	case backend.TypeRequirement:
		return fmt.Sprintf("The %s service must answer within two seconds (%d)", area, n)
	default:
		return fmt.Sprintf("Update the %s docs and dashboards (%d)", area, n)
	}
}

// Issues returns the whole dataset, curated issues first in backlog order,
// then the filler, all rewritten to projectKey.
func Issues(projectKey string) []backend.Issue {
	out := make([]backend.Issue, 0, 60)
	for i, c := range curated {
		iss := c
		iss.Key = rekey(iss.Key, projectKey)
		iss.ParentKey = rekey(iss.ParentKey, projectKey)
		iss.Labels = nonNil(iss.Labels)
		iss.Project = projectKey
		iss.ID = fmt.Sprintf("%d", 10000+i)
		iss.Rank = fmt.Sprintf("0|i%04d:", i)
		iss.Created = base.Add(-time.Duration(30+i) * 24 * time.Hour).Format(time.RFC3339)
		iss.Updated = base.Add(time.Duration(4*24-i) * time.Hour).Format(time.RFC3339)
		out = append(out, iss)
	}
	rng := rand.New(rand.NewSource(seed))
	for n := 0; len(out) < 60; n++ {
		typ := fillerTypes[n%len(fillerTypes)]
		area := fillerAreas[rng.Intn(len(fillerAreas))]
		iss := backend.Issue{
			Key:      fmt.Sprintf("%s-%d", projectKey, 250+n),
			ID:       fmt.Sprintf("%d", 20000+n),
			Project:  projectKey,
			Type:     typ,
			Summary:  fillerSummary(typ, area, n),
			Status:   fillerStatuses[rng.Intn(len(fillerStatuses))],
			Assignee: fillerAssignees[rng.Intn(len(fillerAssignees))],
			Reporter: "PO",
			Priority: fillerPriority[rng.Intn(len(fillerPriority))],
			Labels:   []string{area},
			Rank:     fmt.Sprintf("0|i%04d:", len(curated)+n),
			Created:  base.Add(-time.Duration(60+n) * 24 * time.Hour).Format(time.RFC3339),
			Updated:  base.Add(-time.Duration(n) * 6 * time.Hour).Format(time.RFC3339),
		}
		if typ == backend.TypeRequirement {
			iss.Status = []string{"Draft", "Approved"}[rng.Intn(2)]
		} else {
			if s := fillerSprints[rng.Intn(len(fillerSprints))]; s != "" {
				iss.SprintID = s
				iss.SprintName = "Sprint " + s
			}
			if rng.Intn(4) != 0 {
				iss.StoryPoints = pts(fillerPoints[rng.Intn(len(fillerPoints))])
			}
		}
		if e := fillerEpics[rng.Intn(len(fillerEpics))]; e != "" {
			iss.ParentKey = rekey(e, projectKey)
		}
		out = append(out, iss)
	}
	return out
}

// Detail returns the description and links for key. Curated issues have
// hand-written details; filler issues get a short generated one.
func Detail(projectKey, key string) (backend.IssueDetail, bool) {
	// Curated details are keyed by their PLAT keys; map the profile's key back.
	canonical := key
	if strings.HasPrefix(key, projectKey+"-") {
		canonical = ProjectKey + strings.TrimPrefix(key, projectKey)
	}
	if d, ok := curatedDetails[canonical]; ok {
		out := backend.IssueDetail{Key: key, Description: d.Description, Fields: map[string]any{}}
		for _, l := range d.Links {
			l.Key = rekey(l.Key, projectKey)
			out.Links = append(out.Links, l)
		}
		return out, true
	}
	for _, iss := range Issues(projectKey) {
		if iss.Key == key {
			return backend.IssueDetail{
				Key:         key,
				Description: "Generated demo issue. " + iss.Summary + ".",
				Links:       []backend.Link{},
				Fields:      map[string]any{},
			}, true
		}
	}
	return backend.IssueDetail{}, false
}

// rekey swaps the PLAT prefix for projectKey. Keys with another prefix (the
// XT test keys) are returned unchanged.
func rekey(key, projectKey string) string {
	if strings.HasPrefix(key, ProjectKey+"-") {
		return projectKey + strings.TrimPrefix(key, ProjectKey)
	}
	return key
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return append([]string{}, s...)
}
