import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { DatasetSection, DATASET_ROW_CAP } from "./DatasetSection";
import type { TestDataset } from "../../api";

const getTestDataset = vi.fn();
vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, GetTestDataset: (...args: unknown[]) => getTestDataset(...args) };
});

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

beforeEach(() => {
  getTestDataset.mockReset();
  getTestDataset.mockResolvedValue(dataset());
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

  it("surfaces a read failure instead of pretending the test has no dataset", async () => {
    getTestDataset.mockRejectedValue(new Error("jira unreachable"));
    render(<DatasetSection profileId="p1" testKey="QA-1" supported />);

    expect(await screen.findByText(/jira unreachable/)).toBeInTheDocument();
  });
});
