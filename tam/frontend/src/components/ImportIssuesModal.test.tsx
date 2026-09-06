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
const mapping: api.ImportMapping = { type: "Issue Type", summary: "Summary", description: "", priority: "", labels: "", assignee: "", storyPoints: "Points", parentKey: "" };

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListProfiles).mockResolvedValue([
    { id: "p1", name: "Acme", jiraUrl: "demo", projectKey: "PLAT", backend: "jira", createdAt: "" },
  ]);
  vi.mocked(api.GetSettings).mockResolvedValue({ defaultProfileId: "p1", theme: "light" });
  vi.mocked(api.PreviewImport).mockResolvedValue({ headers: ["Issue Type", "Summary", "Points"], rowCount: 2 });
  vi.mocked(api.AutoMapImport).mockResolvedValue(mapping);
  vi.mocked(api.SaveImportTemplate).mockResolvedValue("C:/tam-import-template.csv");
});

async function pickFile(user: ReturnType<typeof userEvent.setup>) {
  const input = await screen.findByLabelText("File");
  await user.upload(input, new File([csv], "backlog.csv", { type: "text/csv" }));
  await waitFor(() => expect(api.PreviewImport).toHaveBeenCalled());
}

describe("ImportIssuesModal", () => {
  it("previews the file, pre-fills the mapping, and dry runs", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue({ rows: 2, created: [], errors: [{ row: 3, message: "Summary is empty." }] });
    renderModal();
    await pickFile(user);
    const [b64, isXlsx] = vi.mocked(api.PreviewImport).mock.calls[0];
    expect(atob(b64)).toBe(csv);
    expect(isXlsx).toBe(false);
    expect(await screen.findByText("3 columns, 2 rows")).toBeInTheDocument();
    expect(screen.getByLabelText("Summary")).toHaveValue("Summary");
    expect(screen.getByLabelText("Story points")).toHaveValue("Points");
    expect(screen.getByLabelText("Assignee")).toHaveValue("");
    await user.click(screen.getByRole("button", { name: "Dry run" }));
    await waitFor(() => expect(api.ImportIssues).toHaveBeenCalledWith("p1", b64, false, "backlog.csv", mapping, true));
    expect(await screen.findByText("Dry run: 1 row would become a draft, 1 would be skipped.")).toBeInTheDocument();
    expect(screen.getByText("Row 3")).toBeInTheDocument();
    expect(screen.getByText("Summary is empty.")).toBeInTheDocument();
  });

  it("imports with the edited mapping and reports the drafts", async () => {
    const user = userEvent.setup();
    vi.mocked(api.ImportIssues).mockResolvedValue({ rows: 2, created: ["TAM-NEW-1"], errors: [{ row: 3, message: "Summary is empty." }] });
    const { onImported } = renderModal();
    await pickFile(user);
    await user.selectOptions(await screen.findByLabelText("Priority"), "Points");
    await user.click(screen.getByRole("button", { name: "Import" }));
    await waitFor(() => expect(api.ImportIssues).toHaveBeenCalledWith("p1", expect.any(String), false, "backlog.csv", { ...mapping, priority: "Points" }, false));
    expect(await screen.findByText("Imported 1 draft; 1 row was skipped.")).toBeInTheDocument();
    expect(onImported).toHaveBeenCalledWith(["TAM-NEW-1"]);
    expect(screen.getByRole("button", { name: "Import" })).toBeDisabled();
    expect(screen.getByRole("button", { name: "Dry run" })).toBeDisabled();
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
    await waitFor(() => expect(api.SaveImportTemplate).toHaveBeenCalled());
    expect(await screen.findByText("Template saved to C:/tam-import-template.csv")).toBeInTheDocument();
  });

  it("refuses to import without a Summary column", async () => {
    const user = userEvent.setup();
    vi.mocked(api.AutoMapImport).mockResolvedValue({ ...mapping, summary: "" });
    renderModal();
    await pickFile(user);
    await user.click(await screen.findByRole("button", { name: "Import" }));
    expect(await screen.findByText("Map a Summary column first.")).toBeInTheDocument();
    expect(api.ImportIssues).not.toHaveBeenCalled();
  });
});
