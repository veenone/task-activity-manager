import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DatasetEditor } from "./DatasetEditor";
import type { TestDataset } from "../../api";

const saveTestDataset = vi.fn();
const renamePlaceholder = vi.fn();
const importDatasetCSV = vi.fn();
const exportDatasetCSV = vi.fn();
vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return {
    ...actual,
    SaveTestDataset: (...args: unknown[]) => saveTestDataset(...args),
    RenameDatasetPlaceholder: (...args: unknown[]) => renamePlaceholder(...args),
    ImportDatasetCSV: (...args: unknown[]) => importDatasetCSV(...args),
    ExportDatasetCSV: (...args: unknown[]) => exportDatasetCSV(...args),
  };
});

// useConfirm wraps DialogContext, which is not mounted in these tests.
const confirm = vi.fn();
vi.mock("../useConfirm", () => ({ useConfirm: () => ({ confirm }) }));

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
  renamePlaceholder.mockReset();
  renamePlaceholder.mockResolvedValue(0);
  confirm.mockReset();
  confirm.mockResolvedValue(true);
  importDatasetCSV.mockReset();
  exportDatasetCSV.mockReset();
  exportDatasetCSV.mockResolvedValue("C:/tmp/QA-1-dataset.csv");
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

  // Xray's own bookkeeping comes back with the dataset and has to go back
  // untouched: a parameter saved without its id and list binding is rebuilt
  // by the server as free text, losing the allowed values (#283).
  it("offers a list parameter's allowed values and saves its binding untouched", async () => {
    const listed: TestDataset = {
      parameters: [
        {
          name: "TIER",
          combinatorial: false,
          id: 2741,
          type: "LIST",
          options: ["gold", "silver"],
          listScope: "PARAMETER_AD_HOC",
          listId: 162,
        },
      ],
      rows: [["gold"]],
    };
    editor(listed);

    // The cell offers what the list allows, rather than leaving the user to
    // remember it.
    const cell = screen.getByLabelText("Row 1, parameter 1");
    const list = document.getElementById(cell.getAttribute("list") ?? "");
    expect(list).not.toBeNull();
    expect(Array.from(list!.querySelectorAll("option")).map((o) => o.getAttribute("value"))).toEqual(
      ["gold", "silver"],
    );

    await userEvent.clear(cell);
    await userEvent.type(cell, "silver");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.parameters[0]).toEqual(listed.parameters[0]);
    expect(saved.rows).toEqual([["silver"]]);
  });

  it("moves a parameter and the values under it", async () => {
    editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Move parameter 1 right" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.parameters.map((p) => p.name)).toEqual(["REGION", "USERNAME"]);
    expect(saved.rows).toEqual([
      ["eu-west", "alice"],
      ["us-east", "bob"],
    ]);
  });

  it("moves a row, because the order is the order the iterations run in", async () => {
    editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Move row 2 up" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.rows).toEqual([
      ["bob", "us-east"],
      ["alice", "eu-west"],
    ]);
  });

  // A ${placeholder} refers to a parameter by name. Rename the parameter and
  // the reference resolves to nothing, and the test runs with the literal
  // text in place of a value (#277 W2).
  it("offers to rewrite the references after a rename, and does it when accepted", async () => {
    const existing: TestDataset = {
      parameters: [{ name: "USER", combinatorial: false, id: 2741 }],
      rows: [["alice"]],
    };
    editor(existing);

    const name = screen.getByLabelText("Parameter 1 name");
    await userEvent.clear(name);
    await userEvent.type(name, "ACCOUNT");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await vi.waitFor(() => expect(renamePlaceholder).toHaveBeenCalled());
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(renamePlaceholder).toHaveBeenCalledWith("p1", "QA-1", "USER", "ACCOUNT");
  });

  it("leaves the references alone when the rewrite is declined", async () => {
    confirm.mockResolvedValue(false);
    const existing: TestDataset = {
      parameters: [{ name: "USER", combinatorial: false, id: 2741 }],
      rows: [["alice"]],
    };
    const { onSaved } = editor(existing);

    const name = screen.getByLabelText("Parameter 1 name");
    await userEvent.clear(name);
    await userEvent.type(name, "ACCOUNT");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await vi.waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(renamePlaceholder).not.toHaveBeenCalled();
  });

  it("asks nothing when a parameter is added rather than renamed", async () => {
    const { onSaved } = editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Add parameter" }));
    await userEvent.type(screen.getByLabelText("Parameter 3 name"), "TIER");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await vi.waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(confirm).not.toHaveBeenCalled();
  });

  // A dataset is a table, and a spreadsheet is where people edit tables.
  it("shows the table an imported file describes", async () => {
    const imported: TestDataset = {
      parameters: [
        { name: "TIER", combinatorial: false, id: 2741 },
        { name: "ROUND", combinatorial: true },
      ],
      rows: [["gold", "1"]],
    };
    importDatasetCSV.mockResolvedValue(imported);
    editor(twoByTwo);

    await userEvent.upload(
      screen.getByLabelText("Import CSV"),
      new File(["TIER,ROUND*\ngold,1\n"], "dataset.csv", { type: "text/csv" }),
    );

    expect(await screen.findByDisplayValue("TIER")).toBeInTheDocument();
    expect(screen.getByDisplayValue("ROUND")).toBeInTheDocument();
    // The file replaces the table, so what was there is gone.
    expect(screen.queryByDisplayValue("USERNAME")).not.toBeInTheDocument();
    // The table the file replaced goes with it, so the parameters it still
    // has keep their identity.
    expect(importDatasetCSV).toHaveBeenCalledWith(expect.any(String), twoByTwo);
  });

  it("says why a file was refused and keeps the table on screen", async () => {
    importDatasetCSV.mockRejectedValue(new Error("that file has no parameter row"));
    editor(twoByTwo);

    await userEvent.upload(
      screen.getByLabelText("Import CSV"),
      new File([""], "empty.csv", { type: "text/csv" }),
    );

    expect(await screen.findByText(/no parameter row/)).toBeInTheDocument();
    expect(screen.getByDisplayValue("USERNAME")).toBeInTheDocument();
  });

  it("exports the table on screen, not the one it started with", async () => {
    editor(twoByTwo);

    await userEvent.clear(screen.getByLabelText("Row 1, parameter 1"));
    await userEvent.type(screen.getByLabelText("Row 1, parameter 1"), "carol");
    await userEvent.click(screen.getByRole("button", { name: "Export CSV" }));

    await vi.waitFor(() => expect(exportDatasetCSV).toHaveBeenCalled());
    const [key, table] = exportDatasetCSV.mock.calls[0] as [string, TestDataset];
    expect(key).toBe("QA-1");
    expect(table.rows[0][0]).toBe("carol");
  });

  it("leaves the dataset alone on cancel", async () => {
    const { onCancel } = editor(twoByTwo);

    await userEvent.type(screen.getByLabelText("Row 1, parameter 1"), "typo");
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onCancel).toHaveBeenCalled();
    expect(saveTestDataset).not.toHaveBeenCalled();
  });
});
