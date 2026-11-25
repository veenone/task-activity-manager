package demo

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"agile-suite/tam/internal/backend"
	"agile-suite/tam/internal/demo"
)

// demoLink is one link the way Jira stores it: an id, a type, and the
// outward and inward issues, so it shows from both of its ends. Keys are in
// the dataset's PLAT form.
type demoLink struct {
	id, typ, outward, inward string
}

// demoLinkTypes are the three link types the demo defines.
var demoLinkTypes = []backend.LinkType{
	{Name: "Relates", Inward: "relates to", Outward: "relates to"},
	{Name: "Blocks", Inward: "is blocked by", Outward: "blocks"},
	{Name: "Tested By", Inward: "is tested by", Outward: "tests"},
}

// seedLinks is a short Blocks chain for the Dependencies view (#612). The
// gateway keys block the promo code story, as its first comment says, and
// the story and the VAT bug both block the discount breakdown.
func seedLinks() []demoLink {
	return []demoLink{
		{id: "20001", typ: "Blocks", outward: "PLAT-409", inward: "PLAT-412"},
		{id: "20002", typ: "Blocks", outward: "PLAT-412", inward: "PLAT-398"},
		{id: "20003", typ: "Blocks", outward: "PLAT-401", inward: "PLAT-398"},
	}
}

func (b *Backend) LinkTypes(context.Context) ([]backend.LinkType, error) {
	return append([]backend.LinkType{}, demoLinkTypes...), nil
}

// canonical maps a profile key back to the dataset's PLAT form.
func (b *Backend) canonical(key string) string {
	if rest, ok := strings.CutPrefix(key, b.project+"-"); ok {
		return demo.ProjectKey + "-" + rest
	}
	return key
}

// profileKey maps a dataset key to the profile's project.
func (b *Backend) profileKey(key string) string {
	if rest, ok := strings.CutPrefix(key, demo.ProjectKey+"-"); ok {
		return b.project + "-" + rest
	}
	return key
}

// jiraTypeName is the Jira name of a logical type, as IssueTypes lists them.
func jiraTypeName(logical string) string {
	switch logical {
	case backend.TypeEpic:
		return "Epic"
	case backend.TypeStory:
		return "Story"
	case backend.TypeBug:
		return "Bug"
	case backend.TypeRequirement:
		return "Requirement"
	case backend.TypeSubtask:
		return "Sub-task"
	}
	return "Task"
}

// dependencyLinks are key's links from b.links, seen from key: outward when
// key is the link's outward issue, inward when it is the inward one. Callers
// hold b.mu.
func (b *Backend) dependencyLinks(key string) []backend.Link {
	at := b.canonical(key)
	out := []backend.Link{}
	for _, l := range b.links {
		var direction, other string
		switch at {
		case l.outward:
			direction, other = "outward", l.inward
		case l.inward:
			direction, other = "inward", l.outward
		default:
			continue
		}
		link := backend.Link{ID: l.id, Direction: direction, Type: l.typ, Key: b.profileKey(other)}
		if iss, ok := b.find(link.Key); ok {
			link.Summary, link.IssueType = iss.Summary, jiraTypeName(iss.Type)
		}
		out = append(out, link)
	}
	return out
}

// issueLinks are everything key's search row carries: the curated detail's
// links and the dependency ones. Callers hold b.mu.
func (b *Backend) issueLinks(key string) []backend.Link {
	links := []backend.Link{}
	if d, ok := demo.Detail(b.project, key); ok {
		links = append(links, d.Links...)
	}
	return append(links, b.dependencyLinks(key)...)
}

// CreateLink records the link so both of its ends show it from now on.
func (b *Backend) CreateLink(_ context.Context, fromKey string, d backend.LinkDraft) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if _, ok := b.find(fromKey); !ok {
		return fmt.Errorf("demo: no issue %s", fromKey)
	}
	if _, ok := b.find(d.ToKey); !ok {
		return fmt.Errorf("demo: no issue %s", d.ToKey)
	}
	outward, inward := fromKey, d.ToKey
	if d.Direction == "inward" {
		outward, inward = d.ToKey, fromKey
	}
	b.nextLink++
	b.links = append(b.links, demoLink{id: strconv.Itoa(b.nextLink), typ: d.Type, outward: b.canonical(outward), inward: b.canonical(inward)})
	return nil
}
