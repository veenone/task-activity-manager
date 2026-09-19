package jira

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"

	corejira "agile-suite/core/jira"
)

// screenRefusal is the phrase Jira uses when a field is not on the screen it
// is being set through. Matching it is what earns the create path its extra
// sentence, since that is the one refusal a person cannot act on without
// knowing why TAM sent the field at all.
//
// ponytail: substring match on Jira's own English. A localised instance
// simply gets the naming without the extra sentence, which is the safe way
// to be wrong; matching on a code instead would need Jira to send one.
const screenRefusal = "cannot be set"

// messageCap bounds the humanized message the same way core bounds the raw
// one: it is persisted into a commit failure row and rendered whole.
const messageCap = 1024

// humanizeFieldError rewrites a write Jira refused so it names the fields
// rather than only their ids. Jira keys its errors map by field id, which is
// all the transport can know; the instance's field list is what turns
// customfield_10253 into Team, and this backend is where both are in reach.
//
// An error naming no field is returned untouched, id and all: there is
// nothing to name and the transport detail is then the only thing to go on.
// A field the instance does not name keeps its id rather than being given an
// invented one. onCreate adds the sentence that only makes sense for a
// create screen.
func (b *Backend) humanizeFieldError(ctx context.Context, err error, onCreate bool) error {
	var we *corejira.WriteError
	if !errors.As(err, &we) || len(we.Fields) == 0 {
		return err
	}
	ids := make([]string, 0, len(we.Fields))
	for id := range we.Fields {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	parts := append([]string{}, we.Messages...)
	if len(ids) == 1 {
		parts = append(parts, fmt.Sprintf("Jira refused the field %s: %s", b.fieldLabel(ctx, ids[0]), we.Fields[ids[0]]))
	} else {
		parts = append(parts, fmt.Sprintf("Jira refused %d fields.", len(ids)))
		for _, id := range ids {
			parts = append(parts, fmt.Sprintf("%s: %s", b.fieldLabel(ctx, id), we.Fields[id]))
		}
	}
	if onCreate && anyScreenRefusal(we.Fields) {
		if len(ids) == 1 {
			parts = append(parts, "TAM sent it because Jira's create metadata for this issue type lists it; if it is not on the create screen, a Jira administrator has to put it there.")
		} else {
			parts = append(parts, "TAM sent them because Jira's create metadata for this issue type lists them; if they are not on the create screen, a Jira administrator has to put them there.")
		}
	}
	msg := strings.Join(parts, " ")
	if len(msg) > messageCap {
		msg = msg[:messageCap] + "…"
	}
	return errors.New(msg)
}

// fieldLabel is how a person sees the field: its name and its id, or the id
// alone when the instance does not list it.
func (b *Backend) fieldLabel(ctx context.Context, id string) string {
	if name := b.c.FieldName(ctx, id); name != "" {
		return fmt.Sprintf("%s (%s)", name, id)
	}
	return id
}

func anyScreenRefusal(fields map[string]string) bool {
	for _, msg := range fields {
		if strings.Contains(msg, screenRefusal) {
			return true
		}
	}
	return false
}
