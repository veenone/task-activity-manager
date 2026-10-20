import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BulkStepReplaceModal } from "./BulkStepReplaceModal";

const scanSteps = vi.fn();
const replaceSteps = vi.fn();

vi.mock("../api", () => ({
  ScanTestStepText: (...a: unknown[]) => scanSteps(...a),
  ReplaceTestStepText: (...a: unknown[]) => replaceSteps(...a),
  errMsg: (e: unknown) => String(e),
}));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

const twoMatches = {
  matches: [
    {
      testKey: "QA-1",
      xrayId: "11",
      index: 1,
      field: "action",
      before: "GET /rest/raven/1.0/api/test",
      after: "GET /rest/raven/2.0/api/test",
      hits: 1,
    },
    {
      testKey: "QA-2",
      xrayId: "21",
      index: 3,
      field: "expected",
      before: "1.0 payload",
      after: "2.0 payload",
      hits: 1,
    },
  ],
  skipped: ["QA-3"],
  failed: [],
};

function renderModal(onComplete = vi.fn()) {
  render(
    <BulkStepReplaceModal
      testKeys={["QA-1", "QA-2", "QA-3"]}
      onComplete={onComplete}
      onCancel={() => {}}
    />,
  );
  return onComplete;
}

async function findMatches(find = "/rest/raven/1.0/", replace = "/rest/raven/2.0/") {
  await userEvent.type(screen.getByLabelText("Find"), find);
  await userEvent.type(screen.getByLabelText("Replace with"), replace);
  await userEvent.click(screen.getByRole("button", { name: "Find matches" }));
}

beforeEach(() => {
  scanSteps.mockReset();
  replaceSteps.mockReset();
  scanSteps.mockResolvedValue(twoMatches);
  replaceSteps.mockResolvedValue({ succeeded: ["QA-1", "QA-2"], failed: [] });
});

describe("BulkStepReplaceModal", () => {
  it("scans all three step fields by default and shows what each match becomes", async () => {
    renderModal();
    await findMatches();

    expect(scanSteps).toHaveBeenCalledWith("p1", ["QA-1", "QA-2", "QA-3"], {
      find: "/rest/raven/1.0/",
      replace: "/rest/raven/2.0/",
      fields: ["action", "data", "expected"],
      matchCase: false,
      wholeWord: false,
    });
    expect(
      await screen.findByText("2 of 2 matches selected, across 2 tests."),
    ).toBeTruthy();
    expect(screen.getByText("GET /rest/raven/2.0/api/test")).toBeTruthy();
    expect(screen.getByText("1 test has no match and will not change.")).toBeTruthy();
  });

  it("sends only the matches left selected", async () => {
    const onComplete = renderModal();
    await findMatches();
    await userEvent.click(
      await screen.findByRole("checkbox", { name: /QA-2 step 3 Expected/ }),
    );

    expect(screen.getByText("1 of 2 matches selected, across 1 test.")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Replace 1" }));

    expect(replaceSteps).toHaveBeenCalledWith(
      "p1",
      [{ testKey: "QA-1", xrayId: "11", field: "action" }],
      expect.objectContaining({ find: "/rest/raven/1.0/" }),
    );
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it("narrows the search to the chosen step fields", async () => {
    renderModal();
    await userEvent.click(screen.getByRole("checkbox", { name: "Action" }));
    await userEvent.click(screen.getByRole("checkbox", { name: "Data" }));
    await findMatches();

    expect(scanSteps).toHaveBeenCalledWith(
      "p1",
      ["QA-1", "QA-2", "QA-3"],
      expect.objectContaining({ fields: ["expected"] }),
    );
  });

  it("passes the case and whole-word options through", async () => {
    renderModal();
    await userEvent.click(screen.getByRole("checkbox", { name: "Match case" }));
    await userEvent.click(screen.getByRole("checkbox", { name: "Whole word" }));
    await findMatches();

    expect(scanSteps).toHaveBeenCalledWith(
      "p1",
      ["QA-1", "QA-2", "QA-3"],
      expect.objectContaining({ matchCase: true, wholeWord: true }),
    );
  });

  it("drops the preview when the query changes, so a stale list cannot be applied", async () => {
    renderModal();
    await findMatches();
    expect(await screen.findByText("2 of 2 matches selected, across 2 tests.")).toBeTruthy();

    await userEvent.type(screen.getByLabelText("Replace with"), "X");

    expect(
      screen.getByText("Find the matches first. Nothing is changed until you apply."),
    ).toBeTruthy();
    expect(screen.getByRole("button", { name: "Replace 0" })).toHaveProperty(
      "disabled",
      true,
    );
    expect(replaceSteps).not.toHaveBeenCalled();
  });

  it("refuses an empty search without calling the backend", async () => {
    renderModal();
    await userEvent.click(screen.getByRole("button", { name: "Find matches" }));

    expect(screen.getByText("Type the text to find.")).toBeTruthy();
    expect(scanSteps).not.toHaveBeenCalled();
  });

  it("says so when nothing matches", async () => {
    scanSteps.mockResolvedValue({ matches: [], skipped: ["QA-1", "QA-2", "QA-3"], failed: [] });
    renderModal();
    await findMatches("nothing");

    expect(
      await screen.findByText("No step text in the 3 selected tests matches."),
    ).toBeTruthy();
  });

  it("lists the tests a replace could not be applied to and stays open", async () => {
    replaceSteps.mockResolvedValue({
      succeeded: ["QA-1"],
      failed: [{ testKey: "QA-2", error: "step 21 is no longer in QA-2" }],
    });
    const onComplete = renderModal();
    await findMatches();
    await userEvent.click(await screen.findByRole("button", { name: "Replace 2" }));

    expect((await screen.findByText("QA-2")).closest("li")?.textContent).toBe(
      "QA-2: step 21 is no longer in QA-2",
    );
    expect(onComplete).not.toHaveBeenCalled();
  });

  it("reports a test whose steps could not be read during the scan", async () => {
    scanSteps.mockResolvedValue({
      matches: twoMatches.matches,
      skipped: [],
      failed: [{ testKey: "QA-3", error: "load steps: 404" }],
    });
    renderModal();
    await findMatches();

    expect((await screen.findByText("QA-3")).closest("li")?.textContent).toBe(
      "QA-3: load steps: 404",
    );
  });
});
