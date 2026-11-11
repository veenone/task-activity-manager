import { useState } from "react";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../wailsjs/go/main/App";
import { IssueGraphView } from "./IssueGraphView";
import { type Notice, NoticeContext, StatusBar } from "./StatusBar";

vi.mock("../wailsjs/go/main/App", () => ({ ExpandIssue: vi.fn() }));

// jsdom has no canvas, so G6 is replaced by a list of what it was given.
vi.mock("./G6Canvas", () => ({
  default: ({
    nodes,
    edges,
    layout,
    onSelect,
  }: {
    nodes: { id: string; dashed?: boolean; badge?: string }[];
    edges: { id: string }[];
    layout?: { focusNode?: string };
    onSelect: (id: string) => void;
  }) => (
    <div>
      <p>{`Centred on ${layout?.focusNode}`}</p>
      <ul aria-label="Nodes">
        {nodes.map((n) => (
          <li key={n.id}>
            <button type="button" onClick={() => onSelect(n.id)}>
              {`${n.id}${n.dashed ? " outside" : ""}${n.badge ? ` ${n.badge}` : ""}`}
            </button>
          </li>
        ))}
      </ul>
      <ul aria-label="Edges">
        {edges.map((e) => (
          <li key={e.id}>{e.id}</li>
        ))}
      </ul>
    </div>
  ),
}));

const link = (key: string, type: string, outward: boolean, words: string) => ({ key, type, outward, words, symmetric: false });
const issue = (key: string, links: object[] = [], more: object = {}) => ({
  key,
  project: key.split("-")[0],
  summary: `About ${key}`,
  type: "Bug",
  status: "Open",
  statusCategory: "new",
  priority: "High",
  created: "2026-10-01T00:00:00Z",
  updated: "2026-11-01T00:00:00Z",
  resolved: "",
  links,
  subtasks: [],
  parent: "",
  ...more,
});

// A-1 blocks B-1 and clones Z-9, which is outside the run.
const result = {
  issues: [
    issue("A-1", [link("B-1", "Blocks", true, "blocks"), link("Z-9", "Cloners", true, "clones")]),
    issue("B-1", [link("A-1", "Blocks", false, "is blocked by")]),
  ],
  sets: [],
};

function show() {
  const onOpen = vi.fn();
  function Wired() {
    const [notice, setNotice] = useState<Notice | null>(null);
    return (
      <NoticeContext.Provider value={setNotice}>
        <IssueGraphView profileId="p1" result={result as never} jiraUrl="https://jira.example.com" label={(p) => p} onOpen={onOpen} />
        <StatusBar progress={null} notice={notice} onDismiss={() => setNotice(null)} theme="light" />
      </NoticeContext.Provider>
    );
  }
  render(<Wired />);
  return { user: userEvent.setup(), onOpen };
}

const nodes = () => within(screen.getByRole("list", { name: "Nodes" })).getAllByRole("button").map((b) => b.textContent);
const edges = () => within(screen.getByRole("list", { name: "Edges" })).queryAllByRole("listitem").map((l) => l.textContent);

beforeEach(() => vi.resetAllMocks());

it("starts at the run's busiest issue with its neighbours from the run, one edge per link", async () => {
  show();
  await screen.findByRole("list", { name: "Nodes" });
  expect(nodes()).toEqual(["A-1 +1", "B-1"]);
  expect(edges()).toEqual(["A-1>B-1:Blocks"]);
  const details = screen.getByRole("complementary", { name: "Issue A-1" });
  expect(details).toHaveTextContent("About A-1");
  expect(details).toHaveTextContent("blocksB-1");
  expect(api.ExpandIssue).not.toHaveBeenCalled();
});

it("expands into issues outside the run through Jira, and walks back and forward", async () => {
  vi.mocked(api.ExpandIssue).mockResolvedValue({
    issues: [{ issue: result.issues[0], fetchedAt: "" }, { issue: issue("Z-9", [link("A-1", "Cloners", false, "is cloned by")]), fetchedAt: "2026-11-10T09:00:00Z" }],
    offline: false,
    missing: [],
  } as never);
  const { user } = show();
  await user.click(await screen.findByRole("button", { name: "Expand links (1)" }));
  expect(api.ExpandIssue).toHaveBeenCalledWith("p1", "A-1");
  expect(nodes()).toEqual(["A-1", "B-1", "Z-9 outside"]);
  expect(edges()).toContain("A-1>Z-9:Cloners");

  await user.click(screen.getByRole("button", { name: "Z-9 outside" }));
  expect(screen.getByRole("complementary", { name: "Issue Z-9" })).toHaveTextContent("In the runNo");
  await user.click(screen.getByRole("button", { name: "‹ Back" }));
  expect(screen.getByRole("complementary", { name: "Issue A-1" })).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Forward ›" }));
  expect(screen.getByRole("complementary", { name: "Issue Z-9" })).toBeVisible();
});

