import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, ProfileProvider, createQueryClient, useProfile } from "@agile-suite/core";
import * as api from "../api";
import { profileBackend } from "../profileBackend";
import { ImportIssuesModal } from "./ImportIssuesModal";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListProfiles: vi.fn(),
    GetSettings: vi.fn(),
    SetTheme: vi.fn(),
    SetDefaultProfile: vi.fn(),
    PreviewImport: vi.fn(),
    AutoMapImport: vi.fn(),
    ImportIssues: vi.fn(),
    SaveImportTemplate: vi.fn(),
  };
});

vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ status: "idle" }) }));

function Loader() {
  const { reload } = useProfile<api.Profile, api.Settings>();
  React.useEffect(() => { void reload(); }, [reload]);
  return null;
}

function renderModal(onImported = vi.fn(), onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <ProfileProvider backend={profileBackend}>
          <Loader />
          <ImportIssuesModal onClose={onClose} onImported={onImported} />
        </ProfileProvider>
      </DialogProvider>
    </QueryClientProvider>,
  );
  return { onImported, onClose };
}

const csv = "Issue Type,Summary,Points\nStory,Apply promo,5\nTask,,\n";
const mapping: api.ImportMapping = { key: "", type: "Issue Type", summary: "Summary", description: "", priority: "", labels: "", assignee: "", storyPoints: "Points", parentKey: "", sprint: "" };

// result fills in the halves a case does not care about, so a test names only
// what it is actually asserting on.
function result(r: Partial<api.ImportResult>): api.ImportResult {
  return { rows: 0, created: [], updated: [], errors: [], sprintCellsIgnored: 0, ...r };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.PreviewImport).mockResolvedValue({ headers: ["Issue Type", "Summary", "Points"], rowCount: 2, sample: ["Story", "Apply promo", "5"] });
  vi.mocked(api.AutoMapImport).mockResolvedValue(mapping);
  vi.mocked(api.SaveImportTemplate).mockResolvedValue("C:/tam-import-template.xlsx");
});

async function pickFile(user: ReturnType<typeof userEvent.setup>) {
  const input = await screen.findByLabelText("File");
  await user.upload(input, new File([csv], "backlog.csv", { type: "text/csv" }));
  await waitFor(() => expect(api.PreviewImport).toHaveBeenCalled());
}

