import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatusBadge } from "./StatusBadge";

// The glyph is read out of the badge rather than off a class, which is the
// whole point of it: a badge that told its three states apart by colour
// alone would pass a class assertion and fail a reader who cannot see the
// colour. jsdom applies no stylesheet, so a class says nothing here anyway.
function glyphOf(container: HTMLElement): string {
  const glyph = container.querySelector<HTMLElement>(".status-badge-glyph");
  if (!glyph) throw new Error("the badge drew no glyph");
  return glyph.textContent ?? "";
}

describe("StatusBadge", () => {
  it("tells its states apart by a glyph, not only by colour", () => {
    const { container: active } = render(<StatusBadge tone="active" label="Active" />);
    const { container: future } = render(<StatusBadge tone="future" label="Future" />);
    const { container: closed } = render(<StatusBadge tone="closed" label="Closed" />);
    const { container: draft } = render(<StatusBadge tone="draft" label="Draft" />);
    const glyphs = [glyphOf(active), glyphOf(future), glyphOf(closed), glyphOf(draft)];
    for (const g of glyphs) expect(g).not.toBe("");
    expect(new Set(glyphs).size).toBe(4);
  });

  it("prints the label the caller gave it, in the case the caller gave it", () => {
    render(<StatusBadge tone="active" label="Active" />);
    // Mixed case, not "ACTIVE": the shout is text-transform in the
    // stylesheet, so the accessible name a row builds from the same string
    // is not spelled out letter by letter by a screen reader.
    expect(screen.getByText("Active")).toBeInTheDocument();
    expect(screen.queryByText("ACTIVE")).not.toBeInTheDocument();
  });

  it("takes a glyph of its own when the caller has a better one", () => {
    const { container } = render(<StatusBadge tone="closed" label="Closed" glyph="!" />);
    expect(glyphOf(container)).toBe("!");
  });

  it("keeps the glyph out of the accessible name, so the name is the label", () => {
    render(<StatusBadge tone="future" label="Future" />);
    const glyph = screen.getByText("Future").parentElement?.querySelector(".status-badge-glyph");
    expect(glyph).toHaveAttribute("aria-hidden", "true");
  });
});
