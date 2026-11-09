import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import * as api from "../wailsjs/go/main/App";
import { CasesPage } from "./CasesPage";
import { CyclesPage } from "./CyclesPage";
import { LinkMap } from "./LinkMap";
import { ProjectMap } from "./ProjectMap";
import { Statistics } from "./Statistics";
import { TracePage } from "./TracePage";

vi.mock("../wailsjs/go/main/App", () => ({
  ExportWorkbook: vi.fn(),
  Baselines: vi.fn(),
  CaseGrid: vi.fn(),
  Cycles: vi.fn(),
  GetCycle: vi.fn(),
}));

const issue = (key: string, statusCategory: string, links: object[] = []) => ({
  key,
  project: key.split("-")[0],
  status: statusCategory === "done" ? "Closed" : "Open",
  statusCategory,
  summary: `About ${key}`,
  matched: key !== "B-1",
  created: "2026-10-01T00:00:00Z",
  links,
});
const result = {
  total: 2,
  issues: [
    issue("A-1", "done", [{ key: "B-1", type: "Cloners", words: "clones", outward: true }]),
    issue("B-1", "new", [{ key: "A-1", type: "Cloners", words: "is cloned by", outward: false }]),
  ],
  sets: [{ keys: ["A-1", "B-1"], projects: ["A", "B"], outOfStep: true }],
} as never;
const grid = [{ id: 1, key: "ACME-1", projects: [{ key: "A", verdict: "affected", reason: "r", assessedAt: "" }] }];

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.ExportWorkbook).mockResolvedValue("");
  vi.mocked(api.Baselines).mockResolvedValue([]);
  vi.mocked(api.CaseGrid).mockResolvedValue(grid as never);
  vi.mocked(api.Cycles).mockResolvedValue([{ id: 2, name: "Week 45", takenAt: "2026-11-07T10:00:00Z", publishedAt: "" }]);
  vi.mocked(api.GetCycle).mockResolvedValue({
    id: 2,
    name: "Week 45",
    takenAt: "2026-11-07T10:00:00Z",
    publishedAt: "",
    cases: grid,
    changes: { since: "", new: ["ACME-1"], gone: [], verdicts: [] },
  } as never);
});

async function exported() {
  await userEvent.setup().click(await screen.findByRole("button", { name: "Export to Excel" }));
  const [name, sheets] = vi.mocked(api.ExportWorkbook).mock.calls[0];
  return { name, sheets: sheets.map((s) => s.name), first: sheets[0] };
}

it("exports Trace's linked sets", async () => {
  render(<TracePage result={result} focus={null} onFocus={vi.fn()} />);
  const got = await exported();
  expect(got).toMatchObject({ name: "pie-trace", sheets: ["Linked sets"] });
  expect(got.first.rows).toHaveLength(2);
});

it("exports the Project map's table and its sets", async () => {
  render(<ProjectMap profileId="p1" result={result} onOpen={vi.fn()} />);
  expect(await exported()).toMatchObject({ name: "pie-project-map", sheets: ["Project map", "Linked sets"] });
});

it("exports the Link map's grid and links", async () => {
  render(<LinkMap result={result} onOpen={vi.fn()} refresh={{ profileId: "p1", queryId: 0, runAt: "", onResult: vi.fn() }} />);
  const got = await exported();
  expect(got).toMatchObject({ name: "pie-link-map", sheets: ["Link grid", "Links"] });
});

it("exports the statistics", async () => {
  render(<Statistics result={result} productField="" />);
  expect(await exported()).toMatchObject({ name: "pie-statistics", sheets: ["Headline", "Per project", "Finished by status"] });
});

it("exports every case with its verdicts", async () => {
  render(<CasesPage profileId="p1" cases={[{ id: 1, key: "ACME-1" }] as never} result={null} onChanged={vi.fn()} />);
  const got = await exported();
  expect(api.CaseGrid).toHaveBeenCalledWith("p1");
  expect(got).toMatchObject({ name: "pie-cases", sheets: ["Cases"] });
  expect(got.first.rows[0][0]).toBe("ACME-1");
});

it("exports the open cycle's changes and cases", async () => {
  render(
    <DialogProvider>
      <CyclesPage profileId="p1" result={null} />
    </DialogProvider>,
  );
  await screen.findByRole("heading", { name: "Week 45" });
  expect(await exported()).toMatchObject({ name: "pie-cycle-Week 45", sheets: ["Changes", "Cases"] });
});
