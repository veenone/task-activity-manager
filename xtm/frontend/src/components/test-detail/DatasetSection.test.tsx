import React from "react";
import { render as testingRender, screen } from "@testing-library/react";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DatasetSection, DATASET_ROW_CAP } from "./DatasetSection";
import type { TestDataset } from "../../api";
import { keys } from "../../queries/keys";

const getTestDataset = vi.fn();
const getContainerDataset = vi.fn();
const saveTestDataset = vi.fn();
vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return {
    ...actual,
    GetTestDataset: (...args: unknown[]) => getTestDataset(...args),
    GetContainerDataset: (...args: unknown[]) => getContainerDataset(...args),
    SaveTestDataset: (...args: unknown[]) => saveTestDataset(...args),
  };
});

// The editor asks for confirmation when a parameter is renamed, and
// DialogContext is not mounted here.
vi.mock("../useConfirm", () => ({ useConfirm: () => ({ confirm: vi.fn(async () => false) }) }));

function dataset(over: Partial<TestDataset> = {}): TestDataset {
  return {
    parameters: [
      { name: "USERNAME", combinatorial: false },
      { name: "REGION", combinatorial: false },
    ],
    rows: [
      ["alice", "eu-west"],
      ["bob", "us-east"],
    ],
    ...over,
  };
}

function rowsOf(n: number, cols = 1): string[][] {
  return Array.from({ length: n }, (_, i) => Array.from({ length: cols }, () => `v${i}`));
}

function render(ui: React.ReactElement) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  const result = testingRender(
    <QueryClientProvider client={client}>{ui}</QueryClientProvider>,
  );
  return { ...result, client };
}

beforeEach(() => {
  getTestDataset.mockReset();
  getTestDataset.mockResolvedValue(dataset());
  getContainerDataset.mockReset();
  getContainerDataset.mockResolvedValue({ parameters: [], rows: [] });
  saveTestDataset.mockReset();
  saveTestDataset.mockResolvedValue(undefined);
});