describe("ImportIssuesModal", () => {
  it("previews the file, pre-fills the mapping, and runs the automatic preflight", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue(result({ rows: 2, errors: [{ row: 3, message: "Summary is empty." }] }));
    renderModal();
    await screen.findByText("Import issues (CSV or XLSX)");
    await pickFile(user);
    const [b64, isXlsx] = vi.mocked(api.PreviewImport).mock.calls[0];
    expect(atob(b64)).toBe(csv);
    expect(isXlsx).toBe(false);
    expect(await screen.findByText("backlog.csv (2 rows)")).toBeInTheDocument();
    expect(screen.getByLabelText("Summary *")).toHaveValue("Summary");
    expect(screen.getByLabelText("Story points")).toHaveValue("Points");
    expect(screen.getByLabelText("Assignee")).toHaveValue("");
    expect(within(screen.getByLabelText("Summary *")).getByText("Summary (e.g. Apply promo)")).toBeInTheDocument();
    await waitFor(() => expect(api.ImportIssues).toHaveBeenCalledWith("p1", b64, false, "backlog.csv", mapping, true));
    expect(await screen.findByText("1 valid row, 1 skipped.")).toBeInTheDocument();
    expect(screen.getByText("row 3: Summary is empty.")).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Import" })).toBeEnabled();
  });

  it("imports with the edited mapping and reports the drafts", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue(result({ rows: 2, created: ["TAM-NEW-1"], errors: [{ row: 3, message: "Summary is empty." }] }));
    const { onImported } = renderModal();
    await pickFile(user);
    await waitFor(() => expect(screen.getByRole("button", { name: "Import" })).toBeEnabled());
    await user.selectOptions(screen.getByLabelText("Priority"), "Points");
    await user.click(screen.getByRole("button", { name: "Import" }));
    await waitFor(() => expect(api.ImportIssues).toHaveBeenCalledWith("p1", expect.any(String), false, "backlog.csv", { ...mapping, priority: "Points" }, false));
    expect(await screen.findByText("✓ Imported 1 draft to create as pending changes (1 skipped). Commit them from the Pending changes dialog.")).toBeInTheDocument();
    expect(onImported).toHaveBeenCalledWith(["TAM-NEW-1"]);
    expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument();
  });

  it("shows a parse error and offers the template", async () => {
    const user = userEvent.setup();
    vi.mocked(api.PreviewImport).mockRejectedValue(new Error("open xlsx: zip: not a valid zip file"));
    renderModal();
    const input = await screen.findByLabelText("File");
    await user.upload(input, new File(["junk"], "bad.xlsx", { type: "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" }));
    expect(await screen.findByText(/not a valid zip file/)).toBeInTheDocument();
    expect(vi.mocked(api.PreviewImport).mock.calls[0][1]).toBe(true);
    await user.click(screen.getByRole("button", { name: "Download template" }));
    await waitFor(() => expect(api.SaveImportTemplate).toHaveBeenCalledWith("p1"));
    expect(await screen.findByText("C:/tam-import-template.xlsx")).toBeInTheDocument();
  });

  it("shows the preflight message and disables Import when no Summary column is mapped", async () => {
    const user = userEvent.setup();
    vi.mocked(api.AutoMapImport).mockResolvedValue({ ...mapping, summary: "" });
    renderModal();
    await pickFile(user);
    expect(await screen.findByText("Map a Summary column first, or an Issue key column to update issues that already exist.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Import" })).toBeDisabled();
    expect(api.ImportIssues).not.toHaveBeenCalled();
  });

  it("imports a key column as updates, with no Summary column mapped", async () => {
    const user = userEvent.setup();
    const keyed: api.ImportMapping = { ...mapping, summary: "", key: "Issue Type" };
    vi.mocked(api.AutoMapImport).mockResolvedValue(keyed);
    vi.mocked(api.ImportIssues).mockResolvedValue(result({ rows: 1, updated: ["PLAT-412"] }));
    const { onImported } = renderModal();
    await pickFile(user);
    await waitFor(() => expect(screen.getByRole("button", { name: "Import" })).toBeEnabled());
    await user.click(screen.getByRole("button", { name: "Import" }));
    expect(await screen.findByText("✓ Imported 1 issue to update as pending changes. Commit them from the Pending changes dialog.")).toBeInTheDocument();
    // Nothing was created, so there are no new keys to hand back, but the
    // dialog still finishes rather than offering Import again.
    expect(onImported).toHaveBeenCalledWith([]);
    expect(screen.getByRole("button", { name: "Done" })).toBeInTheDocument();
  });

  it("notes ignored Sprint cells in the preflight when the count is above zero, and stays quiet when it is zero", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue(result({ rows: 1, updated: ["PLAT-412"], sprintCellsIgnored: 2 }));
    renderModal();
    await pickFile(user);
    expect(await screen.findByText("2 rows carry a Sprint value; import does not change an issue's sprint.")).toBeInTheDocument();
    expect(await screen.findByRole("button", { name: "Import" })).toBeEnabled();

    vi.mocked(api.ImportIssues).mockResolvedValue(result({ rows: 1, updated: ["PLAT-412"], sprintCellsIgnored: 0 }));
    await user.click(screen.getByRole("button", { name: "Validate" }));
    await waitFor(() => expect(screen.queryByText(/carry a Sprint value/)).not.toBeInTheDocument());
  });

  it("notes ignored Sprint cells in the result summary after import", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue(result({ rows: 1, updated: ["PLAT-412"], sprintCellsIgnored: 1 }));
    renderModal();
    await pickFile(user);
    await waitFor(() => expect(screen.getByRole("button", { name: "Import" })).toBeEnabled());
    await user.click(screen.getByRole("button", { name: "Import" }));
    expect(await screen.findByText("✓ Imported 1 issue to update as pending changes. Commit them from the Pending changes dialog.")).toBeInTheDocument();
    expect(screen.getByText("1 row carries a Sprint value; import does not change an issue's sprint.")).toBeInTheDocument();
  });

  it("renders a Sprint mapping select and lets it be remapped like any other field", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue(result({ rows: 2, created: ["TAM-NEW-1"] }));
    const { onImported } = renderModal();
    await pickFile(user);
    await waitFor(() => expect(screen.getByRole("button", { name: "Import" })).toBeEnabled());
    expect(screen.getByLabelText("Sprint")).toHaveValue("");
    await user.selectOptions(screen.getByLabelText("Sprint"), "Points");
    await user.click(screen.getByRole("button", { name: "Import" }));
    await waitFor(() => expect(api.ImportIssues).toHaveBeenCalledWith("p1", expect.any(String), false, "backlog.csv", { ...mapping, sprint: "Points" }, false));
    expect(onImported).toHaveBeenCalledWith(["TAM-NEW-1"]);
  });

  it("shows the zero-drafts outcome when every row is skipped on import", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues)
      .mockResolvedValueOnce(result({ rows: 1 }))
      .mockResolvedValueOnce(result({ rows: 1, errors: [{ row: 2, message: "Already a draft (TAM-NEW-1); commit or discard it first." }] }));
    renderModal();
    await pickFile(user);
    await waitFor(() => expect(screen.getByRole("button", { name: "Import" })).toBeEnabled());
    await user.click(screen.getByRole("button", { name: "Import" }));
    expect(await screen.findByText("Nothing was imported.")).toBeInTheDocument();
    expect(screen.getByText("row 2: Already a draft (TAM-NEW-1); commit or discard it first.")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Import" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Validate" })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Cancel" })).toBeInTheDocument();
  });
});
