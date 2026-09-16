package committer

import (
	"fmt"

	"agile-suite/tam/internal/issuerepo"
)

// Held is a pending row this Commit did not send because something it names
// was not created in Jira: a draft issue or a draft sprint whose create was
// refused, or which is itself waiting for one. It stays in the journal and
// the next Commit tries again. It is not a failure: nothing was sent, so
// there is nothing Jira refused about this row itself, and Reason says what
// it waits for.
type Held struct {
	Key        string `json:"key"`
	EntityType string `json:"entityType"`
	RowID      int64  `json:"rowId"`
	WaitsFor   string `json:"waitsFor"`
	Reason     string `json:"reason"`
}

// dependencies is what one Commit knows about placeholders it could not make
// real: a TAM-NEW key or a draft sprint id, with the label a sentence names
// it by and why it is still a placeholder. Every phase asks it before it
// sends a row naming one.
type dependencies struct {
	blocked map[string]blocker
}

type blocker struct {
	label string
	why   string
}

func newDependencies() *dependencies {
	return &dependencies{blocked: map[string]blocker{}}
}

// block records a placeholder this Commit will not make real.
func (d *dependencies) block(placeholder, label, why string) {
	d.blocked[placeholder] = blocker{label: label, why: why}
}

// blockedBy answers the first of values that names a blocked placeholder:
// an issue key as it stands, or a sprint id either bare or as the id half of
// a journaled move value.
func (d *dependencies) blockedBy(values ...string) (string, bool) {
	for _, v := range values {
		if v == "" {
			continue
		}
		if _, ok := d.blocked[v]; ok {
			return v, true
		}
		if id := issuerepo.MoveID(v); id != v {
			if _, ok := d.blocked[id]; ok {
				return id, true
			}
		}
	}
	return "", false
}

// reason is the sentence a held row carries: "waits for TAM-NEW-2, which
// Jira refused".
func (d *dependencies) reason(placeholder string) string {
	b := d.blocked[placeholder]
	return fmt.Sprintf("waits for %s, %s", b.label, b.why)
}

// hold reports a row held back for waits, and blocks the row's own key when
// that key is a draft not already blocked, so whatever waits for it is held
// in turn. A link drafted from a refused epic is a row under the epic's own
// key; it must not overwrite why the epic itself is blocked.
func (d *dependencies) hold(res *Result, key, entityType string, rowID int64, waits string) {
	res.Held = append(res.Held, Held{Key: key, EntityType: entityType, RowID: rowID, WaitsFor: waits, Reason: d.reason(waits)})
	if _, already := d.blocked[key]; isDraftKey(key) && !already {
		d.block(key, key, "which is waiting for "+d.blocked[waits].label)
	}
}
