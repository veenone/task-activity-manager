import { describe, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import type { query } from "../wailsjs/go/models";
import { Pager, usePaged } from "./Pager";
import { ProjectMap } from "./ProjectMap";
import { ResultsTable } from "./ResultsTable";
import { Statistics } from "./Statistics";
import { TracePage } from "./TracePage";

// The map reads the profile's baselines; none here.
vi.mock("../wailsjs/go/main/App", () => ({ Baselines: vi.fn(async () => []) }));

function Numbers({ count, start }: { count: number; start?: number }) {
  const { rows, pager } = usePaged(
    Array.from({ length: count }, (_, i) => i + 1),
    start,
  );
  return (
    <>
      <ul aria-label="Numbers">
        {rows.map((n) => (
          <li key={n}>{n}</li>
        ))}
      </ul>
      <Pager label="Numbers" {...pager} />
    </>
  );
}

const shown = () => within(screen.getByRole("list", { name: "Numbers" })).getAllByRole("listitem").map((li) => li.textContent);
const range = () => within(screen.getByRole("navigation", { name: "Numbers pages" })).getByText(/ of 60$/).textContent;

describe("Pager", () => {
  it("moves between pages and to a typed page", async () => {
    const user = userEvent.setup();
    render(<Numbers count={60} />);
    expect(range()).toBe("1-25 of 60");
    await user.click(screen.getByRole("button", { name: "Next page" }));
    expect(shown()[0]).toBe("26");
    await user.click(screen.getByRole("button", { name: "Last page" }));
    expect(range()).toBe("51-60 of 60");
    expect(screen.getByRole("button", { name: "Next page" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "First page" }));
    expect(shown()[0]).toBe("1");

    const page = screen.getByRole("textbox", { name: "Page" });
    await user.clear(page);
    await user.type(page, "2{Enter}");
    expect(range()).toBe("26-50 of 60");
    await user.clear(page);
    await user.type(page, "9{Enter}");
    expect(range()).toBe("51-60 of 60");
  });

  it("goes back to the first page when the page size changes", async () => {
    const user = userEvent.setup();
    render(<Numbers count={60} />);
    await user.click(screen.getByRole("button", { name: "Last page" }));
    await user.selectOptions(screen.getByLabelText("Rows per page"), "50");
    expect(range()).toBe("1-50 of 60");
  });

  it("falls back to the last page that still exists", async () => {
    const user = userEvent.setup();
    const { rerender } = render(<Numbers count={60} />);
    await user.click(screen.getByRole("button", { name: "Last page" }));
    rerender(<Numbers count={30} />);
    expect(shown()).toEqual(["26", "27", "28", "29", "30"]);
    rerender(<Numbers count={10} />);
    expect(shown()).toHaveLength(10);
    expect(screen.queryByRole("navigation")).not.toBeInTheDocument();
  });

  it("can open on the page that holds a given row", () => {
    render(<Numbers count={60} start={30} />);
    expect(shown()[0]).toBe("26");
  });
});

function issue(key: string, more: Partial<query.Issue> = {}) {
  const project = key.split("-")[0];
  return {
    key,
    project,
    projectName: `${project} name`,
    type: "Bug",
    summary: `Summary of ${key}`,
    status: "Open",
    statusCategory: "new",
    priority: "High",
    created: "2026-10-01T00:00:00.000+0000",
    updated: "2026-10-02T00:00:00.000+0000",
    resolved: "",
    bases: [],
    matched: true,
    links: [],
    ...more,
  } as unknown as query.Issue;
}

// Thirty sets, each joining A-n to B-n.
const many = {
  total: 60,
  issues: Array.from({ length: 30 }, (_, i) => [issue(`A-${i + 1}`), issue(`B-${i + 1}`)]).flat(),
  sets: Array.from({ length: 30 }, (_, i) => ({ keys: [`A-${i + 1}`, `B-${i + 1}`], projects: ["A", "B"], outOfStep: false })),
} as unknown as query.Result;

describe("pages and project names in the views", () => {
  it("pages the results and names each project", () => {
    render(<ResultsTable issues={many.issues} caseKeys={new Set()} onOpenCases={async () => true} />);
    expect(screen.getAllByRole("cell", { name: "A name (A)" })).toHaveLength(13);
    expect(screen.getByText("1-25 of 60")).toBeVisible();
  });

  it("keeps the key alone for a run stored before names were read", () => {
    render(<ResultsTable issues={[issue("A-1", { projectName: "" })]} caseKeys={new Set()} onOpenCases={async () => true} />);
    expect(screen.getByRole("cell", { name: "A" })).toBeVisible();
  });

  it("opens Trace on the page holding the set the map chose", () => {
    render(<TracePage result={many} focus="B-28" onFocus={() => {}} />);
    const list = screen.getByRole("list", { name: "Linked sets" });
    expect(within(list).getByRole("button", { current: true })).toHaveTextContent("A-28");
    expect(screen.getByText("26-30 of 30")).toBeVisible();
    expect(screen.getByRole("option", { name: "A name (A)" })).toBeInTheDocument();
    expect(screen.getByRole("group", { name: "Project A name (A)" })).toBeVisible();
  });

  it("turns Trace's list to the open set when a filter changes", async () => {
    const user = userEvent.setup();
    render(<TracePage result={many} focus={null} onFocus={() => {}} />);
    await user.click(screen.getByRole("button", { name: "Next page" }));
    expect(screen.getByText("26-30 of 30")).toBeVisible();
    await user.selectOptions(screen.getByLabelText("Project"), "B");
    expect(screen.getByText("1-25 of 30")).toBeVisible();
    const list = screen.getByRole("list", { name: "Linked sets" });
    expect(within(list).getByRole("button", { current: true })).toHaveTextContent("A-1");
  });

  it("opens the set's project groups on the page holding the chosen key", () => {
    const keys = Array.from({ length: 6 }, (_, i) => `P${i + 1}-1`);
    const wide = {
      total: 6,
      issues: keys.map((k) => issue(k)),
      sets: [{ keys, projects: keys.map((k) => k.split("-")[0]), outOfStep: false }],
    } as unknown as query.Result;
    render(<TracePage result={wide} focus="P6-1" onFocus={() => {}} />);
    expect(screen.getByRole("group", { name: "Project P6 name (P6)" })).toBeVisible();
    expect(screen.queryByRole("group", { name: "Project P1 name (P1)" })).not.toBeInTheDocument();
  });

  it("pages the Statistics per-project table", async () => {
    const user = userEvent.setup();
    const projects = { total: 30, issues: Array.from({ length: 30 }, (_, i) => issue(`P${i + 1}-1`)), sets: [] } as unknown as query.Result;
    render(<Statistics result={projects} productField="" />);
    const nav = screen.getByRole("navigation", { name: "Per project pages" });
    expect(within(nav).getByText("1-25 of 30")).toBeVisible();
    await user.click(within(nav).getByRole("button", { name: "Next page" }));
    expect(within(screen.getByRole("table", { name: "Per project" })).getAllByRole("row")).toHaveLength(6);
  });

  it("pages the map's rows and names its columns", async () => {
    render(<ProjectMap profileId="p1" result={many} onOpen={() => {}} />);
    await userEvent.setup().click(screen.getByRole("button", { name: "Table" }));
    expect(screen.getByRole("columnheader", { name: "B name (B)" })).toBeVisible();
    expect(within(screen.getByRole("table", { name: "Project map" })).getAllByRole("row")).toHaveLength(26);
  });

  it("names projects in the Statistics filter and table", () => {
    render(<Statistics result={many} productField="" />);
    expect(screen.getByRole("option", { name: "A name (A)" })).toBeInTheDocument();
    expect(within(screen.getByRole("table", { name: "Per project" })).getByRole("cell", { name: "B name (B)" })).toBeVisible();
  });
});
