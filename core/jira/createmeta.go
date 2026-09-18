package jira

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
)

// Create metadata is the list of fields an issue type's create screen
// carries. Data Center 8.4 answers it per issue type, paged, and a field
// absent from that answer is not on the screen. Older instances only have
// the classic expand call, which on some versions lists fields that are not
// on the screen at all, and sending one of those gets "Field cannot be set.
// It is not on the appropriate screen". So the per-type endpoint is asked
// first and the classic call only when the per-type one answers 404.

// The two endpoints a CreateMeta can have come from.
const (
	MetaPerType = "issuetype"
	MetaClassic = "classic"
)

// The kinds a field is rendered and shaped by. They are also the FieldSpec
// types TAM's New issue dialog draws inputs for.
const (
	KindString   = "string"
	KindTextarea = "textarea"
	KindOption   = "option"
	KindNumber   = "number"
	KindDate     = "date"
	KindDateTime = "datetime"
	KindArray    = "array"
	KindUser     = "user"
	// KindOther is a field no text form can fill: an attachment, issue
	// links, time tracking.
	KindOther = "other"
)

// metaPage is how many fields one per-type page asks for, and metaMaxPages
// bounds a server that ignores startAt and answers the same page forever.
const (
	metaPage     = 50
	metaMaxPages = 40
)

// MetaSchema is a field's schema as createmeta reports it.
type MetaSchema struct {
	Type   string `json:"type"`
	Items  string `json:"items"`
	System string `json:"system"`
	Custom string `json:"custom"`
}

// MetaOption is one allowed value of a field.
type MetaOption struct {
	ID    string `json:"id"`
	Value string `json:"value"`
	Name  string `json:"name"`
}

// Label is what a person reads for the option: its value, or its name when
// Jira sent no value (components and versions carry names).
func (o MetaOption) Label() string {
	if o.Value != "" {
		return o.Value
	}
	return o.Name
}

// MetaField is one field of a create or edit screen. ID is fieldId on the
// per-type answer and the map key on the classic and editmeta ones.
//
// Operations is what a write may do to the field: a field an edit screen
// lists with add and remove but not set refuses the single value a TAM edit
// sends, and the edit-screen reader drops it for that reason. An absent
// array is not an empty one; some Data Center payloads omit it, and that
// says nothing rather than no.
//
// HasDefaultValue is parsed but not read yet: the createmeta probe
// (docs/superpowers/plans/assets/2026-09-15-createmeta-probe.md) decides
// whether the create screen check uses it.
type MetaField struct {
	ID              string       `json:"fieldId"`
	Name            string       `json:"name"`
	Required        bool         `json:"required"`
	HasDefaultValue bool         `json:"hasDefaultValue"`
	Operations      []string     `json:"operations"`
	Schema          MetaSchema   `json:"schema"`
	AllowedValues   []MetaOption `json:"allowedValues"`
}

// MetaFields is a screen's fields, whichever endpoint reported them. Both
// create metadata and editmeta answer with the same MetaField, so the lookup
// over them is written once.
type MetaFields []MetaField

// Field finds a field by id.
func (m MetaFields) Field(id string) (MetaField, bool) {
	for _, f := range m {
		if f.ID == id {
			return f, true
		}
	}
	return MetaField{}, false
}

// CreateMeta is one issue type's create fields and the endpoint that
// answered, since only the per-type answer can be read as the screen.
type CreateMeta struct {
	Source string
	Fields MetaFields
}

// Field finds a field by id.
func (m CreateMeta) Field(id string) (MetaField, bool) { return m.Fields.Field(id) }

// Kind is how the field is rendered and shaped.
func (f MetaField) Kind() string {
	switch f.Schema.Type {
	case "string":
		if strings.HasSuffix(f.Schema.Custom, ":textarea") || f.Schema.System == "description" || f.Schema.System == "environment" {
			return KindTextarea
		}
		return KindString
	case "number":
		return KindNumber
	case "date":
		return KindDate
	case "datetime":
		return KindDateTime
	case "user":
		return KindUser
	case "array":
		switch f.Schema.Items {
		case "attachment", "issuelinks", "worklog":
			return KindOther
		}
		return KindArray
	case "option", "option-with-child", "priority", "version", "component", "resolution", "securitylevel":
		return KindOption
	case "attachment", "issuelinks", "timetracking", "worklog", "comments-page", "any":
		return KindOther
	}
	if len(f.AllowedValues) > 0 {
		return KindOption
	}
	return KindString
}

