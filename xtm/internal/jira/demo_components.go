package jira

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
)

// demoComponentStore holds demo-mode components for the life of the process.
// It is package-level because app.go builds a new Client for every call, so
// state on the Client would be lost between a create and the next list. It
// never touches a user database.
var demoComponentStore = struct {
	sync.Mutex
	seeded bool
	nextID int
	byID   map[string]Component
}{}

func demoComponentsLocked() map[string]Component {
	s := &demoComponentStore
	if !s.seeded {
		s.byID = map[string]Component{}
		for _, n := range demoComponentList {
			s.nextID++
			id := strconv.Itoa(10000 + s.nextID)
			s.byID[id] = Component{ID: id, Name: n, AssigneeType: "PROJECT_DEFAULT"}
		}
		s.seeded = true
	}
	return s.byID
}

// resetDemoComponents restores the seeded list. Tests call it.
func resetDemoComponents() {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	demoComponentStore.seeded = false
	demoComponentStore.nextID = 0
	demoComponentStore.byID = nil
}

func demoComponentDetails() []Component {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	out := make([]Component, 0, len(demoComponentsLocked()))
	for _, c := range demoComponentsLocked() {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func demoNameTaken(byID map[string]Component, name, exceptID string) bool {
	for id, c := range byID {
		if id != exceptID && strings.EqualFold(c.Name, name) {
			return true
		}
	}
	return false
}

func demoCreateComponent(in ComponentInput) (Component, error) {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	byID := demoComponentsLocked()
	if demoNameTaken(byID, in.Name, "") {
		return Component{}, &HTTPError{Method: http.MethodPost, Code: 400, Status: "400 Bad Request",
			Message: fmt.Sprintf("name: A component with the name %s already exists in this project.", in.Name)}
	}
	demoComponentStore.nextID++
	id := strconv.Itoa(10000 + demoComponentStore.nextID)
	c := Component{ID: id, Name: in.Name, Description: in.Description, LeadName: in.LeadUserName,
		LeadDisplayName: in.LeadUserName, AssigneeType: in.AssigneeType}
	byID[id] = c
	return c, nil
}

func demoUpdateComponent(id string, in ComponentInput) (Component, error) {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	byID := demoComponentsLocked()
	if _, ok := byID[id]; !ok {
		return Component{}, &HTTPError{Method: http.MethodPut, Code: 404, Status: "404 Not Found",
			Message: "The component with id " + id + " does not exist."}
	}
	if demoNameTaken(byID, in.Name, id) {
		return Component{}, &HTTPError{Method: http.MethodPut, Code: 400, Status: "400 Bad Request",
			Message: fmt.Sprintf("name: A component with the name %s already exists in this project.", in.Name)}
	}
	c := Component{ID: id, Name: in.Name, Description: in.Description, LeadName: in.LeadUserName,
		LeadDisplayName: in.LeadUserName, AssigneeType: in.AssigneeType}
	byID[id] = c
	return c, nil
}

func demoDeleteComponent(id string) error {
	demoComponentStore.Lock()
	defer demoComponentStore.Unlock()
	byID := demoComponentsLocked()
	if _, ok := byID[id]; !ok {
		return &HTTPError{Method: http.MethodDelete, Code: 404, Status: "404 Not Found",
			Message: "The component with id " + id + " does not exist."}
	}
	delete(byID, id)
	return nil
}

// demoUsers backs the lead picker in demo mode.
var demoUsers = []User{
	{Name: "alice", DisplayName: "Alice Andersen"},
	{Name: "bob", DisplayName: "Bob Brown"},
	{Name: "carol", DisplayName: "Carol Chen"},
}

func demoSearchUsers(q string) []User {
	q = strings.ToLower(q)
	out := []User{}
	for _, u := range demoUsers {
		if strings.Contains(strings.ToLower(u.Name), q) || strings.Contains(strings.ToLower(u.DisplayName), q) {
			out = append(out, u)
		}
	}
	return out
}
