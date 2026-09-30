package boardrepo_test

import (
	"context"
	"slices"
	"testing"

	"agile-suite/tam/internal/boardrepo"
)

// tick is the write the panel's checkbox makes, failing the test rather than
// the assertion below it when the store refuses.
func tick(t *testing.T, r *boardrepo.Repository, key, item string, on bool) {
	t.Helper()
	if err := r.SetDoneAgreementTick(context.Background(), "p1", 1, key, item, on); err != nil {
		t.Fatalf("tick %q on %s: %v", item, key, err)
	}
}

func ticksOf(t *testing.T, r *boardrepo.Repository, keys ...string) map[string][]string {
	t.Helper()
	got, err := r.DoneAgreementTicks(context.Background(), "p1", 1, keys)
	if err != nil {
		t.Fatalf("read the ticks of %q: %v", keys, err)
	}
	return got
}

// A tick is one issue's own judgement about one item, and it survives the
// process: it is a row, not state in the panel.
func TestATickBelongsToOneIssueAndCarriesTheWordingItWasMadeAgainst(t *testing.T) {
	r, _ := newRepo(t)
	tick(t, r, "PLAT-1", "Docs updated", true)
	tick(t, r, "PLAT-1", "Reviewed by someone else", true)
	tick(t, r, "PLAT-2", "Unit tests pass", true)

	got := ticksOf(t, r, "PLAT-1", "PLAT-2", "PLAT-3")
	if want := []string{"Docs updated", "Reviewed by someone else"}; !slices.Equal(got["PLAT-1"], want) {
		t.Errorf("PLAT-1 ticks = %q, want %q", got["PLAT-1"], want)
	}
	if want := []string{"Unit tests pass"}; !slices.Equal(got["PLAT-2"], want) {
		t.Errorf("PLAT-2 ticks = %q, want only its own", got["PLAT-2"])
	}
	if ticks, ok := got["PLAT-3"]; ok {
		t.Errorf("PLAT-3 ticks = %q, want an issue nobody has ticked left out", ticks)
	}
}

func TestUntickingForgetsTheTick(t *testing.T) {
	r, _ := newRepo(t)
	tick(t, r, "PLAT-1", "Unit tests pass", true)
	tick(t, r, "PLAT-1", "Unit tests pass", false)
	if got := ticksOf(t, r, "PLAT-1")["PLAT-1"]; len(got) != 0 {
		t.Errorf("ticks = %q, want none left after unticking", got)
	}
	// Unticking what is not ticked is the state the reader asked for, not an
	// error: a second click on a box the last one already cleared.
	if err := r.SetDoneAgreementTick(context.Background(), "p1", 1, "PLAT-1", "Unit tests pass", false); err != nil {
		t.Errorf("untick an unticked item: %v", err)
	}
	// Ticking twice is one tick, because the wording is the key.
	tick(t, r, "PLAT-1", "Unit tests pass", true)
	tick(t, r, "PLAT-1", "Unit tests pass", true)
	if got := ticksOf(t, r, "PLAT-1")["PLAT-1"]; len(got) != 1 {
		t.Errorf("ticks = %q, want the same item ticked twice to be one tick", got)
	}
}

// The record is the point of the feature: a tick is evidence a team can be
// shown at a sprint review, so nothing here rewrites or drops a row because
// the agreement has since been reworded. The store answers with the wording
// each tick was made against and leaves the comparison to its caller, which
// is what lets the panel show an old tick as made against different words
// instead of quietly moving it or losing it.
func TestAWordingChangeLeavesTheTickAsItWasMade(t *testing.T) {
	r, _ := newRepo(t)
	tick(t, r, "PLAT-1", "Unit tests pass", true)

	// The team rewords the item. Nothing tells the store, because nothing has
	// to: the row still says what was ticked.
	if got := ticksOf(t, r, "PLAT-1")["PLAT-1"]; !slices.Equal(got, []string{"Unit tests pass"}) {
		t.Errorf("ticks = %q, want the wording the tick was made against", got)
	}
	// A tick against the new wording is a second tick, not a moved one: both
	// are true statements about what somebody ticked and when.
	tick(t, r, "PLAT-1", "Unit tests pass on CI", true)
	want := []string{"Unit tests pass", "Unit tests pass on CI"}
	if got := ticksOf(t, r, "PLAT-1")["PLAT-1"]; !slices.Equal(got, want) {
		t.Errorf("ticks = %q, want %q", got, want)
	}
}

