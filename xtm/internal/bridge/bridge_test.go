package bridge_test

import (
	"reflect"
	"testing"

	"agile-suite/xtm/internal/backend"
	"agile-suite/xtm/internal/backend/kiwi"
	"agile-suite/xtm/internal/backend/xray"
	"agile-suite/xtm/internal/backend/zephyr"
	"agile-suite/xtm/internal/bridge"
)

// featureKeys extracts the Feature keys from a Gap slice, in order, so tests
// can assert both membership and order without repeating Message text.
func featureKeys(gaps []bridge.Gap) []string {
	out := make([]string, len(gaps))
	for i, g := range gaps {
		out[i] = g.Feature
	}
	return out
}

// TestComputeGaps_XrayToKiwi exercises ComputeGaps against the two real
// backends' actual Capabilities() (Xray as source, Kiwi without the
// requirements plugin as target — the common publish direction, spec
// p6_3-bridge-spec.md). Both constructors are pure (Capabilities() never
// touches the underlying client), so this needs no network and no demo mode.
func TestComputeGaps_XrayToKiwi(t *testing.T) {
	source := xray.New(nil).Capabilities()
	target := kiwi.NewFromClient(nil).Capabilities()

	gaps := bridge.ComputeGaps(source, target)

	want := []string{
		"folders",
		"steps",
		// Kiwi models no dataset, so a parameterized Xray test publishes as a
		// plain one. It writes steps, so it is not blocked (#378).
		"datasets",
		"statusModel",
		"preconditions",
		"requirements",
		"issueLinkTypes",
		"bugCreation",
		"containerKind:testset",
	}
	got := featureKeys(gaps)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gap features = %v, want %v (in this exact order)", got, want)
	}

	// Kiwi's real Capabilities() already supports environments and test types
	// (Build ~= environment; is_automated ~= test type), so those must NOT be
	// reported as gaps for this pair — only the rule's presence matters, which
	// TestComputeGaps_AllRulesFire below verifies with a target that lacks them.
	for _, unexpected := range []string{"environments", "testTypes", "tags", "testRuns"} {
		for _, f := range got {
			if f == unexpected {
				t.Errorf("unexpected gap %q: Kiwi supports this capability, so it should not be flagged lossy", unexpected)
			}
		}
	}

	// Spot-check severities and that messages are non-empty.
	bySeverity := map[string]string{}
	for _, g := range gaps {
		bySeverity[g.Feature] = g.Severity
		if g.Message == "" {
			t.Errorf("gap %q has an empty message", g.Feature)
		}
	}
	wantSeverity := map[string]string{
		"folders":               bridge.SeverityLossy,
		"steps":                 bridge.SeverityLossy,
		"datasets":              bridge.SeverityLossy,
		"statusModel":           bridge.SeverityInfo,
		"preconditions":         bridge.SeverityLossy,
		"requirements":          bridge.SeverityLossy,
		"issueLinkTypes":        bridge.SeverityLossy,
		"bugCreation":           bridge.SeverityInfo,
		"containerKind:testset": bridge.SeverityLossy,
	}
	for feature, wantSev := range wantSeverity {
		if bySeverity[feature] != wantSev {
			t.Errorf("gap %q severity = %q, want %q", feature, bySeverity[feature], wantSev)
		}
	}
}

// TestComputeGaps_XrayToXray confirms identical capabilities (publishing to
// another connection on the SAME backend) produce no gaps at all.
func TestComputeGaps_XrayToXray(t *testing.T) {
	caps := xray.New(nil).Capabilities()
	gaps := bridge.ComputeGaps(caps, caps)
	if len(gaps) != 0 {
		t.Fatalf("Xray -> Xray gaps = %v, want none", gaps)
	}
}

// TestComputeGaps_AllRulesFire uses hand-built Capabilities (a source that
// supports everything, a target that supports nothing) so every rule
// ComputeGaps implements is exercised at least once, including the ones that
// don't currently fire for the real Xray/Kiwi pair (environments, test
// types, tags, test runs) because Kiwi happens to support those today.
func TestComputeGaps_AllRulesFire(t *testing.T) {
	source := backend.Capabilities{
		StepModel:                   "objects",
		SupportsStepWrites:          true,
		SupportsDatasets:            true,
		SupportsDatasetWrites:       true,
		SupportsTestTypes:           true,
		SupportsFolders:             true,
		SupportsPreconditionObjects: true,
		SupportsRequirementObjects:  true,
		SupportsIssueLinkTypes:      true,
		SupportsEnvironments:        true,
		SupportsContainers:          true,
		ContainerKinds:              []string{"testset", "testplan", "testexec"},
		SupportsTestRuns:            true,
		SupportsWorkflowTransitions: true,
		SupportsBugCreation:         true,
		SupportsTags:                true,
	}
	target := backend.Capabilities{StepModel: "inline-text"} // everything else zero-value

	gaps := bridge.ComputeGaps(source, target)
	want := []string{
		"folders", "steps", "stepWrites", "datasets", "statusModel", "preconditions",
		"requirements", "issueLinkTypes", "environments", "testTypes", "tags",
		"bugCreation", "testRuns", "containerKind:testexec", "containerKind:testplan",
		"containerKind:testset",
	}
	got := featureKeys(gaps)
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("gap features = %v, want %v", got, want)
	}
}

