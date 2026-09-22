import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { ProgressBar } from "./ProgressBar";

const bar = (name: string) => screen.getByRole("progressbar", { name });

describe("ProgressBar", () => {
  it("reports the numbers the caller gave it, and a sentence a reader can hear", () => {
    render(<ProgressBar value={6} max={14} label="Time elapsed in Sprint 14" valueText="Day 6 of 14" />);
    const el = bar("Time elapsed in Sprint 14");
    expect(el).toHaveAttribute("aria-valuenow", "6");
    expect(el).toHaveAttribute("aria-valuemin", "0");
    expect(el).toHaveAttribute("aria-valuemax", "14");
    expect(el).toHaveAttribute("aria-valuetext", "Day 6 of 14");
  });

  it("clamps a value that ran past either end of its own track", () => {
    render(<ProgressBar value={19} max={14} label="Over" valueText="over" />);
    render(<ProgressBar value={-3} max={14} label="Under" valueText="under" />);
    expect(bar("Over")).toHaveAttribute("aria-valuenow", "14");
    expect(bar("Under")).toHaveAttribute("aria-valuenow", "0");
  });

  it("draws an empty bar rather than NaN when there is nothing to divide by", () => {
    // A future sprint with nothing estimated is max 0, and a width of
    // NaN% is a rule the browser drops, which leaves the fill at whatever
    // it inherited rather than at empty.
    const { container } = render(<ProgressBar value={0} max={0} label="Points" valueText="0 of 0 pts" />);
    expect(bar("Points")).toHaveAttribute("aria-valuenow", "0");
    for (const el of container.querySelectorAll("[style]")) {
      expect(el.getAttribute("style")).not.toContain("NaN");
    }
  });

  it("fills in proportion to the value, so the picture and the number agree", () => {
    const { container } = render(<ProgressBar value={7} max={28} label="Quarter" valueText="7 of 28" />);
    const fill = container.querySelector<HTMLElement>(".progress-bar-fill");
    expect(fill?.style.width).toBe("25%");
  });

  it("puts the marker where the caller asked and drops one that is off the track", () => {
    const { container: on } = render(<ProgressBar value={1} max={2} label="On" valueText="on" marker={0.4} />);
    const { container: off } = render(<ProgressBar value={1} max={2} label="Off" valueText="off" marker={1.7} />);
    const { container: none } = render(<ProgressBar value={1} max={2} label="None" valueText="none" />);
    expect(on.querySelector<HTMLElement>(".progress-bar-marker")?.style.left).toBe("40%");
    expect(off.querySelector(".progress-bar-marker")).toBeNull();
    expect(none.querySelector(".progress-bar-marker")).toBeNull();
  });
});
