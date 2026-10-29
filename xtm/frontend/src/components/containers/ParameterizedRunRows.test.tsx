import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import * as api from "../../api";
import type { ParameterizedRun } from "../../api";
import { ParameterizedRunIndicator, ParameterizedRunRows } from "./ParameterizedRunRows";

vi.mock("../../api", async (importOriginal) => ({
  ...(await importOriginal<typeof import("../../api")>()),
  GetParameterizedRun: vi.fn(),
  SetIterationStatus: vi.fn(),
  SetIterationStepStatus: vi.fn(),
}));

const run: ParameterizedRun = {
  id: "run-1",
  executionKey: "EXEC-1",
  testKey: "QA-1",
  datasetSourceKind: "test",
  datasetSourceKey: "QA-1",
  status: "FAIL",
  iterations: [
    {
      id: "iteration-1",
      rank: 0,
      status: "PASS",
      parameters: [
        { name: "USER", value: "alice" },
        { name: "REGION", value: "eu" },
      ],
      steps: [
        {
          id: "result-1",
          definitionId: "step-1",
          rank: 0,
          status: "PASS",
          action: "Sign in as alice",
          data: "Region eu",
          expected: "Dashboard",
        },
      ],
    },
    {
      id: "iteration-2",
      rank: 1,
      status: "FAIL",
      parameters: [
        { name: "USER", value: "bob" },
        { name: "REGION", value: "us" },
      ],
      steps: [],
    },
    {
      id: "iteration-3",
      rank: 2,
      status: "TO" + "DO",
      parameters: [
        { name: "USER", value: "carol" },
        { name: "REGION", value: "ap" },
      ],
      steps: [],
    },
  ],
};

function view(canWrite: boolean) {
  const client = new QueryClient({
    defaultOptions: { queries: { retry: false } },
  });
  vi.mocked(api.GetParameterizedRun).mockResolvedValue(structuredClone(run) as never);
  render(
    <QueryClientProvider client={client}>
      <table>
        <tbody>
          <tr>
            <td>
              <ParameterizedRunRows profileId="p1" execKey="EXEC-1" testKey="QA-1" expanded canWrite={canWrite} />
            </td>
          </tr>
        </tbody>
      </table>
    </QueryClientProvider>,
  );
  return client;
}

describe("ParameterizedRunRows", () => {
  beforeEach(() => vi.clearAllMocks());

  it("shows progress, source, parameter values and resolved steps read-only", async () => {
    view(false);
    expect(await screen.findByText("2 of 3 completed")).toBeInTheDocument();
    expect(screen.getByText("Test QA-1")).toBeInTheDocument();
    expect(screen.getByText("alice")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: /iteration 1/i }));
    expect(screen.getByText("Sign in as alice")).toBeInTheDocument();
    expect(screen.getByText("Read-only results")).toBeInTheDocument();
    expect(screen.queryByRole("combobox", { name: /iteration 1 status/i })).not.toBeInTheDocument();
  });

  it("patches the exact cached step after a supported edit", async () => {
    const client = view(true);
    await screen.findByText("2 of 3 completed");
    await userEvent.click(screen.getByRole("button", { name: /iteration 1/i }));
    await userEvent.selectOptions(screen.getByRole("combobox", { name: /step 1 status/i }), "FAIL");
    await waitFor(() => expect(api.SetIterationStepStatus).toHaveBeenCalledWith("p1", "EXEC-1", "QA-1", "iteration-1", "result-1", "FAIL"));
    expect((client.getQueryData(["parameterized-run", "p1", "EXEC-1", "QA-1"]) as ParameterizedRun).iterations[0].steps[0].status).toBe("FAIL");
  });

  it("keeps the current result visible and reports a write failure", async () => {
    vi.mocked(api.SetIterationStatus).mockRejectedValue(new Error("write refused"));
    view(true);
    await screen.findByText("2 of 3 completed");
    await userEvent.selectOptions(screen.getByRole("combobox", { name: /iteration 1 status/i }), "FAIL");
    expect(await screen.findByText("write refused")).toBeInTheDocument();
    expect(screen.getByRole("combobox", { name: /iteration 1 status/i })).toHaveValue("PASS");
  });

  it("identifies a parameterized member and shows its iteration count before expansion", async () => {
    const onLoaded = vi.fn();
    const onToggle = vi.fn();
    const client = new QueryClient({
      defaultOptions: { queries: { retry: false } },
    });
    vi.mocked(api.GetParameterizedRun).mockResolvedValue(structuredClone(run) as never);

    render(
      <QueryClientProvider client={client}>
        <ParameterizedRunIndicator profileId="p1" execKey="EXEC-1" testKey="QA-1" expanded={false} onLoaded={onLoaded} onToggle={onToggle} />
      </QueryClientProvider>,
    );

    expect(await screen.findByText("3 iterations")).toBeVisible();
    expect(onLoaded).toHaveBeenCalledWith(3);
    await userEvent.click(
      screen.getByRole("button", {
        name: /show 3 dataset iterations for qa-1/i,
      }),
    );
    expect(onToggle).toHaveBeenCalledTimes(1);
  });

  // #362: a read that failed used to render nothing, so a run the store refused
  // looked exactly like a test with no dataset.
  it("says so when the run cannot be read", async () => {
    const onLoaded = vi.fn();
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(api.GetParameterizedRun).mockRejectedValue(
      new Error("read the parameterized run of QA-1: iteration 11862 has a step with missing identity"),
    );

    render(
      <QueryClientProvider client={client}>
        <ParameterizedRunIndicator profileId="p1" execKey="EXEC-1" testKey="QA-1" expanded={false} onLoaded={onLoaded} onToggle={vi.fn()} />
      </QueryClientProvider>,
    );

    expect(await screen.findByText("iterations unavailable")).toBeVisible();
    expect(onLoaded).not.toHaveBeenCalled();
  });

  // A test with no dataset answers with the unsupported sentinel, and every
  // member row asks, so that answer must stay silent or every ordinary row
  // grows a warning.
  it("stays silent for a test that has no dataset", async () => {
    const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.mocked(api.GetParameterizedRun).mockRejectedValue(new Error("backend: operation not supported"));

    const { container } = render(
      <QueryClientProvider client={client}>
        <ParameterizedRunIndicator profileId="p1" execKey="EXEC-1" testKey="QA-1" expanded={false} onLoaded={vi.fn()} onToggle={vi.fn()} />
      </QueryClientProvider>,
    );

    await waitFor(() => expect(vi.mocked(api.GetParameterizedRun)).toHaveBeenCalled());
    expect(screen.queryByText("iterations unavailable")).not.toBeInTheDocument();
    expect(container).toBeEmptyDOMElement();
  });
});
