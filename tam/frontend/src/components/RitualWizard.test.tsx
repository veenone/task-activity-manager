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
});
