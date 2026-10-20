import { render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";

import type { DatasetParameter, Step } from "../../api";
import { StepsSection } from "./StepsSection";
import type { TestDetailState } from "./useTestDetailState";
import type { TestDetailActions } from "./useTestDetailActions";
import type { TestDetailProps } from "./types";

vi.mock("./StepRow", () => ({
  StepRow: ({ parameters }: { parameters: DatasetParameter[] }) => (
    <li data-testid="step-parameters">{parameters.map((parameter) => parameter.name).join(",")}</li>
  ),
  InlineStepsEditor: () => null,
}));

const step: Step = {
  xrayId: "step-1", index: 0, action: "", data: "", expected: "", calledTestKey: "",
};

function state(parameters: DatasetParameter[]): TestDetailState {
  return {
    profileId: "p1",
    confirm: vi.fn(),
    steps: [step],
    setSteps: vi.fn(),
    stepsLoading: false,
    stepsError: "",
    setShowCloneSteps: vi.fn(),
    setShowCallPicker: vi.fn(),
    setShowCrossCall: vi.fn(),
    setShowCrossClone: vi.fn(),
    jiraStepInfo: null,
    caps: { stepModel: "objects" },
    crossProjectEnabled: false,
    dataset: { parameters, rows: [] },
  } as unknown as TestDetailState;
}

const actions = {
  refreshSteps: vi.fn(), addStep: vi.fn(), duplicateStep: vi.fn(), moveStep: vi.fn(),
} as unknown as TestDetailActions;
const props = {
  testKey: "QA-1", pendingForTest: [], onEdited: vi.fn(),
} as unknown as TestDetailProps;

describe("StepsSection dataset parameters", () => {
  it("passes only the current Test default parameters to each manual step", () => {
    const { rerender } = render(
      <StepsSection s={state([{ name: "QA1", combinatorial: false }])} a={actions} p={props} />,
    );
    expect(screen.getByTestId("step-parameters")).toHaveTextContent("QA1");

    rerender(
      <StepsSection
        s={state([{ name: "QA2", combinatorial: false }])}
        a={actions}
        p={{ ...props, testKey: "QA-2" }}
      />,
    );
    expect(screen.getByTestId("step-parameters")).toHaveTextContent("QA2");
    expect(screen.getByTestId("step-parameters")).not.toHaveTextContent("QA1");
  });
});
