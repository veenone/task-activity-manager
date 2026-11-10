import { expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { BaselineView } from "./BaselineView";

// jsdom has no canvas, so G6 is replaced by lists of what it was given.
vi.mock("./G6Canvas", () => ({
  default: ({
    nodes,
    edges,
    combos,
    onSelect,
  }: {
    nodes: { id: string; label: string; combo?: string; lit?: boolean; faded?: boolean }[];
    edges: { id: string }[];
    combos: { id: string; label: string; parent?: string }[];
    onSelect: (id: string) => void;
  }) => (
    <div>
      <ul aria-label="Groups">
        {combos.map((c) => (
          <li key={c.id}>{`${c.id} ${c.label}${c.parent ? ` in ${c.parent}` : ""}`}</li>
        ))}
      </ul>
      <ul aria-label="Nodes">
        {nodes.map((n) => (
          <li key={n.id}>
            <button type="button" onClick={() => onSelect(n.id)}>
              {`${n.label} @${n.combo}${n.lit ? " lit" : ""}${n.faded ? " faded" : ""}`}
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

const NOW = Date.now();
const ago = (d: number) => new Date(NOW - d * 86_400_000).toISOString();
const link = (key: string, type: string, outward: boolean, words: string) => ({ key, type, outward, words, symmetric: false });
const issue = (key: string, more: object = {}) => ({
  key,
  project: key.split("-")[0],
  projectName: "",
  summary: `About ${key}`,
  statusCategory: "new",
  created: ago(10),
  links: [],
  subtasks: [],
  parent: "",
  fixVersions: [],
  ...more,
});

// RFT-91 (outside) blocks COS-210, which blocks COS-188, which blocks COL-45.
// COS-231 has no link out, so it folds away.
const result = {
  issues: [
    issue("RFT-91", { links: [link("COS-210", "Blocks", true, "blocks")] }),
    issue("COS-210", { fixVersions: ["3.4.0"], links: [link("RFT-91", "Blocks", false, "is blocked by"), link("COS-188", "Blocks", true, "blocks")] }),
    issue("COS-188", { fixVersions: ["3.4.0"], links: [link("COS-210", "Blocks", false, "is blocked by"), link("COL-45", "Blocks", true, "blocks")] }),
    issue("COL-45", { fixVersions: ["3.4.0"], links: [link("COS-188", "Blocks", false, "is blocked by")] }),
    issue("COS-231", { fixVersions: ["3.3.0"], statusCategory: "done" }),
    issue("COS-232", { fixVersions: ["3.3.0"] }),
  ],
  sets: [],
};
const baselines = [{ key: "COS", descendants: ["COL"] }];

function show() {
  const onOpen = vi.fn();
  render(<BaselineView result={result as never} baselines={baselines as never} onOpen={onOpen} />);
  return { user: userEvent.setup(), onOpen };
}

const list = (name: string) => within(screen.getByRole("list", { name })).queryAllByRole("listitem").map((l) => l.textContent);

it("draws the baseline inside its boundary, derived projects nested, and linked issues outside", async () => {
  show();
  await screen.findByRole("list", { name: "Nodes" });
  expect(list("Groups")).toEqual(["base Baseline COS", "d:COL Derived: COL in base", "x:RFT RFT"]);
  // Only COS-210 links out; the open issues without a link out fold, per project.
  expect(list("Nodes")).toEqual(["COS-210 @base", "RFT-91 @x:RFT", "+ 2 more @base", "+ 1 more @d:COL"]);
  expect(screen.getByRole("status")).toHaveTextContent("1 of 5 issues inside drawn, 1 outside");
});

it("traces what blocks an issue, back through every step, and lists the chain", async () => {
  const { user, onOpen } = show();
  await user.click(await screen.findByRole("button", { name: "+ 1 more @d:COL" }));
  await user.click(screen.getByRole("button", { name: /^COL-45/ }));
  expect(list("Nodes")).toEqual(
    expect.arrayContaining(["COS-210 @base lit", "COS-188 @base lit", "COL-45 @d:COL", "RFT-91 @x:RFT lit"]),
  );
  expect(list("Chain")).toEqual(["COS-188blocksCOL-45", "COS-210blocksCOS-188", "RFT-91blocksCOS-210"]);
  await user.click(screen.getByRole("button", { name: "Open COL-45 in Trace" }));
  expect(onOpen).toHaveBeenCalledWith("COL-45");
});

it("traces impact forward instead, and fades what is off the chain", async () => {
  const { user } = show();
  await user.click(await screen.findByRole("radio", { name: "Impact" }));
  await user.click(screen.getByRole("button", { name: /^RFT-91/ }));
  expect(list("Chain")).toEqual(["RFT-91blocksCOS-210", "COS-210blocksCOS-188", "COS-188blocksCOL-45"]);
  await user.click(screen.getByRole("checkbox", { name: "Only issues with links out" }));
  expect(list("Nodes")).toContain("COS-232 @base faded");
});

it("shows only the links crossing the boundary when asked", async () => {
  const { user } = show();
  await user.click(await screen.findByRole("checkbox", { name: "Only issues with links out" }));
  await user.click(screen.getByRole("radio", { name: "Crossing its boundary" }));
  expect(list("Edges")).toEqual(["RFT-91>COS-210:Blocks"]);
});

it("counts the baseline's health for a version, and compares it with another", async () => {
  const { user } = show();
  const health = await screen.findByRole("list", { name: "Health" });
  expect(health).toHaveTextContent("5 issues inside");
  expect(health).toHaveTextContent("20% finished");
  expect(health).toHaveTextContent("1 blocked from outside");
  await user.selectOptions(screen.getByRole("combobox", { name: "Version" }), "3.4.0");
  expect(screen.getByRole("list", { name: "Health" })).toHaveTextContent("3 issues inside");
  await user.selectOptions(screen.getByRole("combobox", { name: "Compared with" }), "3.3.0");
  const cmp = screen.getByLabelText("Comparison");
  expect(cmp).toHaveTextContent("issues inside3 (+1)");
  expect(cmp).toHaveTextContent("finished0% (−50 pts)");
});

it("says so when no project is marked as a baseline", () => {
  render(<BaselineView result={result as never} baselines={[]} onOpen={vi.fn()} />);
  expect(screen.getByText(/Mark a project as a baseline/)).toBeVisible();
});
