package jira

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	corejira "agile-suite/core/jira"
	"agile-suite/tam/internal/backend"
)

// A transition is not a field edit. The journal holds the status a card was
// dropped on, never a transition id, which would be stale the moment the
// issue moved; this file is where that target becomes one of the
// transitions Jira offers for that issue at that moment. Reading what a
// write demands before making it is the same shape CreateFields uses for
// the New issue form.

// resolutionField is the Jira field id of the resolution screen almost
// every Data Center workflow puts on the way into Done. It is the one
// required field TAM fills in by itself.
const resolutionField = "resolution"

// SetTransitionResolution records the profile's preferred resolution name,
// the `transition_resolution` setting. A transition that asks only for a
// resolution takes it when the transition allows it, and the transition's
// first allowed value otherwise. It is set once, when the backend is built.
func (b *Backend) SetTransitionResolution(name string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.transitionResolution = strings.TrimSpace(name)
}

func (b *Backend) resolution() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.transitionResolution
}

// Transition moves key into targetStatusID: it lists the issue's
// transitions with their fields, picks the one whose To.ID is the target,
// and fires it. When two transitions reach the same status the one with the
// lowest id wins, so the same board drop resolves the same way on every
// run; picking arbitrarily is fine, picking differently each time is not.
//
// It returns a *backend.NoTransition, which errors.Is finds
// backend.ErrNoTransition through, when nothing reaches the target, and an
// error wrapping backend.ErrTransitionFields when the transition asks for a
// field beyond a resolution. Both name the issue and the target.
func (b *Backend) Transition(ctx context.Context, key, targetStatusID string) error {
	list, err := b.c.Transitions(ctx, key)
	if err != nil {
		return fmt.Errorf("transitions of %s: %w", key, err)
	}
	tr, ok := pickTransition(list, targetStatusID)
	if !ok {
		return &backend.NoTransition{Key: key, TargetStatusID: targetStatusID, Reachable: reachableNames(list)}
	}
	fields, err := transitionFields(key, targetStatusID, tr, b.resolution())
	if err != nil {
		return err
	}
	return b.c.DoTransition(ctx, key, tr.ID, fields)
}

// CanTransition reports whether targetStatusID is reachable from where the
// issue is now and names every status that is. It only reads, and it makes
// no judgement about the fields a transition would demand: a card whose way
// into Done is guarded by a required custom field can still be dropped
// there, and Commit is where that answer belongs.
func (b *Backend) CanTransition(ctx context.Context, key, targetStatusID string) (backend.TransitionCheck, error) {
	list, err := b.c.Transitions(ctx, key)
	if err != nil {
		return backend.TransitionCheck{}, fmt.Errorf("transitions of %s: %w", key, err)
	}
	_, ok := pickTransition(list, targetStatusID)
	return backend.TransitionCheck{Reachable: reachableNames(list), Allowed: ok}, nil
}

// pickTransition is the transition that reaches targetStatusID, the lowest
// id when several do.
func pickTransition(list []corejira.RawTransition, targetStatusID string) (corejira.RawTransition, bool) {
	var (
		best  corejira.RawTransition
		found bool
	)
	for _, tr := range list {
		if tr.To.ID != targetStatusID {
			continue
		}
		if !found || lowerID(tr.ID, best.ID) {
			best, found = tr, true
		}
	}
	return best, found
}

// lowerID compares two transition ids as numbers, falling back to text for
// an instance that hands out ids a number cannot hold.
func lowerID(a, b string) bool {
	na, erra := strconv.Atoi(a)
	nb, errb := strconv.Atoi(b)
	if erra == nil && errb == nil {
		return na < nb
	}
	return a < b
}

// reachableNames is what the issue can move to right now, in Jira's own
// order, deduplicated, each named rather than numbered so a failure reads
// as "it can move to In Progress, Done" and not "to 3, 5".
func reachableNames(list []corejira.RawTransition) []string {
	out := []string{}
	seen := map[string]bool{}
	for _, tr := range list {
		name := strings.TrimSpace(tr.To.Name)
		if name == "" {
			name = tr.To.ID
		}
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	return out
}

// transitionFields is what the transition's own screen demands, filled in
// as far as TAM honestly can. Nothing required means no fields key at all,
// since some instances reject a request that carries an empty one. A
// resolution alone is filled from the profile's setting when the transition
// allows that value and from the first allowed value otherwise. Anything
// else comes back as an error naming the fields: guessing a value for
// someone else's custom field is worse than saying the move has to be made
// in Jira.
func transitionFields(key, targetStatusID string, tr corejira.RawTransition, preferred string) (map[string]any, error) {
	var required []string
	for id, f := range tr.Fields {
		if f.Required {
			required = append(required, id)
		}
	}
	sort.Strings(required)
	switch {
	case len(required) == 0:
		return nil, nil
	case len(required) == 1 && required[0] == resolutionField:
		value, ok := resolutionValue(tr.Fields[resolutionField], preferred)
		if !ok {
			return nil, missingFields(key, targetStatusID, tr, required)
		}
		return map[string]any{resolutionField: value}, nil
	default:
		return nil, missingFields(key, targetStatusID, tr, required)
	}
}

// resolutionValue is what goes into the resolution field: the id of the
// allowed value the setting names, the id of the first allowed value
// otherwise, and the setting as a name when the instance listed no allowed
// values at all. A transition that lists nothing and has no setting to fall
// back on is refused rather than guessed at.
func resolutionValue(f corejira.RawTransitionField, preferred string) (any, bool) {
	if len(f.AllowedValues) == 0 {
		if preferred == "" {
			return nil, false
		}
		return map[string]string{"name": preferred}, true
	}
	for _, av := range f.AllowedValues {
		if preferred != "" && strings.EqualFold(strings.TrimSpace(av.Name), preferred) {
			return map[string]string{"id": av.ID}, true
		}
	}
	return map[string]string{"id": f.AllowedValues[0].ID}, true
}

// missingFields is the refusal, naming the issue, the target, the
// transition, and every field by its Jira name with its id beside it: a
// message that said only "customfield_11400 is required" tells the user
// nothing they can act on.
func missingFields(key, targetStatusID string, tr corejira.RawTransition, required []string) error {
	names := make([]string, 0, len(required))
	for _, id := range required {
		if name := strings.TrimSpace(tr.Fields[id].Name); name != "" {
			names = append(names, fmt.Sprintf("%s (%s)", name, id))
			continue
		}
		names = append(names, id)
	}
	return fmt.Errorf("%s cannot move to status %s here: the %q transition asks for %s, so make this move in Jira: %w",
		key, targetStatusID, tr.Name, strings.Join(names, ", "), backend.ErrTransitionFields)
}
