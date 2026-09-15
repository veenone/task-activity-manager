package demo

import (
	"context"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"

	"agile-suite/core/confluence"
)

// Confluence is an in-memory Confluence space. A demo profile's Rituals Sync
// runs against it, and so do ritualsync's tests, which is why it carries
// FailNext and After: the two ways a test makes Confluence misbehave at a
// precise moment.
//
// stageConflict bumps the first Standup page it creates straight after the
// create, with a paragraph "a teammate" added, so a demo user who edits the
// Standup and syncs again meets a conflict. It is the demo Commit's staged
// conflict on <project>-412, for pages.
type Confluence struct {
	mu       sync.Mutex
	space    string
	pages    map[string]*fakePage
	next     int
	failures map[string]error
	after    map[string]func()
	stage    bool
	staged   bool
}

type fakePage struct {
	id, parent, title, body string
	version                 int
}

var _ confluence.Pages = (*Confluence)(nil)

// DemoRootTitle is the demo root page's title.
const DemoRootTitle = "Team rituals"

// NewConfluence answers a space holding only its root page.
func NewConfluence(space, rootID string, stageConflict bool) *Confluence {
	c := &Confluence{space: space, pages: map[string]*fakePage{}, next: 1000,
		failures: map[string]error{}, after: map[string]func(){}, stage: stageConflict}
	c.pages[rootID] = &fakePage{id: rootID, title: DemoRootTitle, body: "<p>Ritual pages for this team.</p>", version: 1}
	return c
}

// FailNext makes the next op on target fail with err.
func (c *Confluence) FailNext(op, target string, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.failures[op+"|"+target] = err
}

// After runs fn once, after the next op on target has computed its answer and
// before it returns it.
func (c *Confluence) After(op, target string, fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.after[op+"|"+target] = fn
}

func (c *Confluence) failure(op, target string) error {
	key := op + "|" + target
	err, ok := c.failures[key]
	if ok {
		delete(c.failures, key)
	}
	return err
}

func (c *Confluence) afterHook(op, target string) func() {
	key := op + "|" + target
	fn, ok := c.after[key]
	if !ok {
		return func() {}
	}
	delete(c.after, key)
	return fn
}

func (c *Confluence) stored(p *fakePage) confluence.StoredPage {
	ancestors := []string{}
	for parent := p.parent; parent != ""; {
		ancestors = append([]string{parent}, ancestors...)
		pp, ok := c.pages[parent]
		if !ok {
			break
		}
		parent = pp.parent
	}
	return confluence.StoredPage{ID: p.id, Title: p.title, Version: p.version, Body: p.body, AncestorIDs: ancestors}
}

func notFound() error {
	return &confluence.HTTPError{Code: http.StatusNotFound, Status: "404 Not Found", Message: "No content found with id"}
}

// GetPageStorage reads a page.
func (c *Confluence) GetPageStorage(_ context.Context, id string) (confluence.StoredPage, error) {
	c.mu.Lock()
	if err := c.failure("get", id); err != nil {
		c.mu.Unlock()
		return confluence.StoredPage{}, err
	}
	p, ok := c.pages[id]
	var out confluence.StoredPage
	if ok {
		out = c.stored(p)
	}
	hook := c.afterHook("get", id)
	c.mu.Unlock()
	hook()
	if !ok {
		return confluence.StoredPage{}, notFound()
	}
	return out, nil
}

