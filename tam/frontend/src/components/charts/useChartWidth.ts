import { useEffect, useState } from "react";
import type { RefObject } from "react";
import { FALLBACK_WIDTH } from "./geometry";

// useChartWidth is the width of the box a chart draws into, kept current
// through a ResizeObserver the way XTM's Sankey measures its own. Height is
// fixed by the chart, so this is the one dimension that is measured.
//
// Before the first measurement, and under a runner with no ResizeObserver,
// it answers FALLBACK_WIDTH rather than zero, so a chart always draws
// something and a test can count what it drew.
export function useChartWidth(ref: RefObject<HTMLElement | null>): number {
  const [width, setWidth] = useState(0);
  useEffect(() => {
    const el = ref.current;
    if (!el || typeof ResizeObserver === "undefined") return;
    const ro = new ResizeObserver((entries) => {
      setWidth(Math.floor(entries[0].contentRect.width));
    });
    ro.observe(el);
    return () => ro.disconnect();
  }, [ref]);
  return width > 0 ? width : FALLBACK_WIDTH;
}