it("says in the status bar when it shows stored issues, until Jira answers again", async () => {
  const z = { issue: issue("Z-9", [link("A-1", "Cloners", false, "is cloned by")], { links: [link("A-1", "Cloners", false, "is cloned by"), link("Q-1", "Relates", true, "relates to")] }), fetchedAt: "2026-11-01T09:00:00Z" };
  vi.mocked(api.ExpandIssue)
    .mockResolvedValueOnce({ issues: [{ issue: result.issues[0], fetchedAt: "" }, z], offline: true, missing: [] } as never)
    .mockResolvedValueOnce({ issues: [z, { issue: issue("Q-1"), fetchedAt: "2026-11-10T09:00:00Z" }], offline: false, missing: [] } as never);
  const { user } = show();
  await user.click(await screen.findByRole("button", { name: "Expand links (1)" }));
  expect(screen.getByRole("contentinfo")).toHaveTextContent("Offline: Jira cannot be reached");
  await user.click(within(screen.getByRole("list", { name: "Nodes" })).getByRole("button", { name: /^Z-9/ }));
  expect(screen.getByRole("complementary", { name: "Issue Z-9" })).toHaveTextContent("Fetched2026-11-01 09:00");
  await user.click(screen.getByRole("button", { name: "Expand links (1)" }));
  expect(screen.getByRole("contentinfo")).not.toHaveTextContent("Offline");
});

it("hides a relationship type, and finds an issue by key, fetching one not loaded", async () => {
  vi.mocked(api.ExpandIssue).mockResolvedValue({ issues: [{ issue: issue("CC-5"), fetchedAt: "" }], offline: false, missing: [] } as never);
  const { user } = show();
  await screen.findByRole("list", { name: "Nodes" });
  await user.click(screen.getByRole("checkbox", { name: /Blocks/ }));
  expect(edges()).toEqual([]);
  await user.type(screen.getByRole("searchbox", { name: "Find an issue" }), "cc-5{Enter}");
  expect(api.ExpandIssue).toHaveBeenCalledWith("p1", "CC-5");
  expect(await screen.findByRole("complementary", { name: "Issue CC-5" })).toBeVisible();
});

it("opens an issue of the run in Trace", async () => {
  const { user, onOpen } = show();
  await user.click(await screen.findByRole("button", { name: "Open in Trace" }));
  expect(onOpen).toHaveBeenCalledWith("A-1");
});

it("takes one expansion at a time, so a second cannot drop the first one's issues", async () => {
  let finish!: (v: unknown) => void;
  vi.mocked(api.ExpandIssue).mockReturnValue(new Promise((r) => (finish = r)) as never);
  const { user } = show();
  await user.click(await screen.findByRole("button", { name: "Expand links (1)" }));
  await user.type(screen.getByRole("searchbox", { name: "Find an issue" }), "QQ-1{Enter}");
  expect(api.ExpandIssue).toHaveBeenCalledTimes(1);
  finish({ issues: [{ issue: result.issues[0], fetchedAt: "" }, { issue: issue("Z-9"), fetchedAt: "" }], offline: false, missing: [] });
  expect(await screen.findByRole("button", { name: /^Z-9/ })).toBeVisible();
});

it("centres the graph on the run's busiest issue, on a key typed in Find, and on Centre here", async () => {
  vi.mocked(api.ExpandIssue).mockResolvedValue({ issues: [{ issue: issue("CC-5"), fetchedAt: "" }], offline: false, missing: [] } as never);
  const { user } = show();
  expect(await screen.findByText("Centred on A-1")).toBeVisible();
  // Choosing a node selects it and leaves the centre where it is.
  await user.click(within(screen.getByRole("list", { name: "Nodes" })).getByRole("button", { name: "B-1" }));
  expect(screen.getByText("Centred on A-1")).toBeVisible();
  await user.click(screen.getByRole("button", { name: "Centre here" }));
  expect(screen.getByText("Centred on B-1")).toBeVisible();
  await user.type(screen.getByRole("searchbox", { name: "Find an issue" }), "cc-5{Enter}");
  expect(await screen.findByText("Centred on CC-5")).toBeVisible();
});