// FindPageByTitle looks through the whole space, lowest id first.
func (c *Confluence) FindPageByTitle(_ context.Context, spaceKey, title string) (confluence.StoredPage, bool, error) {
	c.mu.Lock()
	if err := c.failure("find", title); err != nil {
		c.mu.Unlock()
		return confluence.StoredPage{}, false, err
	}
	if spaceKey != c.space {
		hook := c.afterHook("find", title)
		c.mu.Unlock()
		hook()
		return confluence.StoredPage{}, false, nil
	}
	ids := make([]string, 0, len(c.pages))
	for id := range c.pages {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	var out confluence.StoredPage
	var found bool
	for _, id := range ids {
		if c.pages[id].title == title {
			out = c.stored(c.pages[id])
			found = true
			break
		}
	}
	hook := c.afterHook("find", title)
	c.mu.Unlock()
	hook()
	return out, found, nil
}

func (c *Confluence) titleTaken(title string) bool {
	for _, p := range c.pages {
		if p.title == title {
			return true
		}
	}
	return false
}

// CreatePage creates a page under parentID.
func (c *Confluence) CreatePage(_ context.Context, spaceKey, parentID, title, body string) (confluence.StoredPage, error) {
	c.mu.Lock()
	if err := c.failure("create", title); err != nil {
		c.mu.Unlock()
		return confluence.StoredPage{}, err
	}
	if spaceKey != c.space {
		c.mu.Unlock()
		return confluence.StoredPage{}, notFound()
	}
	if c.titleTaken(title) {
		c.mu.Unlock()
		return confluence.StoredPage{}, &confluence.HTTPError{Code: http.StatusBadRequest, Status: "400 Bad Request",
			Message: "A page with this title already exists: A page already exists with the title " + title + " in the space with key " + spaceKey}
	}
	if _, ok := c.pages[parentID]; !ok {
		c.mu.Unlock()
		return confluence.StoredPage{}, notFound()
	}
	p := &fakePage{id: strconv.Itoa(c.next), parent: parentID, title: title, body: body, version: 1}
	c.next++
	c.pages[p.id] = p
	out := c.stored(p)
	if c.stage && !c.staged && strings.HasSuffix(title, " · Standup") {
		c.staged = true
		p.version = 2
		p.body += "<p>Added in Confluence by a teammate: the payment sandbox is down until Thursday.</p>"
	}
	hook := c.afterHook("create", title)
	c.mu.Unlock()
	hook()
	return out, nil
}

// UpdatePage replaces a page, refusing any version but current plus one.
func (c *Confluence) UpdatePage(_ context.Context, id, title, body string, version int) (confluence.StoredPage, error) {
	c.mu.Lock()
	if err := c.failure("update", id); err != nil {
		c.mu.Unlock()
		return confluence.StoredPage{}, err
	}
	p, ok := c.pages[id]
	if !ok {
		c.mu.Unlock()
		return confluence.StoredPage{}, notFound()
	}
	if version != p.version+1 {
		current := p.version
		c.mu.Unlock()
		return confluence.StoredPage{}, &confluence.HTTPError{Code: http.StatusConflict, Status: "409 Conflict",
			Message: "Version must be incremented on update. Current version is: " + strconv.Itoa(current)}
	}
	p.title, p.body, p.version = title, body, version
	out := c.stored(p)
	hook := c.afterHook("update", id)
	c.mu.Unlock()
	hook()
	return out, nil
}

// Seed adds a page as somebody writing in Confluence would, with no failure
// or staging applied. parentID "" puts it at the top of the space.
func (c *Confluence) Seed(parentID, title, body string) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	p := &fakePage{id: strconv.Itoa(c.next), parent: parentID, title: title, body: body, version: 1}
	c.next++
	c.pages[p.id] = p
	return p.id
}

// Restore puts back a page the app knew about before a restart emptied this
// space. A restored Standup counts as already staged.
func (c *Confluence) Restore(id, parentID, title, body string, version int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.pages[id] = &fakePage{id: id, parent: parentID, title: title, body: body, version: version}
	if n, err := strconv.Atoi(id); err == nil && n >= c.next {
		c.next = n + 1
	}
	if strings.HasSuffix(title, " · Standup") {
		c.staged = true
	}
}

// EditRemote changes a page the way a teammate editing it in Confluence would.
func (c *Confluence) EditRemote(id, body string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if p, ok := c.pages[id]; ok {
		p.body = body
		p.version++
	}
}

// Remove deletes a page from the space.
func (c *Confluence) Remove(id string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.pages, id)
}

// Page reads a page without any failure or hook applying.
func (c *Confluence) Page(id string) (confluence.StoredPage, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	p, ok := c.pages[id]
	if !ok {
		return confluence.StoredPage{}, false
	}
	return c.stored(p), true
}
