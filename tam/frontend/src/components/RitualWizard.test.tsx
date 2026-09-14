import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import * as api from "../api";
import { RitualWizard } from "./RitualWizard";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetRitualDraft: vi.fn(), SaveRitualDraft: vi.fn() };
});

const ISSUES = [
  { key: "PLAT-1", summary: "Checkout", status: "To Do" },
  { key: "PLAT-3", summary: "Timeout", status: "Done" },
];

function renderWizard(ritualType = "review") {
  return render(
    <RitualWizard
      profileId="p1"
      boardId={1}
      sprintId={12}
      ritualType={ritualType}
      sprintIssues={ISSUES as never}
      onSaved={vi.fn()}
      onCancel={vi.fn()}
    />,
  );
}

beforeEach(() => {
  vi.mocked(api.GetRitualDraft).mockResolvedValue({
    profileId: "p1", boardId: 1, sprintId: 12, ritualType: "review",
    title: "Sprint 12 Review", remark: "", body: "", issuesJson: '[{"key":"PLAT-3","remark":""}]',
    confluencePageId: "", confluenceVersion: 0, status: "draft", updatedAt: "", publishedAt: "",
  } as never);
  vi.mocked(api.SaveRitualDraft).mockResolvedValue();
});

describe("RitualWizard", () => {
  it("opens on the stored selection rather than the whole sprint", async () => {
    renderWizard();
    const selected = await screen.findByRole("checkbox", { name: /PLAT-3/ });
    expect(selected).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /PLAT-1/ })).not.toBeChecked();
  });

  it("saves the author remark and a remark for each chosen issue", async () => {
    const user = userEvent.setup();
    renderWizard();

    await user.type(await screen.findByLabelText("Remark"), "short sprint");
    const row = screen.getByRole("group", { name: /PLAT-3/ });
    await user.type(within(row).getByLabelText("Issue remark"), "demoed");
    await user.click(screen.getByRole("button", { name: "Save draft" }));

    expect(api.SaveRitualDraft).toHaveBeenCalled();
    const draft = vi.mocked(api.SaveRitualDraft).mock.calls[0][1];
    expect(draft.remark).toBe("short sprint");
    expect(JSON.parse(draft.issuesJson)).toEqual([{ key: "PLAT-3", remark: "demoed" }]);
  });

  it("does not publish when saving", async () => {
    const user = userEvent.setup();
    renderWizard();
    await user.click(await screen.findByRole("button", { name: "Save draft" }));
    expect(screen.queryByRole("button", { name: /Publish/ })).not.toBeInTheDocument();
  });

  it("renders selected issues first, in the stored order, ahead of the unselected sprint issues", async () => {
    vi.mocked(api.GetRitualDraft).mockResolvedValue({
      profileId: "p1", boardId: 1, sprintId: 12, ritualType: "review",
      title: "Sprint 12 Review", remark: "", body: "",
      issuesJson: '[{"key":"PLAT-3","remark":""}]',
      confluencePageId: "", confluenceVersion: 0, status: "draft", updatedAt: "", publishedAt: "",
    } as never);
    renderWizard();
    const groups = await screen.findAllByRole("group");
    expect(groups[0]).toHaveAccessibleName(/PLAT-3/);
    expect(groups[1]).toHaveAccessibleName(/PLAT-1/);
  });

  it("still shows a chosen issue that has left the sprint, and lets it be removed", async () => {
    vi.mocked(api.GetRitualDraft).mockResolvedValue({
      profileId: "p1", boardId: 1, sprintId: 12, ritualType: "review",
      title: "Sprint 12 Review", remark: "", body: "",
      issuesJson: '[{"key":"PLAT-3","remark":""},{"key":"PLAT-99","remark":"moved out"}]',
      confluencePageId: "", confluenceVersion: 0, status: "draft", updatedAt: "", publishedAt: "",
    } as never);
    const user = userEvent.setup();
    renderWizard();

    const missing = await screen.findByRole("checkbox", { name: /PLAT-99/ });
    expect(missing).toBeChecked();
    expect(screen.getByText(/no longer in the sprint/i)).toBeInTheDocument();

    await user.click(missing);
    await user.click(screen.getByRole("button", { name: "Save draft" }));

    expect(api.SaveRitualDraft).toHaveBeenCalled();
    // .at(-1) rather than [0]: this suite does not clear mock call history
    // between tests, and an earlier test in this file also saves, so the
    // first call recorded on this mock is not necessarily this test's own.
    const calls = vi.mocked(api.SaveRitualDraft).mock.calls;
    const draft = calls[calls.length - 1][1];
    expect(JSON.parse(draft.issuesJson)).toEqual([{ key: "PLAT-3", remark: "" }]);
  });
});
