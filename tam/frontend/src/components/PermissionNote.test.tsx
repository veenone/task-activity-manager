import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import { PermissionNote } from "./PermissionNote";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetPermissions: vi.fn() };
});

function renderNote() {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <PermissionNote profileId="p1" />
    </QueryClientProvider>,
  );
}

describe("PermissionNote", () => {
  beforeEach(() => vi.mocked(api.GetPermissions).mockReset());

  it("says once, in text, which permissions the token lacks", async () => {
    vi.mocked(api.GetPermissions).mockResolvedValue({
      project: "PLAT",
      byProject: { PLAT: { CREATE_ISSUES: false, EDIT_ISSUES: true, MANAGE_SPRINTS_PERMISSION: false } },
      names: { CREATE_ISSUES: "Create Issues", MANAGE_SPRINTS_PERMISSION: "Manage Sprints" },
    });
    renderNote();
    expect(await screen.findByRole("note")).toHaveTextContent(
      'Your Jira account is missing permissions, so some actions here are turned off: "Create Issues" in PLAT, "Manage Sprints" in PLAT.',
    );
  });

  it("says nothing when every permission is held", async () => {
    vi.mocked(api.GetPermissions).mockResolvedValue({ project: "PLAT", byProject: { PLAT: { CREATE_ISSUES: true } }, names: {} });
    const { container } = renderNote();
    await vi.waitFor(() => expect(api.GetPermissions).toHaveBeenCalled());
    expect(container.textContent).toBe("");
  });
});
