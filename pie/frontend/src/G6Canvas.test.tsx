import { StrictMode } from "react";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import G6Canvas, { type CanvasNode } from "./G6Canvas";

// jsdom has no canvas, so G6's Graph is a recorder of what it was asked.
const failNext = vi.hoisted(() => [] as string[]);
const made = vi.hoisted(
  () => [] as { calls: string[]; options: Record<string, unknown> }[],
);
vi.mock("@antv/g6", () => ({
  NodeEvent: { CLICK: "node:click" },
  Graph: class {
    calls: string[] = [];
    options: Record<string, unknown>;
    constructor(options: Record<string, unknown>) {
      this.options = options;
      made.push(this);
    }
    on() {}
    setData() {}
    setLayout(l: { type: string }) {
      this.calls.push(`layout:${l.type}`);
    }
    hasNode() {
      return true;
    }
    render() {
      this.calls.push("render");
      return failNext.length
        ? Promise.reject(new Error(failNext.pop()))
        : Promise.resolve();
    }
    draw() {
      this.calls.push("draw");
      return Promise.resolve();
    }
    fitView() {
      this.calls.push("fit");
      return Promise.resolve();
    }
    // As if the fit had enlarged a small graph.
    getZoom() {
      return 1.6;
    }
    zoomTo(z: number) {
      this.calls.push(`zoom:${z}`);
      return Promise.resolve();
    }
    fitCenter() {
      this.calls.push("centre");
      return Promise.resolve();
    }
    focusElement(id: string) {
      this.calls.push(`focus:${id}`);
      return Promise.resolve();
    }
    destroy() {
      this.calls.push("destroy");
    }
  },
}));

beforeEach(() => {
  made.length = 0;
});

const nodes: CanvasNode[] = [
  { id: "A-1", label: "A-1" },
  { id: "B-1", label: "B-1" },
];
// One list, as the views memoise theirs; a new array is new data.
const none: never[] = [];
const settle = () => new Promise((r) => setTimeout(r, 0));

it("lays out the first data of every graph it makes, StrictMode's second one too", async () => {
  render(
    <StrictMode>
      <G6Canvas
        nodes={nodes}
        edges={none}
        focus={null}
        onSelect={() => {}}
        label="Graph"
      />
    </StrictMode>,
  );
  await settle();
  const live = made[made.length - 1];
  expect(live.calls).toContain("render");
  expect(live.calls).not.toContain("draw");
});

it("lays out and fits again on every new set of nodes, and only redraws for a new focus", async () => {
  const { rerender } = render(
    <G6Canvas
      nodes={nodes}
      edges={none}
      focus={null}
      onSelect={() => {}}
      label="Graph"
    />,
  );
  await settle();
  const g = made[0];
  // Fitting shrinks what overflows and centres, but never enlarges.
  expect(g.calls).toEqual(["render", "fit", "zoom:1", "centre"]);
  expect(g.options.autoFit).toBeUndefined();
  g.calls.length = 0;
  const fewer = nodes.slice(0, 1);
  rerender(
    <G6Canvas
      nodes={fewer}
      edges={none}
      focus={null}
      onSelect={() => {}}
      label="Graph"
    />,
  );
  await settle();
  expect(g.calls).toEqual(["render", "fit", "zoom:1", "centre"]);
  g.calls.length = 0;
  rerender(
    <G6Canvas
      nodes={fewer}
      edges={none}
      focus="A-1"
      onSelect={() => {}}
      label="Graph"
    />,
  );
  await settle();
  expect(g.calls).toEqual(["draw", "focus:A-1"]);
});

it("takes the layout it is given, and lays out again when it changes", async () => {
  const radial = { type: "radial", focusNode: "A-1" };
  const { rerender } = render(
    <G6Canvas
      nodes={nodes}
      edges={none}
      focus={null}
      layout={radial}
      onSelect={() => {}}
      label="Graph"
    />,
  );
  await settle();
  const g = made[0];
  expect(g.options.layout).toMatchObject({ ...radial, animation: false });
  g.calls.length = 0;
  rerender(
    <G6Canvas
      nodes={nodes}
      edges={none}
      focus={null}
      layout={{ type: "radial", focusNode: "B-1" }}
      onSelect={() => {}}
      label="Graph"
    />,
  );
  await settle();
  expect(g.calls).toEqual(["layout:radial", "render", "fit", "zoom:1", "centre"]);
});

it("lays out again only when what is drawn changes, not for the same issues in new lists", async () => {
  const { rerender } = render(
    <G6Canvas
      nodes={nodes}
      edges={none}
      focus={null}
      onSelect={() => {}}
      label="Graph"
    />,
  );
  await settle();
  const g = made[0];
  g.calls.length = 0;
  // A view rebuilds its lists when the focus moves; the boxes must stay put.
  rerender(
    <G6Canvas
      nodes={nodes.map((n) => ({ ...n, faded: true }))}
      edges={[]}
      focus="B-1"
      onSelect={() => {}}
      label="Graph"
    />,
  );
  await settle();
  expect(g.calls).toEqual(["draw", "focus:B-1"]);
});

it("says so when the graph cannot be drawn", async () => {
  failNext.push("bad layout");
  render(
    <G6Canvas
      nodes={nodes}
      edges={none}
      focus={null}
      onSelect={() => {}}
      label="Graph"
    />,
  );
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "The graph could not be drawn: bad layout",
  );
});

it("leaves the camera on the whole fitted graph after a new layout, even with a node in focus", async () => {
  render(<G6Canvas nodes={nodes} edges={none} focus="A-1" onSelect={() => {}} label="Graph" />);
  await settle();
  // Moving to the focus after fitting pushed the rest of a grown graph off the canvas.
  expect(made[0].calls).toEqual(["render", "fit", "zoom:1", "centre"]);
});
