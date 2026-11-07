import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import type { cases, query } from "../wailsjs/go/models";
import { CasesPage } from "./CasesPage";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  CaseGrid: vi.fn(),
  GetCase: vi.fn(),
  Evidence: vi.fn(),
  ProjectGroups: vi.fn(),
}));

const list = [
  { id: 1, key: "A-1", state: "open", notes: "", openedAt: "", assessed: 1, affected: 1 },
  { id: 2, key: "B-1", state: "open", notes: "", openedAt: "", assessed: 2, affected: 1 },
] as unknown as cases.Case[];

const row = (c: cases.Case, projects: { key: string; verdict: string }[]) => ({ ...c, projects: projects.map((p) => ({ reason: "", assessedAt: "", ...p })) });

const result = {
  issues: [{ key: "A-1", project: "ALPHA", projectName: "Card Platform", summary: "Reader resets", matched: true }],
} as unknown as query.Result;

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.CaseGrid).mockResolvedValue([
    row(list[0], [{ key: "BETA", verdict: "affected" }]),
    row(list[1], [
      { key: "ALPHA", verdict: "not affected" },
      { key: "BETA", verdict: "affected" },
    ]),
  ] as never);
  vi.mocked(api.GetCase).mockResolvedValue(row(list[1], []) as never);
  vi.mocked(api.Evidence).mockResolvedValue([]);
});

function show() {
  render(
    <DialogProvider>
      <CasesPage profileId="p1" cases={list} result={result} onChanged={vi.fn()} />
    </DialogProvider>,
  );
  return userEvent.setup();
}

const cells = (r: HTMLElement) => [...(r as HTMLTableRowElement).cells].map((c) => c.textContent);

it("shows each case's verdict per project, and how many cases each project has affected", async () => {
  const user = show();
  await user.click(screen.getByRole("button", { name: "Grid" }));
  const grid = await screen.findByRole("table", { name: "Cases by project" });
  expect(api.CaseGrid).toHaveBeenCalledWith("p1");
  const rows = within(grid).getAllByRole("row");
  expect(cells(rows[0])).toEqual(["Case", "Card Platform (ALPHA)", "BETA"]);
  expect(cells(rows[1])).toEqual(["A-1", "-", "affected"]);
  expect(cells(rows[2])).toEqual(["B-1", "not affected", "affected"]);
  expect(cells(rows[3])).toEqual(["Affected", "0", "2"]);
});

it("opens a case from the grid in the list", async () => {
  const user = show();
  await user.click(screen.getByRole("button", { name: "Grid" }));
  const grid = await screen.findByRole("table", { name: "Cases by project" });
  await user.click(within(grid).getByRole("button", { name: "B-1" }));
  expect(await screen.findByRole("region", { name: "Case B-1" })).toBeVisible();
  expect(screen.getByRole("button", { name: "List" })).toHaveAttribute("aria-pressed", "true");
});
