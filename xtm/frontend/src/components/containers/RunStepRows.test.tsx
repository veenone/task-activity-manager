import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";

import * as api from "../../api";
import type { RunStepResult } from "../../api";
import { RunStepRows } from "./RunStepRows";

vi.mock("../../api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../api")>()),
  GetRunStepResults: vi.fn(),
  SetRunStepStatus: vi.fn(),
}));

// The names a Zephyr instance configures. None of them is one of Xray's six,
// which is the point: a dropdown offering Xray's names on this profile sends
// a result the backend has no id for.
const ZEPHYR_STATUSES = ["UNEXECUTED", "PASS", "FAIL", "WIP", "BLOCKED", "PASS WARNING"];

const steps: RunStepResult[] = [
  {
    id: "4001",
    definitionId: "3001",
    rank: 1,
    status: "FAIL",
    action: "Open the gateway",
    data: "",
    expected: "the console loads",
    comment: "gateway returned no response",
    uncommitted: false,
  },
  {
    id: "4002",
    definitionId: "3002",
    rank: 2,
    status: "UNEXECUTED",
    action: "Submit the form",
    data: "valid payload",
    expected: "202 Accepted",
    comment: "",
    uncommitted: false,
  },
];

function renderRows(statuses = ZEPHYR_STATUSES) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <RunStepRows profileId="p1" execKey="501" testKey="PROJ-101" expanded runStatuses={statuses} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.mocked(api.GetRunStepResults).mockReset().mockResolvedValue(steps);
  vi.mocked(api.SetRunStepStatus).mockReset().mockResolvedValue(undefined);
});

it("offers the backend's own result names, not Xray's", async () => {
  renderRows();
  const select = await screen.findByLabelText("Step 1 result");
  const offered = Array.from(select.querySelectorAll("option")).map((option) => option.textContent);
  expect(offered).toEqual(ZEPHYR_STATUSES);
});

it("shows each step's recorded result and comment", async () => {
  renderRows();
  expect(await screen.findByLabelText("Step 1 result")).toHaveValue("FAIL");
  expect(screen.getByLabelText("Step 2 result")).toHaveValue("UNEXECUTED");
  expect(screen.getByText("gateway returned no response")).toBeInTheDocument();
});

it("journals a pick against the step result's own id and marks it uncommitted", async () => {
  renderRows();
  const select = await screen.findByLabelText("Step 1 result");
  await userEvent.selectOptions(select, "BLOCKED");

  await waitFor(() =>
    expect(api.SetRunStepStatus).toHaveBeenCalledWith("p1", "501", "PROJ-101", "4001", "BLOCKED"),
  );
  // The result is journalled and waits for Commit, and nothing refetches to
  // discover that, so the row has to say so itself (#366).
  expect(await screen.findByTitle("Set locally. Commit to send it to Jira.")).toBeInTheDocument();
  expect(select).toHaveValue("BLOCKED");
});

it("applies one result to every selected step", async () => {
  renderRows();
  await userEvent.click(await screen.findByLabelText("Select every step of PROJ-101"));
  await userEvent.selectOptions(
    screen.getByLabelText("Selected steps of PROJ-101: result to apply"),
    "PASS WARNING",
  );
  await userEvent.click(screen.getByRole("button", { name: "Apply" }));

  await waitFor(() => expect(api.SetRunStepStatus).toHaveBeenCalledTimes(2));
  expect(api.SetRunStepStatus).toHaveBeenCalledWith("p1", "501", "PROJ-101", "4001", "PASS WARNING");
  expect(api.SetRunStepStatus).toHaveBeenCalledWith("p1", "501", "PROJ-101", "4002", "PASS WARNING");
});

// A bulk apply keeps going and names what did not take, rather than stopping
// at the first refusal (#365).
it("reports the steps a bulk apply could not set", async () => {
  vi.mocked(api.SetRunStepStatus).mockRejectedValueOnce(new Error("400 Bad Request")).mockResolvedValue(undefined);
  renderRows();
  await userEvent.click(await screen.findByLabelText("Select every step of PROJ-101"));
  await userEvent.click(screen.getByRole("button", { name: "Apply" }));

  await waitFor(() => expect(api.SetRunStepStatus).toHaveBeenCalledTimes(2));
  expect(await screen.findByRole("alert")).toHaveTextContent("step 1");
});

// A status the instance no longer offers still has to be the one shown, or the
// select renders a different step's result as this step's.
it("keeps a recorded result that is no longer offered selectable", async () => {
  vi.mocked(api.GetRunStepResults).mockResolvedValue([{ ...steps[0], status: "RETIRED STATUS" }]);
  renderRows();
  const select = await screen.findByLabelText("Step 1 result");
  expect(select).toHaveValue("RETIRED STATUS");
});

// A backend that cannot address step results answers ErrUnsupported, and every
// member row of every execution asks. Reporting that as a failure would put an
// error on every row of an Xray execution.
it("reads an unsupported answer as a test with no step results", async () => {
  vi.mocked(api.GetRunStepResults).mockRejectedValue(new Error("backend: operation not supported"));
  renderRows();
  expect(await screen.findByText(/no recorded step results/)).toBeInTheDocument();
  expect(screen.queryByRole("alert")).not.toBeInTheDocument();
});

it("reports a read that failed for any other reason", async () => {
  vi.mocked(api.GetRunStepResults).mockRejectedValue(new Error("502 Bad Gateway"));
  renderRows();
  expect(await screen.findByText(/502 Bad Gateway/)).toBeInTheDocument();
});
