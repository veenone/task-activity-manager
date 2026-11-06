package query

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	corejira "agile-suite/core/jira"
)

// Starred is the saved filters the account has starred. Data Center lists no
// others, so a filter nobody starred is reached through Filter by its id.
func Starred(ctx context.Context, client *corejira.Client) ([]corejira.RawFilter, error) {
	out, err := client.FavouriteFilters(ctx)
	if err != nil {
		return nil, readable(err)
	}
	return out, nil
}

// Filter reads one saved filter by the id in its Jira address.
func Filter(ctx context.Context, client *corejira.Client, id string) (corejira.RawFilter, error) {
	id = strings.TrimSpace(id)
	if _, err := strconv.ParseUint(id, 10, 64); err != nil {
		return corejira.RawFilter{}, errors.New("A filter id is a number, as in the filter's Jira address.")
	}
	f, err := client.Filter(ctx, id)
	var he *corejira.HTTPError
	if errors.As(err, &he) && (he.Code == http.StatusNotFound || he.Code == http.StatusBadRequest) {
		return corejira.RawFilter{}, fmt.Errorf("Jira has no filter %s that this account can see.", id)
	}
	if err != nil {
		return corejira.RawFilter{}, readable(err)
	}
	return f, nil
}

// Projects is every project the account can see.
func Projects(ctx context.Context, client *corejira.Client) ([]corejira.Project, error) {
	out, err := client.Projects(ctx)
	if err != nil {
		return nil, readable(err)
	}
	return out, nil
}
