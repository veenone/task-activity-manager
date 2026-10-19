import { fireEvent, render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { JUnitNewExecModal } from "./JUnitNewExecModal";

const { analyze, apply } = vi.hoisted(() => ({ analyze: vi.fn(), apply: vi.fn() }));
vi.mock("../contexts/ProfileContext", () => ({ useProfile: () => ({ activeId: "p1" }) }));
vi.mock("../files", () => ({ fileToBase64: () => Promise.resolve({ b64: "xml" }) }));
vi.mock("../api", async (original) => {
  const actual = await original<typeof import("../api")>();
  return { ...actual, AnalyzeJUnitImportNewExec: analyze, ApplyJUnitImportNewExec: apply };
});

describe("JUnitNewExecModal dataset iterations", () => {
  beforeEach(() => {
    analyze.mockReset().mockResolvedValue({
      total: 1,
      skipped: [],
      rows: [{
        testcase: "Existing One", testKey: "TC-1", summary: "Existing One", result: "FAIL", create: false,
        targetKind: "iteration", parameters: [{ name: "USER", value: "bob" }],
        datasetSourceKind: "test", datasetSourceKey: "TC-1", included: true, reason: "",
        candidateIterations: [{ iterationId: "signature-bob", currentRun: "", parameters: [{ name: "USER", value: "bob" }] }],
      }],
    });
    apply.mockReset().mockResolvedValue({ execKey: "new-container-1", created: 0, allocated: 1, resultsSet: 1, failed: [] });
  });

  it("shows and submits an existing test's stable parameter signature", async () => {
    const user = userEvent.setup();
    render(<JUnitNewExecModal onCancel={vi.fn()} onApplied={vi.fn()} />);
    await user.type(screen.getByRole("textbox", { name: /execution summary/i }), "Imported");
    await user.click(screen.getByRole("button", { name: /choose junit/i }));
    const input = document.querySelector('input[type="file"]') as HTMLInputElement;
    fireEvent.change(input, { target: { files: [new File(["xml"], "results.xml")] } });

    expect(await screen.findByText("USER=bob")).toBeVisible();
    await user.click(screen.getByRole("button", { name: /create execution/i }));
    expect(apply).toHaveBeenCalledWith("p1", "Imported", [expect.objectContaining({ targetKind: "iteration", parameters: [{ name: "USER", value: "bob" }] })]);
  });
});
