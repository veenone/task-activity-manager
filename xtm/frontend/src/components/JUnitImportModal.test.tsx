import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { JUnitImportPreview } from "../api";
import { JUnitImportModal } from "./JUnitImportModal";

const { analyze, apply } = vi.hoisted(() => ({ analyze: vi.fn(), apply: vi.fn() }));

vi.mock("../contexts/ProfileContext", () => ({ useProfile: () => ({ activeId: "p1" }) }));
vi.mock("../files", () => ({ fileToBase64: () => Promise.resolve({ b64: "xml" }) }));
vi.mock("../api", async (original) => {
  const actual = await original<typeof import("../api")>();
  return { ...actual, AnalyzeJUnitImport: analyze, ApplyJUnitImport: apply };
});

function preview(): JUnitImportPreview {
  return {
    execKey: "TE-1",
    total: 3,
    matched: [],
    skipped: [],
    mappings: [
      {
        testcase: "login alice",
        testKey: "TC-1",
        summary: "Login",
        result: "PASS",
        included: true,
        reason: "",
        datasetSourceKind: "test",
        datasetSourceKey: "TC-1",
        candidateIterations: [{ iterationId: "i-alice", currentRun: "FAIL", parameters: [{ name: "USER", value: "alice" }] }],
        match: {
          testcase: "login alice", testKey: "TC-1", summary: "Login", result: "PASS", currentRun: "FAIL",
          targetKind: "iteration", iterationId: "i-alice", parameters: [{ name: "USER", value: "alice" }],
          datasetSourceKind: "test", datasetSourceKey: "TC-1",
        },
      },
      {
        testcase: "login region",
        testKey: "TC-1",
        summary: "Login",
        result: "FAIL",
        included: true,
        reason: "missing dataset property REGION",
        datasetSourceKind: "test",
        datasetSourceKey: "TC-1",
        candidateIterations: [
          { iterationId: "i-eu", currentRun: "PASS", parameters: [{ name: "REGION", value: "a-very-long-european-region-value" }] },
          { iterationId: "i-us", currentRun: "FAIL", parameters: [{ name: "REGION", value: "us" }] },
        ],
      },
      {
        testcase: "plain test",
        testKey: "TC-2",
        summary: "Plain",
        result: "PASS",
        included: true,
        reason: "",
        datasetSourceKind: "",
        datasetSourceKey: "",
        candidateIterations: [],
        match: {
          testcase: "plain test", testKey: "TC-2", summary: "Plain", result: "PASS", currentRun: "FAIL",
          targetKind: "run", iterationId: "", parameters: [], datasetSourceKind: "", datasetSourceKey: "",
        },
      },
    ],
  };
}

async function upload() {
  render(<JUnitImportModal execKey="TE-1" onCancel={vi.fn()} onApplied={vi.fn()} />);
  const input = document.querySelector('input[type="file"]') as HTMLInputElement;
  fireEvent.change(input, { target: { files: [new File(["xml"], "results.xml")] } });
  await screen.findByText("login alice");
}

describe("JUnitImportModal iteration mappings", () => {
  beforeEach(() => {
    analyze.mockReset().mockResolvedValue(preview());
    apply.mockReset().mockResolvedValue({ succeeded: ["TC-1", "TC-2"], failed: [] });
  });

  it("shows automatic and unresolved mappings and blocks Apply", async () => {
    await upload();

    expect(screen.getAllByText("USER=alice").some((element) => element.tagName === "SPAN")).toBe(true);
    expect(screen.getByRole("combobox", { name: /iteration for login region/i })).toHaveValue("");
    expect(screen.getByRole("button", { name: /apply/i })).toBeDisabled();
    expect(screen.getByText(/a-very-long-european-region-value/)).toBeInTheDocument();
  });

  it("allows manual mapping or exclusion and submits mixed run targets", async () => {
    const user = userEvent.setup();
    await upload();

    await user.selectOptions(screen.getByRole("combobox", { name: /iteration for login region/i }), "i-eu");
    const applyButton = screen.getByRole("button", { name: /apply 3 results/i });
    expect(applyButton).toBeEnabled();
    await user.click(applyButton);

    await waitFor(() => expect(apply).toHaveBeenCalledTimes(1));
    const submitted = apply.mock.calls[0][2];
    expect(submitted).toEqual(expect.arrayContaining([
      expect.objectContaining({ targetKind: "iteration", iterationId: "i-eu" }),
      expect.objectContaining({ targetKind: "run", testKey: "TC-2" }),
    ]));
  });

  it("reports duplicate iteration targets until one row is excluded", async () => {
    const p = preview();
    p.mappings[1].candidateIterations = p.mappings[0].candidateIterations;
    analyze.mockResolvedValue(p);
    const user = userEvent.setup();
    await upload();

    await user.selectOptions(screen.getByRole("combobox", { name: /iteration for login region/i }), "i-alice");
    expect(screen.getByRole("alert")).toHaveTextContent(/same target/i);
    expect(screen.getByRole("button", { name: /apply/i })).toBeDisabled();
    await user.click(screen.getByRole("checkbox", { name: /include login region/i }));
    expect(screen.queryByRole("alert")).not.toBeInTheDocument();
    expect(screen.getByRole("button", { name: /apply 2 results/i })).toBeEnabled();
  });
});
