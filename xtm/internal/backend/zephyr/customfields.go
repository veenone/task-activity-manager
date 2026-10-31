// The two custom field namespaces a Zephyr profile has.
//
// Jira's own custom fields live on the test issue, and those are the ones XTM
// can edit: a custom field change commits as a Jira issue field update. Zephyr
// adds its own fields on an EXECUTION and on a TESTSTEP entity, which are a
// different namespace rather than more issue fields, and which this backend
// has no write path for at all.
//
// The scope on backend.CustomFieldDef is what carries that difference to the
// store, the commit engine and the detail panel. Prefixing ids inside one flat
// list was the alternative and was rejected, because a prefix encodes meaning
// in a string every consumer then has to parse (#371 Phase 3).

package zephyr

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strings"

	"agile-suite/xtm/internal/backend"
)

// zephyrEntities are the entity types Zephyr keeps custom fields for, paired
// with the scope each one maps to. Zephyr names more entities than these, but
// an execution and a step are the two a test's detail panel can say anything
// useful about.
var zephyrEntities = []struct {
	entityType string
	scope      string
}{
	{"EXECUTION", backend.FieldScopeExecution},
	{"TESTSTEP", backend.FieldScopeStep},
}

// ListCustomFields reads the Jira namespace and then Zephyr's own.
//
// The two halves fail differently on purpose. The Jira fields are the ones a
// user can edit, and a sync that quietly reported none would empty the detail
// panel's custom fields, so a failure there is reported. Zephyr's namespaces
// are additional information with no write path, and the custom field stage is
// a hard failure in the sync engine, so losing a whole sync over a namespace
// whose response body was never measured would be the wrong trade: a failure
// there is logged and that namespace is left out.
func (a *Adapter) ListCustomFields(ctx context.Context, projectKey string) ([]backend.CustomFieldDef, error) {
	defs, err := a.jiraSide().ListCustomFields(ctx, projectKey)
	if err != nil {
		return nil, err
	}
	// The store keys definitions by (profile, field id), so two definitions
	// sharing an id would overwrite each other there and the surviving scope
	// would depend on the order they were read in. Zephyr's own ids come from
	// one counter, so a repeat is the same field seen twice.
	seen := make(map[string]bool, len(defs))
	for _, d := range defs {
		seen[d.ID] = true
	}
	for _, e := range zephyrEntities {
		own, err := a.z.entityCustomFields(ctx, e.entityType)
		if err != nil {
			log.Printf("xtm: zephyr: custom fields on a %s: %v (listing the profile without them)", e.entityType, err)
			continue
		}
		for _, f := range own {
			id := jsonString(f.ID)
			// A definition with no id cannot be stored and one with no name
			// renders as an empty label beside its value, which reads as a
			// broken panel rather than as a field nobody named.
			if id == "" || strings.TrimSpace(f.Name) == "" || seen[id] {
				continue
			}
			seen[id] = true
			defs = append(defs, backend.CustomFieldDef{
				ID:    id,
				Name:  strings.TrimSpace(f.Name),
				Type:  f.typeName(),
				Scope: e.scope,
			})
		}
	}
	return defs, nil
}

// zephyrField is one definition out of a Zephyr namespace.
//
// The entity endpoint was measured answering 200 on the probed instance, but
// its body was not captured, so the two keys SmartBear's API uses for a field
// type are both read and the id is taken through jsonString, which accepts it
// quoted or bare the way the step and execution ids arrive.
type zephyrField struct {
	ID              json.RawMessage `json:"id"`
	Name            string          `json:"name"`
	CustomFieldType string          `json:"customFieldType"`
	Type            string          `json:"type"`
}

// typeName is the field's type, whichever key carried it. An empty type is
// left empty rather than guessed: the detail panel edits every type as text.
func (f zephyrField) typeName() string {
	if f.CustomFieldType != "" {
		return f.CustomFieldType
	}
	return f.Type
}

// entityCustomFields reads one Zephyr namespace.
//
// Both documented shapes are accepted, a bare array and an array wrapped in an
// object, because this response was never captured and Zephyr wraps some of
// its collections and not others: the steps array arrives inside
// stepBeanCollection while a step result list arrives bare.
func (c *Client) entityCustomFields(ctx context.Context, entityType string) ([]zephyrField, error) {
	path := "/customfield/entity?entityType=" + url.QueryEscape(entityType)
	var raw json.RawMessage
	if err := c.getJSON(ctx, path, &raw); err != nil {
		return nil, err
	}
	var bare []zephyrField
	if err := json.Unmarshal(raw, &bare); err == nil {
		return bare, nil
	}
	var wrapped struct {
		CustomFields []zephyrField `json:"customFields"`
	}
	if err := json.Unmarshal(raw, &wrapped); err != nil {
		return nil, fmt.Errorf("zephyr: decode the custom fields of a %s: %w", entityType, err)
	}
	return wrapped.CustomFields, nil
}