// CreateMeta reads one issue type's create fields. issueTypeID picks the
// per-type endpoint; an empty id, or a 404 from that endpoint, falls back to
// the classic expand call, by id when there is one and by issueTypeName
// otherwise. Any other failure is returned as it is: a 403 on the per-type
// endpoint will not read better through the classic one.
func (c *Client) CreateMeta(ctx context.Context, projectKey, issueTypeID, issueTypeName string) (CreateMeta, error) {
	if strings.TrimSpace(issueTypeID) != "" {
		fields, err := c.perTypeMeta(ctx, projectKey, issueTypeID)
		if err == nil {
			return CreateMeta{Source: MetaPerType, Fields: fields}, nil
		}
		var he *HTTPError
		if !errors.As(err, &he) || he.Code != http.StatusNotFound {
			return CreateMeta{}, err
		}
	}
	fields, err := c.classicMeta(ctx, projectKey, issueTypeID, issueTypeName)
	if err != nil {
		return CreateMeta{}, err
	}
	return CreateMeta{Source: MetaClassic, Fields: fields}, nil
}

func (c *Client) perTypeMeta(ctx context.Context, projectKey, issueTypeID string) ([]MetaField, error) {
	out := []MetaField{}
	start := 0
	for page := 0; page < metaMaxPages; page++ {
		q := url.Values{}
		q.Set("startAt", strconv.Itoa(start))
		q.Set("maxResults", strconv.Itoa(metaPage))
		var body struct {
			Total  int         `json:"total"`
			IsLast bool        `json:"isLast"`
			Values []MetaField `json:"values"`
		}
		path := "/rest/api/2/issue/createmeta/" + url.PathEscape(projectKey) + "/issuetypes/" + url.PathEscape(issueTypeID) + "?" + q.Encode()
		if err := c.Get(ctx, path, &body); err != nil {
			return nil, err
		}
		out = append(out, body.Values...)
		start += len(body.Values)
		switch {
		case body.IsLast, len(body.Values) == 0:
			return out, nil
		case body.Total > 0 && start >= body.Total:
			return out, nil
		case body.Total == 0 && len(body.Values) < metaPage:
			return out, nil
		}
	}
	return out, nil
}

func (c *Client) classicMeta(ctx context.Context, projectKey, issueTypeID, issueTypeName string) ([]MetaField, error) {
	q := url.Values{}
	q.Set("projectKeys", projectKey)
	if strings.TrimSpace(issueTypeID) != "" {
		q.Set("issuetypeIds", issueTypeID)
	} else {
		q.Set("issuetypeNames", issueTypeName)
	}
	q.Set("expand", "projects.issuetypes.fields")
	var body struct {
		Projects []struct {
			IssueTypes []struct {
				ID     string               `json:"id"`
				Name   string               `json:"name"`
				Fields map[string]MetaField `json:"fields"`
			} `json:"issuetypes"`
		} `json:"projects"`
	}
	if err := c.Get(ctx, "/rest/api/2/issue/createmeta?"+q.Encode(), &body); err != nil {
		return nil, err
	}
	out := []MetaField{}
	for _, p := range body.Projects {
		for _, t := range p.IssueTypes {
			matches := (issueTypeID != "" && t.ID == issueTypeID) || strings.EqualFold(t.Name, issueTypeName)
			if !matches && len(p.IssueTypes) > 1 {
				continue
			}
			for id, f := range t.Fields {
				f.ID = id
				out = append(out, f)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}

// ShapeValue turns the text a form holds into the JSON Jira wants for f: an
// option's id when Jira listed allowed values and the typed text as a value
// when it did not, every part of a comma list for an array, a name for a
// user, a number for a number, and the ISO day for a date. Text that does not
// parse as what the field wants goes as typed, so Jira's own message is the
// one the user reads.
func ShapeValue(f MetaField, v string) any {
	switch f.Kind() {
	case KindOption:
		if len(f.AllowedValues) > 0 {
			return map[string]string{"id": v}
		}
		return map[string]string{"value": v}
	case KindArray:
		parts := SplitList(v)
		switch {
		case f.Schema.Items == "user":
			return keyed("name", parts)
		case len(f.AllowedValues) > 0:
			return keyed("id", parts)
		case f.Schema.Items == "option":
			return keyed("value", parts)
		}
		return parts
	case KindNumber:
		if n, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
			return n
		}
	case KindUser:
		return map[string]string{"name": strings.TrimSpace(v)}
	case KindDate:
		return strings.TrimSpace(v)
	case KindDateTime:
		day := strings.TrimSpace(v)
		if len(day) == len("2006-01-02") {
			return day + "T00:00:00.000+0000"
		}
		return day
	}
	return v
}

func keyed(key string, parts []string) []map[string]string {
	out := make([]map[string]string, 0, len(parts))
	for _, p := range parts {
		out = append(out, map[string]string{key: p})
	}
	return out
}

// SplitList turns a form's comma list into its non-empty parts.
func SplitList(v string) []string {
	out := []string{}
	for _, p := range strings.Split(v, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
