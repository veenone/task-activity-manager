// The Backlog's selection bar (#605): Set release and Set components queue
// one pending edit per checked issue through SetIssuesCatalog.
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import { CatalogBulkBar } from "./CatalogBulkBar";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetVersions: vi.fn(), GetComponents: vi.fn(), GetPermissions: vi.fn(), SetIssuesCatalog: vi.fn() };
});

const version = (id: number, name: string, released = false): api.Version => ({
  id, project: "PLAT", name, description: "", startDate: "", releaseDate: "", released, archived: false, draft: false,
});

function renderBar(keys = ["PLAT-1", "PLAT-2"], onClear = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <CatalogBulkBar profileId="p1" keys={keys} onClear={onClear} />
      </DialogProvider>
    </QueryClientProvider>,
  );
  return onClear;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetVersions).mockResolvedValue([version(10100, "v1.2"), version(10101, "v1.1", true)]);
  vi.mocked(api.GetComponents).mockResolvedValue([
    { id: 10000, project: "PLAT", name: "Auth", description: "", leadName: "", leadDisplayName: "", assigneeType: "", draft: false },
  ]);
  vi.mocked(api.GetPermissions).mockResolvedValue({ project: "PLAT", byProject: {}, names: {} });
  vi.mocked(api.SetIssuesCatalog).mockResolvedValue({ changed: ["PLAT-1", "PLAT-2"], skipped: [] });
});

describe("CatalogBulkBar", () => {
  it("sets a release on every checked issue, replacing when asked", async () => {
    const user = userEvent.setup();
    const onClear = renderBar();
    expect(screen.getByRole("group", { name: "Checked issues" })).toHaveTextContent("2 issues selected");
    await user.click(screen.getByRole("button", { name: "Set release" }));
    const dialog = await screen.findByRole("dialog", { name: "Set release" });
    await user.click(within(dialog).getByRole("button", { name: "Releases: None" }));
    expect(await within(dialog).findByRole("checkbox", { name: "v1.1 (released)" })).toBeDisabled();
    await user.click(within(dialog).getByRole("checkbox", { name: "v1.2" }));
    await user.selectOptions(within(dialog).getByLabelText("How"), "replace");
    await user.click(within(dialog).getByRole("button", { name: "Set on 2 issues" }));
    await waitFor(() => expect(api.SetIssuesCatalog).toHaveBeenCalledWith("p1", ["PLAT-1", "PLAT-2"], "fixVersions", [10100], false));
    expect(onClear).toHaveBeenCalled();
  });

  it("adds components by default and names what it skipped", async () => {
    vi.mocked(api.SetIssuesCatalog).mockResolvedValue({ changed: ["PLAT-1"], skipped: ["OPS-3"] });
    const user = userEvent.setup();
    renderBar(["PLAT-1", "OPS-3"]);
    await user.click(screen.getByRole("button", { name: "Set components" }));
    const dialog = await screen.findByRole("dialog", { name: "Set components" });
    await user.click(within(dialog).getByRole("button", { name: "Components: None" }));
    await user.click(await within(dialog).findByRole("checkbox", { name: "Auth" }));
    await user.click(within(dialog).getByRole("button", { name: "Set on 2 issues" }));
    await waitFor(() => expect(api.SetIssuesCatalog).toHaveBeenCalledWith("p1", ["PLAT-1", "OPS-3"], "components", [10000], true));
    expect(await screen.findByRole("status")).toHaveTextContent("Skipped OPS-3: not in this profile's project.");
  });

  it("turns Set release off without Resolve Issues, and says why", async () => {
    vi.mocked(api.GetPermissions).mockResolvedValue({ project: "PLAT", byProject: { PLAT: { RESOLVE_ISSUES: false } }, names: { RESOLVE_ISSUES: "Resolve Issues" } });
    renderBar();
    const button = screen.getByRole("button", { name: "Set release" });
    await waitFor(() => expect(button).toBeDisabled());
    expect(button).toHaveAttribute("title", 'Your Jira account does not have the "Resolve Issues" permission in PLAT.');
    expect(screen.getByRole("button", { name: "Set components" })).toBeEnabled();
  });
});