// A target that cannot write steps is the one gap that must stop a run rather
// than be acknowledged. The publish engine records external_ref as soon as
// CreateTest succeeds and before it writes steps, so a refusing target leaves
// every test a stub that a retry reports as already published instead of
// repairing (#378). SeverityBlocking existed from the start with no rule
// producing it, which is why BridgeWizard let this through.
func TestComputeGaps_TargetThatCannotWriteStepsBlocksTheRun(t *testing.T) {
	source := backend.Capabilities{StepModel: "objects", SupportsStepWrites: true}
	target := backend.Capabilities{StepModel: "objects"} // reads steps, refuses to write them

	gaps := bridge.ComputeGaps(source, target)

	var got *bridge.Gap
	for i := range gaps {
		if gaps[i].Feature == "stepWrites" {
			got = &gaps[i]
		}
	}
	if got == nil {
		t.Fatalf("no stepWrites gap for a target that cannot write steps: %v", featureKeys(gaps))
	}
	if got.Severity != bridge.SeverityBlocking {
		t.Errorf("stepWrites severity = %q, want %q: a lossy gap is acknowledgeable and would still run", got.Severity, bridge.SeverityBlocking)
	}
	if got.Message == "" {
		t.Error("stepWrites gap has an empty message")
	}

	// Same step model on both sides, so the flatten rule must stay quiet: the
	// two rules answer different questions and a target can fail either.
	for _, f := range featureKeys(gaps) {
		if f == "steps" {
			t.Error("the flatten rule fired for an objects-to-objects pair")
		}
	}
}

// A target that writes steps must not be blocked, or publishing to Kiwi and to
// another Xray connection would refuse.
func TestComputeGaps_TargetThatWritesStepsIsNotBlocked(t *testing.T) {
	source := backend.Capabilities{StepModel: "objects", SupportsStepWrites: true}
	target := backend.Capabilities{StepModel: "inline-text", SupportsStepWrites: true}

	for _, g := range bridge.ComputeGaps(source, target) {
		if g.Feature == "stepWrites" {
			t.Fatalf("stepWrites gap raised for a target that can write steps: %+v", g)
		}
		if g.Severity == bridge.SeverityBlocking {
			t.Fatalf("a flatten-only pair must not block: %+v", g)
		}
	}
}

// Datasets had no rule at all, so a target without them produced no gap and
// the wizard stayed silent about the largest loss in an Xray-to-Zephyr
// publish (#378). The question the rule asks is whether the target can RECEIVE
// a dataset, so it reads SupportsDatasetWrites: a target that reads datasets
// but cannot create them loses them just the same.
func TestComputeGaps_DatasetsTheTargetCannotReceive(t *testing.T) {
	source := backend.Capabilities{SupportsDatasets: true, SupportsDatasetWrites: true}

	for _, tc := range []struct {
		name   string
		target backend.Capabilities
	}{
		{"no dataset concept at all", backend.Capabilities{}},
		{"reads datasets, cannot create them", backend.Capabilities{SupportsDatasets: true}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			gaps := bridge.ComputeGaps(source, tc.target)
			var got *bridge.Gap
			for i := range gaps {
				if gaps[i].Feature == "datasets" {
					got = &gaps[i]
				}
			}
			if got == nil {
				t.Fatalf("no datasets gap: %v", featureKeys(gaps))
			}
			if got.Severity != bridge.SeverityLossy {
				t.Errorf("datasets severity = %q, want %q", got.Severity, bridge.SeverityLossy)
			}
		})
	}

	// A target that can receive a dataset is not a gap.
	for _, g := range bridge.ComputeGaps(source, source) {
		if g.Feature == "datasets" {
			t.Errorf("datasets gap raised against a target that writes datasets: %+v", g)
		}
	}
}

// The real pair #378 is about, with both backends' own Capabilities(). Both
// constructors are pure, so this needs no network. It pins the two findings
// rather than the whole list, which the hand-built tests above already fix.
func TestComputeGaps_XrayToZephyrBlocksAndReportsDatasets(t *testing.T) {
	source := xray.New(nil).Capabilities()
	target := zephyr.New(nil, nil).Capabilities()

	gaps := bridge.ComputeGaps(source, target)

	want := []string{
		"folders", "stepWrites", "datasets", "preconditions", "environments",
		"testTypes", "containerKind:testplan", "containerKind:testset",
	}
	if got := featureKeys(gaps); !reflect.DeepEqual(got, want) {
		t.Fatalf("gap features = %v, want %v (in this exact order)", got, want)
	}

	bySeverity := map[string]string{}
	for _, g := range gaps {
		bySeverity[g.Feature] = g.Severity
	}
	if bySeverity["stepWrites"] != bridge.SeverityBlocking {
		t.Errorf("stepWrites = %q, want blocking: Zephyr's CreateTestStep refuses, so a publish leaves stubs", bySeverity["stepWrites"])
	}
	if bySeverity["datasets"] != bridge.SeverityLossy {
		t.Errorf("datasets = %q, want lossy: Xray has datasets and Essential has none", bySeverity["datasets"])
	}
}

