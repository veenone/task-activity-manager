import { useEffect } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import { MemberTable } from "./MemberTable";
import type { ContainerState } from "./useContainerState";
import type { ContainerActions, ContainerViewProps } from "./useContainerActions";

vi.mock("./ParameterizedRunRows", () => ({
  ParameterizedRunRows: ({ expanded, onLoaded }: { expanded: boolean; onLoaded?: () => void }) => {
    useEffect(() => { if (expanded) onLoaded?.(); }, [expanded, onLoaded]);
    return expanded ? <div>Nested iterations loaded</div> : null;
  },
}));

function fixture() {
  const setExpandedRuns = vi.fn();
  const state = {
    profileId: "p1", kind: "testexec", selected: "EXEC-1",
    setBugFor: vi.fn(), setLinkBugFor: vi.fn(), selectedRuns: new Set<string>(), setSelectedRuns: vi.fn(),
    bulkStatus: "PASS", setBulkStatus: vi.fn(), setBoardPage: vi.fn(), memberFvFilter: "",
    setDetailKey: vi.fn(), setDetailVersion: vi.fn(), pageSize: 15, setPageSize: vi.fn(),
    caps: { supportsParameterizedRunDetail: true, supportsParameterizedRunWrites: true, supportsBugCreation: false, supportsBugRouting: false },
    containers: [{ key: "EXEC-1" }], board: { rows: [] }, memberRuns: new Map(), kindLabel: "Test Execution",
    allRows: [{ testKey: "QA-1", summary: "Parameterized login", status: "Open", runStatus: "FAIL", isExternal: false }],
    boardTotalPages: 1, safePage: 0,
    pageRows: [{ testKey: "QA-1", summary: "Parameterized login", status: "Open", runStatus: "FAIL", isExternal: false }],
    expandedRuns: new Set<string>(), setExpandedRuns,
    parameterizedTests: new Set<string>(), setParameterizedTests: vi.fn(),
  } as unknown as ContainerState;
  const actions = {
    removeTest: vi.fn(), setRunStatus: vi.fn(), unlinkBug: vi.fn(), setRunComment: vi.fn(),
    toggleRun: vi.fn(), toggleRowSort: vi.fn(), rowSortIndicator: vi.fn(() => ""),
    applyBulkRunStatus: vi.fn(), openBug: vi.fn(),
  } as unknown as ContainerActions;
  const props = { isDemo: true, onChanged: vi.fn() } as ContainerViewProps;
  return { state, actions, props, setExpandedRuns };
}

describe("MemberTable parameterized rows", () => {
  it("expands a Test Execution member from the keyboard", async () => {
    const { state, actions, props, setExpandedRuns } = fixture();
    render(<MemberTable s={state} a={actions} p={props} />);
    const button = screen.getByRole("button", { name: /show dataset iterations for qa-1/i });
    button.focus();
    await userEvent.keyboard("{Enter}");
    expect(setExpandedRuns).toHaveBeenCalled();
  });

  it("disables the aggregate selector after iterations own the result", () => {
    const { state, actions, props } = fixture();
    state.parameterizedTests = new Set(["EXEC-1:QA-1"]);
    render(<MemberTable s={state} a={actions} p={props} />);
    expect(screen.getByRole("combobox", { name: /overall result for qa-1/i })).toBeDisabled();
    expect(screen.getByText("Calculated from iterations")).toBeInTheDocument();
  });
});