// The read a sprint report's section makes: a sprint's issues in one call,
// which is the same call the panel makes for one issue.
func TestOneReadAnswersForEveryIssueNamed(t *testing.T) {
	r, _ := newRepo(t)
	tick(t, r, "PLAT-1", "Unit tests pass", true)
	tick(t, r, "PLAT-2", "Docs updated", true)
	got := ticksOf(t, r, "PLAT-1", "PLAT-2")
	if len(got) != 2 || !slices.Equal(got["PLAT-1"], []string{"Unit tests pass"}) || !slices.Equal(got["PLAT-2"], []string{"Docs updated"}) {
		t.Errorf("ticks = %v, want each issue's own", got)
	}
	// Nothing asked about is nothing answered, rather than the whole board.
	if got := ticksOf(t, r); len(got) != 0 {
		t.Errorf("ticks = %v, want none for a read that named no issue", got)
	}
}

// An item's text and an issue's key cross the Wails boundary, so they are
// input (I1). A blank one has no identity for a row to be keyed by.
func TestTheStoreRefusesWhatCannotIdentifyATick(t *testing.T) {
	r, _ := newRepo(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name      string
		profileID string
		boardID   int
		issueKey  string
		item      string
	}{
		{name: "no profile", profileID: "", boardID: 1, issueKey: "PLAT-1", item: "Unit tests pass"},
		{name: "no board", profileID: "p1", boardID: 0, issueKey: "PLAT-1", item: "Unit tests pass"},
		{name: "no issue", profileID: "p1", boardID: 1, issueKey: " ", item: "Unit tests pass"},
		{name: "no item", profileID: "p1", boardID: 1, issueKey: "PLAT-1", item: "  "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := r.SetDoneAgreementTick(ctx, tc.profileID, tc.boardID, tc.issueKey, tc.item, true); err == nil {
				t.Error("the tick was accepted, want it refused")
			}
		})
	}
	// A read with no profile or no board is refused for the same reason: it
	// would answer with rows belonging to whoever else matched.
	if _, err := r.DoneAgreementTicks(ctx, "", 1, []string{"PLAT-1"}); err == nil {
		t.Error("a read with no profile was accepted, want it refused")
	}
	if _, err := r.DoneAgreementTicks(ctx, "p1", 0, []string{"PLAT-1"}); err == nil {
		t.Error("a read with no board was accepted, want it refused")
	}
	keys := make([]string, boardrepo.MaxTickIssues+1)
	for i := range keys {
		keys[i] = "PLAT-1"
	}
	if _, err := r.DoneAgreementTicks(ctx, "p1", 1, keys); err == nil {
		t.Errorf("a read of %d issues was accepted, want it refused", len(keys))
	}
}

// A purge of the profile takes the ticks with it, and so does removing the
// board: a tick is a judgement against this board's agreement, and once the
// board is gone there is no agreement left for it to be about.
func TestPurgingAndRemovingSweepTheTicks(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name string
		drop func(r *boardrepo.Repository) error
	}{
		{"purge the profile", func(r *boardrepo.Repository) error { return r.PurgeProfile(ctx, "p1") }},
		{"remove the board", func(r *boardrepo.Repository) error { return r.RemoveBoards(ctx, "p1", []int{1}) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, db := newRepo(t)
			tick(t, r, "PLAT-1", "Unit tests pass", true)
			// The other half of a deletion: the same profile's second board
			// and another profile's board keep their own ticks. A sweep with
			// no WHERE behind it passes the assertion above on its own.
			for _, kept := range [][2]any{{"p1", 2}, {"p2", 1}} {
				if err := r.SetDoneAgreementTick(ctx, kept[0].(string), kept[1].(int), "PLAT-9", "Unit tests pass", true); err != nil {
					t.Fatalf("tick to keep: %v", err)
				}
			}
			if err := tc.drop(r); err != nil {
				t.Fatalf("%s: %v", tc.name, err)
			}
			count := func(where string) int {
				t.Helper()
				var n int
				if err := db.QueryRow(`SELECT count(*) FROM done_agreement_tick WHERE ` + where).Scan(&n); err != nil {
					t.Fatal(err)
				}
				return n
			}
			if left := count(`profile_id = 'p1' AND board_id = 1`); left != 0 {
				t.Errorf("%d ticks left after %s, want none", left, tc.name)
			}
			wantKept := 1
			if tc.name == "purge the profile" {
				wantKept = 0
			}
			if mine := count(`profile_id = 'p1' AND board_id = 2`); mine != wantKept {
				t.Errorf("board 2 kept %d ticks after %s, want %d", mine, tc.name, wantKept)
			}
			if other := count(`profile_id = 'p2'`); other != 1 {
				t.Errorf("p2 kept %d ticks after %s, want its own tick untouched", other, tc.name)
			}
		})
	}
}
