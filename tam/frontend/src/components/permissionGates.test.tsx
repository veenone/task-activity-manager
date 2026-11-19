import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { PermissionView, SprintDetail } from "../api";
import { AttachmentsSection } from "./AttachmentsSection";
import { CardMoveMenu } from "./CardMoveMenu";
import { SprintRow } from "./SprintRow";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetPermissions: vi.fn(), ListAttachments: vi.fn() };
});

const NO_SPRINTS = 'Your Jira account does not have the "Manage Sprints" permission in PLAT.';
const NO_ATTACH = 'Your Jira account does not have the "Create Attachments" permission in PLAT.';

function sprint(over: Partial<SprintDetail> = {}): SprintDetail {
  return {
    id: 12, boardId: 1, name: "Sprint 12", state: "active",
    startDate: "2026-08-29T09:00:00Z", endDate: "2026-09-12T09:00:00Z", goal: "",
    issues: [], total: 0, done: 0, points: 0, donePoints: 0,
    membershipCached: true, notSynced: 0, truncated: false,
    ...over,
  };
}

async function openSprintMenu(over: Partial<SprintDetail>, denied: string) {
  const actions = { onStart: vi.fn(), onComplete: vi.fn(), onEdit: vi.fn(), onDelete: vi.fn() };
  render(
    <SprintRow
      detail={sprint(over)} rowId="sprint:12" index={0} open={false} focused flashed={false}
      busy={false} denied={denied} onToggle={() => {}} onKeyDown={() => {}} actions={actions}
    />,
  );
  await userEvent.setup().click(screen.getByRole("button", { name: /^Actions on / }));
  return within(await screen.findByRole("menu"));
}

describe("sprint actions without Manage Sprints", () => {
  it("disables every action on a sprint Jira holds and says which permission is missing", async () => {
    const menu = await openSprintMenu({}, NO_SPRINTS);
    for (const name of ["Complete sprint…", "Edit sprint…", "Delete sprint…"]) {
      const item = menu.getByRole("menuitem", { name });
      expect(item).toBeDisabled();
      expect(item).toHaveAttribute("title", NO_SPRINTS);
    }
  });

  it("still lets a draft be edited and deleted, but not started", async () => {
    const menu = await openSprintMenu({ id: -1, state: "future", draft: true }, NO_SPRINTS);
    expect(menu.getByRole("menuitem", { name: "Start sprint…" })).toBeDisabled();
    expect(menu.getByRole("menuitem", { name: "Edit sprint…" })).toBeEnabled();
    expect(menu.getByRole("menuitem", { name: "Delete sprint…" })).toBeEnabled();
  });

  it("leaves the actions enabled when nothing is denied", async () => {
    const menu = await openSprintMenu({}, "");
    expect(menu.getByRole("menuitem", { name: "Complete sprint…" })).toBeEnabled();
    expect(menu.getByRole("menuitem", { name: "Edit sprint…" })).toBeEnabled();
  });
});

describe("card moves", () => {
  it("disables only the kind of move the account may not make", async () => {
    render(
      <CardMoveMenu
        issueKey="PLAT-1"
        columns={[{ name: "To Do", statusIds: ["1"] }, { name: "Done", statusIds: ["3"] }] as never}
        col={0}
        sprints={[{ id: 7, name: "Sprint 7", state: "future" }] as never}
        sprintId="5"
        disabled={false}
        columnDenied="No transitions for you."
        onColumn={() => {}}
        onSprint={() => {}}
      />,
    );
    await userEvent.setup().click(screen.getByRole("button", { name: "Actions on PLAT-1" }));
    const menu = within(await screen.findByRole("menu"));
    const toDone = menu.getByRole("menuitem", { name: "Move to Done" });
    expect(toDone).toBeDisabled();
    expect(toDone).toHaveAttribute("title", "No transitions for you.");
    expect(menu.getByRole("menuitem", { name: "Move to the backlog" })).toBeEnabled();
  });
});

describe("Attach files", () => {
  function renderSection(view: PermissionView) {
    vi.mocked(api.GetPermissions).mockResolvedValue(view);
    vi.mocked(api.ListAttachments).mockResolvedValue([]);
    render(
      <QueryClientProvider client={createQueryClient()}>
        <DialogProvider>
          <AttachmentsSection profileId="p1" issueKey="PLAT-412" open />
        </DialogProvider>
      </QueryClientProvider>,
    );
  }
  const names = { CREATE_ATTACHMENTS: "Create Attachments" };

  beforeEach(() => vi.clearAllMocks());

  it("is disabled, with the reason on screen, when Jira denied the permission", async () => {
    renderSection({ project: "PLAT", byProject: { PLAT: { CREATE_ATTACHMENTS: false } }, names });
    expect(await screen.findByText(NO_ATTACH)).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Attach files" })).toBeDisabled();
  });

  it("stays enabled when Jira reported nothing about the permission", async () => {
    renderSection({ project: "PLAT", byProject: {}, names });
    expect(await screen.findByText("Journaled now, uploaded on Commit.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Attach files" })).toBeEnabled();
  });
});
