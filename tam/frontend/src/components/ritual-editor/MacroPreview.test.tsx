import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import * as api from "../../api";
import { MACRO_CAVEAT } from "../../lib/ritualText";
import { RitualEditorContext } from "./context";
import { MacroPreview, macroJql } from "./MacroPreview";

vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, RitualMacroIssues: vi.fn() };
});

const macro = (jql: string) =>
  `<ac:structured-macro ac:name="jira"><ac:parameter ac:name="jqlQuery">${jql}</ac:parameter><ac:parameter ac:name="columns">key,summary</ac:parameter></ac:structured-macro>`;

function renderPreview(xml: string) {
  return render(
    <RitualEditorContext.Provider value={{ profileId: "p1" }}>
      <MacroPreview xml={xml} />
    </RitualEditorContext.Provider>,
  );
}

beforeEach(() => vi.clearAllMocks());

describe("MacroPreview", () => {
  it("reads the query out of the macro", () => {
    expect(macroJql(macro("sprint = 14 AND statusCategory != Done"))).toBe("sprint = 14 AND statusCategory != Done");
    expect(macroJql('<ac:structured-macro ac:name="jira"/>')).toBe("");
  });

  it("previews a form it knows from the cache, with the caveat", async () => {
    vi.mocked(api.RitualMacroIssues).mockResolvedValue({
      supported: true, jql: "sprint = 14 ORDER BY Rank",
      issues: [
        { key: "PLAT-1", summary: "Checkout", status: "Done", assignee: "R. Anand" },
        { key: "PLAT-2", summary: "Payments", status: "In Progress", assignee: "" },
      ] as api.Issue[],
    });
    renderPreview(macro("sprint = 14 ORDER BY Rank"));
    expect(await screen.findByText("PLAT-2")).toBeInTheDocument();
    expect(screen.getByText(MACRO_CAVEAT)).toBeInTheDocument();
    expect(api.RitualMacroIssues).toHaveBeenCalledWith("p1", "sprint = 14 ORDER BY Rank");
  });

  // Fix round 3, Minor: the one summary surface left rendering its raw
  // value, which showed a summary's own markup characters on a ritual page.
  it("shows a summary as plain text, the same as every other summary", async () => {
    vi.mocked(api.RitualMacroIssues).mockResolvedValue({
      supported: true, jql: "sprint = 14 ORDER BY Rank",
      issues: [
        { key: "PLAT-3", summary: "Fix {{login}} at [Figma|https://f]", status: "To Do", assignee: "" },
      ] as api.Issue[],
    });
    renderPreview(macro("sprint = 14 ORDER BY Rank"));
    expect(await screen.findByText("Fix login at Figma")).toBeInTheDocument();
  });

  it("shows any other query as text Confluence renders", async () => {
    vi.mocked(api.RitualMacroIssues).mockResolvedValue({ supported: false, jql: "project = PLAT", issues: [] });
    renderPreview(macro("project = PLAT"));
    expect(await screen.findByText("project = PLAT")).toBeInTheDocument();
    expect(screen.getByText("Rendered in Confluence")).toBeInTheDocument();
    expect(screen.queryByText(MACRO_CAVEAT)).toBeNull();
  });
});
