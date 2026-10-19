import { render, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
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
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <DatasetEditor
        profileId="p1"
        testKey="QA-1"
        initial={initial}
        onSaved={onSaved}
        onCancel={onCancel}
      />
    </QueryClientProvider>,
  );
  return { onSaved, onCancel, client };
}

async function addParameter(name: string) {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: "Add parameter" }));
  await user.type(screen.getByLabelText("Parameter name"), name);
  await user.click(screen.getByRole("button", { name: "Save parameter" }));
}

async function openParameter(index: number) {
  const user = userEvent.setup();
  await user.click(screen.getByRole("button", { name: `Actions for parameter ${index}` }));
  await user.click(screen.getByRole("menuitem", { name: "Edit parameter" }));
  return user;
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

    await addParameter("USERNAME");
    await userEvent.type(screen.getByLabelText("Row 1, USERNAME"), "alice");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(saveTestDataset).toHaveBeenCalledWith("p1", "QA-1", {
      parameters: [{ name: "USERNAME", combinatorial: false, type: "TEXT" }],
      rows: [["alice"]],
    });
    await vi.waitFor(() =>
      expect(onSaved).toHaveBeenCalledWith({
        parameters: [{ name: "USERNAME", combinatorial: false, type: "TEXT" }],
        rows: [["alice"]],
      }),
    );
  });

  it("adds a parameter as a column on every existing row", async () => {
    editor(twoByTwo);

    await addParameter("TIER");
    await userEvent.type(screen.getByLabelText("Row 2, TIER"), "gold");
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

    await userEvent.click(screen.getByRole("button", { name: "Actions for parameter 1" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Remove parameter" }));
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

    const user = await openParameter(2);
    await user.click(screen.getByLabelText("Generate combinations from this parameter"));
    await user.click(screen.getByRole("button", { name: "Save parameter" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.parameters[1]).toEqual({
      name: "REGION",
      combinatorial: true,
      type: "TEXT",
    });
  });

  it("shows why a save was refused and keeps the table to fix", async () => {
    saveTestDataset.mockRejectedValue(new Error('the parameter "USERNAME" appears more than once'));
    const { onSaved } = editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByText(/appears more than once/)).toBeInTheDocument();
    expect(onSaved).not.toHaveBeenCalled();
    expect(screen.getByRole("columnheader", { name: /USERNAME/ })).toBeVisible();
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

    const cell = screen.getByRole("combobox", { name: "Row 1, TIER" });
    expect(Array.from(cell.querySelectorAll("option")).map((option) => option.value)).toEqual(
      ["", "gold", "silver"],
    );

    await userEvent.selectOptions(cell, "silver");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    const saved = saveTestDataset.mock.calls[0][2] as TestDataset;
    expect(saved.parameters[0]).toEqual(listed.parameters[0]);
    expect(saved.rows).toEqual([["silver"]]);
  });

  it("moves a parameter and the values under it", async () => {
    editor(twoByTwo);

    await userEvent.click(screen.getByRole("button", { name: "Actions for parameter 1" }));
    await userEvent.click(screen.getByRole("menuitem", { name: "Move right" }));
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

    const user = await openParameter(1);
    const name = screen.getByLabelText("Parameter name");
    await userEvent.clear(name);
    await userEvent.type(name, "ACCOUNT");
    await user.click(screen.getByRole("button", { name: "Save parameter" }));
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

    const user = await openParameter(1);
    const name = screen.getByLabelText("Parameter name");
    await userEvent.clear(name);
    await userEvent.type(name, "ACCOUNT");
    await user.click(screen.getByRole("button", { name: "Save parameter" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    await vi.waitFor(() => expect(onSaved).toHaveBeenCalled());
    expect(confirm).toHaveBeenCalledTimes(1);
    expect(renamePlaceholder).not.toHaveBeenCalled();
  });

  it("asks nothing when a parameter is added rather than renamed", async () => {
    const { onSaved } = editor(twoByTwo);

    await addParameter("TIER");
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

    expect(await screen.findByRole("columnheader", { name: /TIER/ })).toBeVisible();
    expect(screen.getByRole("columnheader", { name: /ROUND/ })).toBeVisible();
    // The file replaces the table, so what was there is gone.
    expect(screen.queryByRole("columnheader", { name: /USERNAME/ })).not.toBeInTheDocument();
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
    expect(screen.getByRole("columnheader", { name: /USERNAME/ })).toBeVisible();
  });

  it("exports the table on screen, not the one it started with", async () => {
    editor(twoByTwo);

    await userEvent.clear(screen.getByLabelText("Row 1, USERNAME"));
    await userEvent.type(screen.getByLabelText("Row 1, USERNAME"), "carol");
    await userEvent.click(screen.getByRole("button", { name: "Export CSV" }));

    await vi.waitFor(() => expect(exportDatasetCSV).toHaveBeenCalled());
    const [key, table] = exportDatasetCSV.mock.calls[0] as [string, TestDataset];
    expect(key).toBe("QA-1");
    expect(table.rows[0][0]).toBe("carol");
  });

  it("leaves the dataset alone on cancel", async () => {
    const { onCancel } = editor(twoByTwo);

    await userEvent.type(screen.getByLabelText("Row 1, USERNAME"), "typo");
    await userEvent.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onCancel).toHaveBeenCalled();
    expect(saveTestDataset).not.toHaveBeenCalled();
  });

  it("opens parameter configuration from a compact header menu", async () => {
    editor(twoByTwo);
    const user = userEvent.setup();

    await user.click(screen.getByRole("button", { name: "Actions for parameter 1" }));
    await user.click(screen.getByRole("menuitem", { name: "Edit parameter" }));

    expect(screen.getByRole("dialog", { name: "Edit parameter" })).toBeVisible();
    expect(screen.getByLabelText("Parameter name")).toHaveValue("USERNAME");
  });

  it("uses semantic row numbers and selects for List values", () => {
    const listed: TestDataset = {
      parameters: [
        {
          name: "TIER",
          combinatorial: false,
          type: "LIST",
          options: ["gold", "silver"],
          listScope: "PARAMETER_AD_HOC",
        },
      ],
      rows: [["gold"]],
    };
    editor(listed);

    expect(screen.getByRole("rowheader", { name: "1" })).toBeVisible();
    expect(screen.getByRole("combobox", { name: "Row 1, TIER" })).toHaveValue("gold");
    expect(screen.getByRole("group", { name: "Dataset toolbar" })).toBeVisible();
    expect(screen.getByRole("group", { name: "Dataset actions" })).toBeVisible();
  });
});