describe("DatasetSection", () => {
  it("shows each parameter and the values of each row", async () => {
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    expect(await screen.findByText("USERNAME")).toBeInTheDocument();
    expect(screen.getByText("REGION")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
    expect(screen.getByText("us-east")).toBeInTheDocument();
  });

  it("counts iterations when no parameter is combinatorial, because then it is the row count", async () => {
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    expect(await screen.findByText(/2 iterations/)).toBeInTheDocument();
  });

  it("reports rows but not iterations when a parameter is combinatorial", async () => {
    getTestDataset.mockResolvedValue(
      dataset({
        parameters: [
          { name: "USERNAME", combinatorial: false },
          { name: "PRIORITY", combinatorial: true },
        ],
      }),
    );
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    expect(await screen.findByText(/2 rows/)).toBeInTheDocument();
    expect(screen.queryByText(/iterations/)).not.toBeInTheDocument();
  });

  it("marks a combinatorial parameter so its extra iterations are not a surprise", async () => {
    getTestDataset.mockResolvedValue(
      dataset({
        parameters: [
          { name: "USERNAME", combinatorial: false },
          { name: "PRIORITY", combinatorial: true },
        ],
      }),
    );
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    expect(await screen.findByTitle(/combinatorial/i)).toBeInTheDocument();
  });

  it("renders nothing when the test has no dataset", async () => {
    getTestDataset.mockResolvedValue({ parameters: [], rows: [] });
    const { container } = render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    await vi.waitFor(() => expect(getTestDataset).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("renders nothing and asks for nothing when the backend has no datasets", () => {
    render(<DatasetSection profileId="p1" testKey="QA-1" supported={false} />);

    expect(getTestDataset).not.toHaveBeenCalled();
  });

  it("caps the rows it draws and says how many there are in total", async () => {
    const total = DATASET_ROW_CAP + 40;
    getTestDataset.mockResolvedValue({
      parameters: [{ name: "A", combinatorial: false }],
      rows: rowsOf(total),
    });
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    expect(await screen.findByText(new RegExp(`${DATASET_ROW_CAP} of ${total}`))).toBeInTheDocument();
    expect(screen.getByText("v0")).toBeInTheDocument();
    expect(screen.queryByText(`v${total - 1}`)).not.toBeInTheDocument();
  });

  it("draws every row when the dataset fits under the cap", async () => {
    getTestDataset.mockResolvedValue({
      parameters: [{ name: "A", combinatorial: false }],
      rows: rowsOf(3),
    });
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    expect(await screen.findByText("v2")).toBeInTheDocument();
    expect(screen.queryByText(/showing/i)).not.toBeInTheDocument();
  });

  it("refetches past the cache when Refresh is pressed", async () => {
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);
    await screen.findByText("USERNAME");

    await userEvent.click(screen.getByRole("button", { name: /refresh/i }));

    await vi.waitFor(() => expect(getTestDataset).toHaveBeenCalledTimes(2));
    expect(getTestDataset).toHaveBeenLastCalledWith("p1", "QA-1", true);
  });

  it("offers to create a dataset on a test that has none, where the backend takes writes", async () => {
    getTestDataset.mockResolvedValue({ parameters: [], rows: [] });
    render(<DatasetSection profileId="p1" testKey="QA-1" supported canEdit />);

    expect(await screen.findByRole("button", { name: /create dataset/i })).toBeInTheDocument();
  });

  it("offers nothing on a backend that cannot write datasets", async () => {
    getTestDataset.mockResolvedValue({ parameters: [], rows: [] });
    const { container } = render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    await vi.waitFor(() => expect(getTestDataset).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("opens the editor on an existing dataset and shows what it saved", async () => {
    const { client } = render(<DatasetSection profileId="p1" testKey="QA-1" supported canEdit />);
    await screen.findByText("USERNAME");

    await userEvent.click(screen.getByRole("button", { name: /^edit$/i }));
    await userEvent.clear(screen.getByLabelText("Row 1, USERNAME"));
    await userEvent.type(screen.getByLabelText("Row 1, USERNAME"), "carol");
    await userEvent.click(screen.getByRole("button", { name: "Save" }));

    // Back to the table, showing the edit without a re-fetch.
    expect(await screen.findByText("carol")).toBeInTheDocument();
    expect(screen.queryByLabelText("Dataset editor")).not.toBeInTheDocument();
    expect(getTestDataset).toHaveBeenCalledTimes(1);
    expect(client.getQueryData<TestDataset>(keys.testDataset("p1", "QA-1"))?.rows[0][0]).toBe("carol");
  });

  it("does not offer the editor on a backend that cannot write", async () => {
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);
    await screen.findByText("USERNAME");

    expect(screen.queryByRole("button", { name: /^edit$/i })).not.toBeInTheDocument();
  });

  // Xray resolves a dataset nearest-first, so an execution's own table is
  // what runs there. Which one is on screen has to be said, not guessed
  // (#277 W3).
  it("shows the execution's dataset and names the level", async () => {
    getContainerDataset.mockResolvedValue({
      parameters: [{ name: "TIER", combinatorial: false }],
      rows: [["run-only"]],
    });
    render(<DatasetSection profileId="p1" testKey="QA-1" supported containerKey="EXEC-1" />);

    expect(await screen.findByText("run-only")).toBeInTheDocument();
    expect(screen.getByText(/defined in EXEC-1/)).toBeInTheDocument();
    // The test's own table is not what runs here, so it is not what is shown.
    expect(screen.queryByText("alice")).not.toBeInTheDocument();
  });

  it("shows the test's dataset, and says so, when the execution defines none", async () => {
    render(<DatasetSection profileId="p1" testKey="QA-1" supported containerKey="EXEC-1" />);

    expect(await screen.findByText("alice")).toBeInTheDocument();
    expect(screen.getByText(/from the test/i)).toBeInTheDocument();
  });

  it("asks for no override when no container is selected", async () => {
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    await screen.findByText("alice");
    expect(getContainerDataset).not.toHaveBeenCalled();
    // With no container there is one level and nothing to disambiguate.
    expect(screen.queryByText(/from the test/i)).not.toBeInTheDocument();
  });

  // Writing at execution level is out of scope on #277, and the editor saves
  // to the test. Offering it over an override would move the wrong table.
  it("does not offer to edit a dataset the execution defines", async () => {
    getContainerDataset.mockResolvedValue({
      parameters: [{ name: "TIER", combinatorial: false }],
      rows: [["run-only"]],
    });
    render(<DatasetSection profileId="p1" testKey="QA-1" supported canEdit containerKey="EXEC-1" />);

    await screen.findByText("run-only");
    expect(screen.queryByRole("button", { name: /^edit$/i })).not.toBeInTheDocument();
  });

  it("surfaces a read failure instead of pretending the test has no dataset", async () => {
    getTestDataset.mockRejectedValue(new Error("jira unreachable"));
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    expect(await screen.findByText(/jira unreachable/)).toBeInTheDocument();
  });
});
