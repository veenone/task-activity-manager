import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";

import type { DatasetParameter, Step } from "../../api";
import { StepRow } from "./StepRow";

const step: Step = {
  xrayId: "step-1",
  index: 0,
  action: "Sign in as ${USERNAME}",
  data: "alice",
  expected: "Welcome",
  calledTestKey: "",
};
const parameters: DatasetParameter[] = [{ name: "USERNAME", combinatorial: false }];

function renderRow(overrides: Partial<React.ComponentProps<typeof StepRow>> = {}) {
  render(
    <ol>
      <StepRow
        profileId="p1"
        testKey="QA-1"
        step={step}
        parameters={parameters}
        pendingForTest={[]}
        isFirst
        isLast
        confirm={vi.fn().mockResolvedValue(true)}
        onMove={vi.fn()}
        onDuplicate={vi.fn()}
        onLocalChange={vi.fn()}
        onLocalDelete={vi.fn()}
        onEdited={vi.fn()}
        {...overrides}
      />
    </ol>,
  );
}

describe("StepRow dataset macros", () => {
  it("offers the helper in Action, Data, and Expected", () => {
    renderRow();
    expect(
      screen.getByRole("button", {
        name: "Insert dataset parameter into Action",
      }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", {
        name: "Insert dataset parameter into Data",
      }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", {
        name: "Insert dataset parameter into Expected",
      }),
    ).toBeEnabled();
  });

  it("keeps the helpers discoverable while the dataset has no parameters", async () => {
    renderRow({ parameters: [] });

    const helper = screen.getByRole("button", {
      name: "Insert dataset parameter into Data",
    });
    expect(helper).toBeEnabled();
    await userEvent.click(helper);
    expect(screen.getByText("Create a dataset parameter before inserting a macro.")).toBeVisible();
  });

  it("does not add helpers to read-only or call-test rows", () => {
    const { unmount } = render(
      <ol>
        <StepRow
          profileId="p1"
          testKey="QA-1"
          step={step}
          parameters={parameters}
          pendingForTest={[]}
          isFirst
          isLast
          readOnly
          confirm={vi.fn()}
          onMove={vi.fn()}
          onDuplicate={vi.fn()}
          onLocalChange={vi.fn()}
          onLocalDelete={vi.fn()}
          onEdited={vi.fn()}
        />
      </ol>,
    );
    expect(screen.queryByRole("button", { name: "Insert dataset parameter" })).not.toBeInTheDocument();
    unmount();

    renderRow({ step: { ...step, calledTestKey: "QA-2" } });
    expect(screen.queryByRole("button", { name: "Insert dataset parameter" })).not.toBeInTheDocument();
  });
});