// TestComputeGaps_Deterministic runs ComputeGaps twice on equivalent inputs
// (ContainerKinds supplied in a different order each time) and checks the
// output is identical both times — the container-kind gaps must be sorted,
// not dependent on input slice order.
func TestComputeGaps_Deterministic(t *testing.T) {
	target := backend.Capabilities{ContainerKinds: []string{"c"}}
	source1 := backend.Capabilities{ContainerKinds: []string{"b", "a", "c"}}
	source2 := backend.Capabilities{ContainerKinds: []string{"a", "c", "b"}}

	gaps1 := bridge.ComputeGaps(source1, target)
	gaps2 := bridge.ComputeGaps(source2, target)
	if !reflect.DeepEqual(gaps1, gaps2) {
		t.Fatalf("gaps not deterministic across equivalent input order: %v vs %v", gaps1, gaps2)
	}
	want := []string{"containerKind:a", "containerKind:b"}
	if got := featureKeys(gaps1); !reflect.DeepEqual(got, want) {
		t.Fatalf("gap features = %v, want %v", got, want)
	}

	// Running it again with the identical input must also be stable.
	gaps1b := bridge.ComputeGaps(source1, target)
	if !reflect.DeepEqual(gaps1, gaps1b) {
		t.Fatalf("ComputeGaps not stable across repeated calls with the same input")
	}
}

func TestDefaultMapping_NameMatchAndFallback(t *testing.T) {
	source := xray.New(nil).Capabilities()
	target := kiwi.NewFromClient(nil).Capabilities()

	sourceStatuses := []string{"TODO", "PASS", "FAIL"}
	targetStatuses := []string{"fail", "pass", "idle"}

	m := bridge.DefaultMapping(source, target, sourceStatuses, targetStatuses)

	wantStatusMap := map[string]string{
		"TODO": "fail", // no case-insensitive match -> deterministic fallback: first target status
		"PASS": "pass", // case-insensitive name match
		"FAIL": "fail", // case-insensitive name match
	}
	if !reflect.DeepEqual(m.StatusMap, wantStatusMap) {
		t.Errorf("StatusMap = %v, want %v", m.StatusMap, wantStatusMap)
	}
	if m.UnmappedPolicy != bridge.UnmappedPolicyKeepInHub {
		t.Errorf("UnmappedPolicy = %q, want %q", m.UnmappedPolicy, bridge.UnmappedPolicyKeepInHub)
	}
	if m.StepMode != bridge.StepModeFlatten {
		t.Errorf("StepMode = %q, want %q (target StepModel is inline-text)", m.StepMode, bridge.StepModeFlatten)
	}
}

func TestDefaultMapping_StepModePassthroughForObjectTarget(t *testing.T) {
	source := kiwi.NewFromClient(nil).Capabilities()
	target := xray.New(nil).Capabilities()

	m := bridge.DefaultMapping(source, target, nil, nil)
	if m.StepMode != bridge.StepModePassthrough {
		t.Errorf("StepMode = %q, want %q (target StepModel is objects)", m.StepMode, bridge.StepModePassthrough)
	}
}

func TestDefaultMapping_EmptyTargetStatusesMapToBlank(t *testing.T) {
	source := xray.New(nil).Capabilities()
	target := kiwi.NewFromClient(nil).Capabilities()

	m := bridge.DefaultMapping(source, target, []string{"Open", "Closed"}, nil)
	want := map[string]string{"Open": "", "Closed": ""}
	if !reflect.DeepEqual(m.StatusMap, want) {
		t.Errorf("StatusMap = %v, want %v", m.StatusMap, want)
	}
}

func TestDefaultMapping_Deterministic(t *testing.T) {
	source := xray.New(nil).Capabilities()
	target := kiwi.NewFromClient(nil).Capabilities()
	sourceStatuses := []string{"TODO", "PASS", "FAIL"}
	targetStatuses := []string{"fail", "pass", "idle"}

	m1 := bridge.DefaultMapping(source, target, sourceStatuses, targetStatuses)
	m2 := bridge.DefaultMapping(source, target, sourceStatuses, targetStatuses)
	if !reflect.DeepEqual(m1, m2) {
		t.Fatalf("DefaultMapping not deterministic: %v vs %v", m1, m2)
	}
}
