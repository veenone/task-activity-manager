package jira

import (
	"context"
	"errors"
	"net/url"
)

// GetCreateScreenFields returns the fields a project's create screen for one
// issue type marks required, less the ones in skip, which the caller's form
// already collects. Exactly one of issueTypeID and issueTypeName is set: bug
// and requirement create know their type by name, precondition and test create
// by the id they resolve, since those type names vary per instance.
//
// Live path: GET /rest/api/2/issue/createmeta with the
// projects.issuetypes.fields expansion. AllowedValues comes from each field's
// allowedValues, taking value, or name where there is no value.
//
// NOTE(xtm): the createmeta fields expansion (required flag, allowedValues
// shape) follows Jira DC REST v2 conventions and must be verified against the
// live Xray Server/DC 8.4.0 instance. Some instances disable the expansion,
// and then this returns an empty slice without an error. An instance that no
// longer has the endpoint at all (Jira 9 removed it, see getCreatemeta) is
// read the same way, so the form opens without the extra fields (#313).
func (c *Client) GetCreateScreenFields(ctx context.Context, projectKey, issueTypeID, issueTypeName string, skip []string) ([]FieldDef, error) {
	if (issueTypeID == "") == (issueTypeName == "") {
		return nil, errors.New("create screen fields: give an issue type id or a name, not both")
	}
	path := "/rest/api/2/issue/createmeta?projectKeys=" + url.QueryEscape(projectKey)
	if issueTypeID != "" {
		path += "&issuetypeIds=" + url.QueryEscape(issueTypeID)
	} else {
		path += "&issuetypeNames=" + url.QueryEscape(issueTypeName)
	}
	path += "&expand=projects.issuetypes.fields"

	var meta struct {
		Projects []struct {
			IssueTypes []struct {
				Fields map[string]struct {
					Name     string `json:"name"`
					Required bool   `json:"required"`
					Schema   struct {
						Type  string `json:"type"`
						Items string `json:"items"`
					} `json:"schema"`
					AllowedValues []struct {
						ID    string `json:"id"`
						Value string `json:"value"`
						Name  string `json:"name"`
					} `json:"allowedValues"`
				} `json:"fields"`
			} `json:"issuetypes"`
		} `json:"projects"`
	}
	ok, err := c.getCreatemeta(ctx, path, &meta)
	if err != nil {
		return nil, err
	}
	if !ok {
		// No createmeta on this instance, which reads the same as an instance
		// with the expansion disabled: no extra required fields to collect.
		// The form then opens without them rather than not at all.
		return nil, nil
	}

	skipped := make(map[string]bool, len(skip))
	for _, id := range skip {
		skipped[id] = true
	}
	// A field offered on more than one issue type is answered once per type.
	// The dialog must render it once, or the second copy overwrites what the
	// user typed into the first.
	seen := make(map[string]bool)
	var out []FieldDef
	for _, proj := range meta.Projects {
		for _, it := range proj.IssueTypes {
			for id, fd := range it.Fields {
				if skipped[id] || !fd.Required || seen[id] {
					continue
				}
				seen[id] = true
				avs := make([]FieldOption, 0, len(fd.AllowedValues))
				for _, av := range fd.AllowedValues {
					v := av.Value
					if v == "" {
						v = av.Name
					}
					avs = append(avs, FieldOption{ID: av.ID, Value: v})
				}
				out = append(out, FieldDef{
					ID:            id,
					Name:          fd.Name,
					Required:      true,
					Type:          bugCreateFieldKind(fd.Schema.Type, fd.Schema.Items),
					AllowedValues: avs,
				})
			}
		}
	}
	return out, nil
}

// GetPreconditionCreateFields returns the required fields on the precondition
// create screen beyond what the New precondition dialog collects: summary,
// description, the Pre-Condition Type and the condition. Demo URLs return a
// representative set without a network call.
func (c *Client) GetPreconditionCreateFields(ctx context.Context, projectKey string) ([]FieldDef, error) {
	if isDemoURL(c.baseURL) {
		return demoPreconditionCreateFields(), nil
	}
	typeID, _, err := c.resolvePreconditionType(ctx)
	if err != nil || typeID == "" {
		return nil, err
	}
	skip := []string{"project", "issuetype", "summary", "description"}
	skip = c.appendXrayFieldIDs(ctx, skip, c.preconditionTypeFieldID, c.conditionFieldID)
	return c.GetCreateScreenFields(ctx, projectKey, typeID, "", skip)
}

// GetTestCreateFields returns the required fields on the Test create screen
// beyond what the New Test panel collects: summary, description, priority,
// labels, components, and the test type with its Cucumber and Generic body.
// Demo URLs return none.
func (c *Client) GetTestCreateFields(ctx context.Context, projectKey string) ([]FieldDef, error) {
	if isDemoURL(c.baseURL) {
		return nil, nil
	}
	typeID, _, err := c.resolveTestType(ctx)
	if err != nil || typeID == "" {
		return nil, err
	}
	skip := []string{"project", "issuetype", "summary", "description", "priority", "labels", "components"}
	skip = c.appendXrayFieldIDs(ctx, skip,
		c.testTypeFieldID, c.cucumberScenarioFieldID, c.cucumberTypeFieldID, c.genericDefinitionFieldID)
	return c.GetCreateScreenFields(ctx, projectKey, typeID, "", skip)
}

// appendXrayFieldIDs adds each resolvable Xray field id to skip. A field that
// does not resolve is left out: at worst the form asks for it twice, which is
// better than failing to ask for anything.
func (c *Client) appendXrayFieldIDs(ctx context.Context, skip []string, resolvers ...func(context.Context) (string, error)) []string {
	for _, resolve := range resolvers {
		if id, err := resolve(ctx); err == nil && id != "" {
			skip = append(skip, id)
		}
	}
	return skip
}

// mergeExtraFields adds the create screen's extra field values to a create
// payload without overriding a field the caller already set.
func mergeExtraFields(fields, extra map[string]any) {
	for k, v := range extra {
		if _, exists := fields[k]; !exists {
			fields[k] = v
		}
	}
}

// demoPreconditionCreateFields is a required custom field for demo mode, so
// the precondition create flow prompts offline as the requirement one does.
func demoPreconditionCreateFields() []FieldDef {
	return []FieldDef{{
		ID:       "customfield_14410",
		Name:     "Environment",
		Required: true,
		Type:     "option",
		AllowedValues: []FieldOption{
			{ID: "15301", Value: "Lab"},
			{ID: "15302", Value: "Staging"},
			{ID: "15303", Value: "Production"},
		},
	}}
}
