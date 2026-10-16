import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DatasetEditor } from "./DatasetEditor";
import type { TestDataset } from "../../api";

const saveTestDataset = vi.fn();
vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, SaveTestDataset: (...args: unknown[]) => saveTestDataset(...args) };
});

const empty: TestDataset = { parameters: [], rows: [] };

const twoByTwo: TestDataset = {
  parameters: [
    { name: "USERNAME", combinatorial: false },
    { name: "REGION", combinatorial: false },
  ],
  rows: [
    ["alice", "eu-west"],
    ["bob", "us-east"],
  ],
};

function editor(initial: TestDataset, onSaved = vi.fn(), onCancel = vi.fn()) {
  render(
    <DatasetEditor
      profileId="p1"
      testKey="QA-1"
      initial={initial}
      onSaved={onSaved}
      onCancel={onCancel}
    />,
  );
  return { onSaved, onCancel };
}

beforeEach(() => {
  saveTestDataset.mockReset();
  saveTestDataset.mockResolvedValue(undefined);
});

describe("DatasetEditor", () => {
  it("saves the parameter and row the user typed", async () => {
    const { onSaved } = editor(empty);

    await userEvent.type(screen.getByLabelText("Parameter 1 name"), "USERNAME");
    await userEvent.type(screen.getByLabelText("Row 1, parameter 1"), "alice");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(saveTestDataset).toHaveBeenCalledWith("p1", "QA-1", {
      parameters: [{ name: "USERNAME", combinatorial: false }],
      rows: [["alice"]],
    });
    await vi.waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith({
        parameters: [{ name: "USERNAME", combinatorial: false }],
        rows: [["alice"]],
      }),
    );
  });

  it("adds a parameter as a column on every existing row", async () => {
    editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Add parameter" }));
    await userEvent.type(screen.getByLabelText("Parameter 3 name"), "TIER");
    await userEvent.type(screen.getByLabelText("Row 2, parameter 3"), "gold");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.parameters.map((p) => p.name)).toEqual(["USERNAME", "REGION", "TIER"]);
    // The row the user did not touch keeps its cells and gains an empty one,
    // rather than going ragged.
    expect(saved.rows).toEqual([
      ["alice", "eu-west", ""],
      ["bob", "us-east", "gold"],
    ]);
  });

  it("removes a parameter with the whole column of values under it", async () => {
    editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Remove parameter 1" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.parameters.map((p) => p.name)).toEqual(["REGION"]);
    expect(saved.rows).toEqual([["eu-west"], ["us-east"]]);
  });

  it("removes a row", async () => {
    editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Remove row 1" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.rows).toEqual([["bob", "us-east"]]);
  });

  it("marks a parameter combinatorial, which is what makes Xray expand it", async () => {
    editor(twoByTwo);

    await userEvent.click(screen.getByLabelText("Parameter 2 is combinatorial"));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.parameters[1]).toEqual({ name: "REGION", combinatorial: true });
  });

  it("shows why a save was refused and keeps the table to fix", async () => {
    saveTestDataset.mockRejectedValue(new Error('the parameter "USERNAME" appears more than once'));
    const { onSaved } = editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText(/appears more than once/)).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
    expect(screen.getByLabelText("Parameter 1 name")).toHaveValue("USERNAME");
  });

  it("leaves the dataset alone on cancel", async () => {
    const { onCancel } = editor(twoByTwo);

    await userEvent.type(screen.getByLabelText("Row 1, parameter 1"), "typo");
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onCancel).toHaveBeenCalled();
    expect(saveTestDataset).not.toHaveBeenCalled();
  });
});
