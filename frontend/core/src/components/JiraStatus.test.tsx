import fs from "node:fs";
import path from "node:path";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { JiraStatus, statusCategory } from "./JiraStatus";

const primitivesCss = fs
  .readFileSync(path.resolve(__dirname, "../../styles/primitives.css"), "utf8")
  .replace(/\/\*[\s\S]*?\*\//g, " ");

/** Declarations of the rule whose selector is exactly `selector`. */
function rule(selector: string): string {
  const start = primitivesCss.indexOf(`\n${selector} {`);
  if (start < 0) return "";
  const open = primitivesCss.indexOf("{", start);
  return primitivesCss.slice(open + 1, primitivesCss.indexOf("}", open));
}

describe("statusCategory", () => {
  it("colours from Jira's category, whatever the instance calls the status", () => {
    expect(statusCategory("Zu erledigen", "new")).toBe("todo");
    expect(statusCategory("En cours", "indeterminate")).toBe("active");
    expect(statusCategory("Erledigt", "done")).toBe("done");
    // Which is the point: these three names guess as grey.
    expect(statusCategory("Erledigt")).toBe("todo");
  });

  it("falls back to the name when no category is known", () => {
    // XTM stores no category at all, and a TAM row synced before schema 19
    // has none either.
    expect(statusCategory("In Progress")).toBe("active");
    expect(statusCategory("In Review", "")).toBe("active");
    expect(statusCategory("Resolved")).toBe("done");
    expect(statusCategory("To Do")).toBe("todo");
  });
});

describe("JiraStatus", () => {
  it("prints the status as Jira names it, in the chip for its category", () => {
    render(<JiraStatus status="Erledigt" category="done" />);
    const chip = screen.getByText("Erledigt");
    expect(chip).toHaveClass("chip-status", "chip-status-done");
    expect(chip).toHaveAttribute("title", "Erledigt");
  });

  // jsdom applies no stylesheet; the colours are pinned on the rules. XTM
  // drew every status in one sky-blue pill, so Done and To Do looked alike.
  it("gives each category its own fill from the tokens", () => {
    expect(rule(".chip-status-todo")).toMatch(/background:\s*var\(--surface-3\)/);
    expect(rule(".chip-status-active")).toMatch(/background:\s*var\(--chip-blue-bg\)/);
    expect(rule(".chip-status-done")).toMatch(/background:\s*var\(--chip-green-bg\)/);
  });
});
