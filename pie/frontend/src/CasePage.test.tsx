import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import type { cases, query } from "../wailsjs/go/models";
import { CasesPage } from "./CasesPage";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  GetCase: vi.fn(),
  SetCaseNotes: vi.fn(),
  AddCaseProjects: vi.fn(),
  RemoveCaseProject: vi.fn(),
  SetVerdict: vi.fn(),
  JiraProjects: vi.fn(),
  ProjectGroups: vi.fn(),
  SaveProjectGroup: vi.fn(),
  DeleteProjectGroup: vi.fn(),
}));

const list = [
  { id: 1, key: "A-1", state: "open", notes: "", openedAt: "2026-11-06T10:00:00Z", assessed: 1, affected: 1 },
  { id: 2, key: "Z-9", state: "open", notes: "", openedAt: "2026-11-05T10:00:00Z", assessed: 0, affected: 0 },
] as unknown as cases.Case[];

const result = {
  issues: [{ key: "A-1", project: "A", projectName: "Alpha", summary: "Reader resets", matched: true }],
} as unknown as query.Result;

function detail(projects: Partial<cases.Project>[] = [{ key: "ALPHA", verdict: "affected", reason: "Same chip", assessedAt: "2026-11-06T11:00:00Z" }]) {
  return { ...list[0], notes: "Seen twice.", projects: projects.map((p) => ({ verdict: "not assessed", reason: "", assessedAt: "", ...p })) } as never;
}

function show(onChanged = vi.fn()) {
  render(
    <DialogProvider>
      <CasesPage profileId="p1" cases={list} result={result} onChanged={onChanged} />
    </DialogProvider>,
  );
  return userEvent.setup();
}

async function openFirst(user: ReturnType<typeof userEvent.setup>) {
  await user.click(screen.getByRole("button", { name: /^A-1/ }));
  return screen.findByRole("region", { name: "Case A-1" });
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.GetCase).mockResolvedValue(detail());
  vi.mocked(api.ProjectGroups).mockResolvedValue([]);
  vi.mocked(api.JiraProjects).mockResolvedValue([
    { key: "ALPHA", name: "Card Platform" },
    { key: "CHARLIE", name: "Mobile Wallet" },
    { key: "DELTA", name: "Personalisation" },
  ] as never);
});

describe("Cases page", () => {
  it("lists cases with their summary from the run and their verdict counts", () => {
    show();
    const a1 = screen.getByRole("button", { name: /^A-1/ });
    expect(a1).toHaveTextContent("Reader resets");
    expect(a1).toHaveTextContent("1 assessed, 1 affected");
    expect(screen.getByRole("button", { name: /^Z-9/ })).toHaveTextContent("0 assessed, 0 affected");
  });

  it("opens a case and saves its notes", async () => {
    const user = show();
    const page = await openFirst(user);
    expect(api.GetCase).toHaveBeenCalledWith("p1", 1);
    const notes = within(page).getByLabelText("Notes");
    expect(notes).toHaveValue("Seen twice.");
    await user.type(notes, " Ask the reader team.");
    await user.click(within(page).getByRole("button", { name: "Save notes" }));
    expect(api.SetCaseNotes).toHaveBeenCalledWith("p1", 1, "Seen twice. Ask the reader team.");
  });

  it("records a verdict and its reason for a project", async () => {
    const onChanged = vi.fn();
    vi.mocked(api.GetCase).mockResolvedValue(detail([{ key: "ALPHA" }]));
    vi.mocked(api.SetVerdict).mockResolvedValue(detail([{ key: "ALPHA", verdict: "not affected", reason: "Other chip" }]));
    const user = show(onChanged);
    const page = await openFirst(user);
    await user.type(within(page).getByLabelText("Reason for ALPHA"), "Other chip");
    await user.selectOptions(within(page).getByLabelText("Verdict for ALPHA"), "not affected");
    expect(api.SetVerdict).toHaveBeenCalledWith("p1", 1, "ALPHA", "not affected", "Other chip");
    expect(within(page).getByLabelText("Verdict for ALPHA")).toHaveValue("not affected");
    expect(onChanged).toHaveBeenCalled();
  });

  it("keeps a reason being typed when another project's verdict changes", async () => {
    vi.mocked(api.GetCase).mockResolvedValue(detail([{ key: "ALPHA" }, { key: "BETA" }]));
    vi.mocked(api.SetVerdict).mockResolvedValue(detail([{ key: "ALPHA" }, { key: "BETA", verdict: "affected" }]));
    const user = show();
    const page = await openFirst(user);
    await user.type(within(page).getByLabelText("Reason for ALPHA"), "Half written");
    await user.selectOptions(within(page).getByLabelText("Verdict for BETA"), "affected");
    expect(api.SetVerdict).toHaveBeenLastCalledWith("p1", 1, "BETA", "affected", "");
    expect(within(page).getByLabelText("Reason for ALPHA")).toHaveValue("Half written");
  });

  it("removes a project only after confirming", async () => {
    vi.mocked(api.RemoveCaseProject).mockResolvedValue(detail([]));
    const user = show();
    const page = await openFirst(user);
    await user.click(within(page).getByRole("button", { name: "Remove ALPHA" }));
    expect(api.RemoveCaseProject).not.toHaveBeenCalled();
    await user.click(within(await screen.findByRole("alertdialog")).getByRole("button", { name: "Remove" }));
    expect(api.RemoveCaseProject).toHaveBeenCalledWith("p1", 1, "ALPHA");
  });
});

describe("project picker", () => {
  it("searches Jira's projects by key or name and adds the chosen ones", async () => {
    vi.mocked(api.AddCaseProjects).mockResolvedValue(detail());
    const user = show();
    const page = await openFirst(user);
    await user.click(within(page).getByRole("button", { name: "Add projects" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Search projects"), "wal");
    expect(within(dialog).getAllByRole("checkbox").map((c) => c.getAttribute("aria-label"))).toEqual(["Mobile Wallet (CHARLIE)"]);
    await user.click(within(dialog).getByRole("checkbox", { name: "Mobile Wallet (CHARLIE)" }));
    await user.click(within(dialog).getByRole("button", { name: "Add 1 project" }));
    expect(api.AddCaseProjects).toHaveBeenCalledWith("p1", 1, ["CHARLIE"]);
    expect(screen.queryByRole("dialog")).not.toBeInTheDocument();
  });

  it("adds a saved group and saves the choice as a new group", async () => {
    vi.mocked(api.ProjectGroups).mockResolvedValue([{ id: 5, name: "Readers", projects: ["ALPHA", "DELTA"] }] as never);
    vi.mocked(api.SaveProjectGroup).mockResolvedValue({ id: 6, name: "Two", projects: ["ALPHA", "DELTA"] } as never);
    const user = show();
    const page = await openFirst(user);
    await user.click(within(page).getByRole("button", { name: "Add projects" }));
    const dialog = await screen.findByRole("dialog");
    await user.click(await within(dialog).findByRole("button", { name: "Choose Readers (2 projects)" }));
    expect(within(dialog).getByRole("button", { name: "Add 2 projects" })).toBeVisible();

    await user.click(within(dialog).getByRole("button", { name: "Save as group..." }));
    const ask = (await screen.findAllByRole("dialog")).at(-1)!;
    await user.type(within(ask).getByRole("textbox"), "Two");
    await user.click(within(ask).getByRole("button", { name: "Save" }));
    expect(api.SaveProjectGroup).toHaveBeenCalledWith("p1", "Two", ["ALPHA", "DELTA"]);
  });
});
