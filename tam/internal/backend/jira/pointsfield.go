package jira

import (
	"errors"
	"fmt"
	"strings"
)

// pointsField is the field this project estimates story points in: the one
// discovered by name, unless the instance has more than one field of that
// name, in which case there is no answer. Returning an error rather than a
// guess is the point: writing an estimate to a field nobody chose is silent,
// and dropping the number the user typed is worse.
//
// This does not say the field is on the issue's edit or create screen. It
// cannot: only Jira's own editmeta and createmeta know that, and TAM does not
// read editmeta yet. A refusal from Jira is still the backstop, which is why
// it is worth reading as a sentence.
func (b *Backend) pointsField(projectKey string, ids fieldIDs) (string, error) {
	if ids.Points != "" {
		return ids.Points, nil
	}
	if dup := ids.ambiguous[pointsFieldName]; len(dup) > 0 {
		return "", fmt.Errorf(
			"TAM cannot tell which field %s estimates story points in: %s are both called %s. Ask a Jira administrator which field this project uses, or set the points in Jira.",
			projectKey, strings.Join(dup, " and "), pointsFieldName)
	}
	return "", errors.New("this Jira has no Story Points field, so points cannot be pushed")
}
