import { useRef, useState } from "react";
import type { KeyboardEvent, SVGProps } from "react";

// usePointFocus is the hover and keyboard behaviour every chart's data
// points share. Each point gets an invisible hit target; hovering one shows
// its tooltip, focusing one shows it too, and Left and Right move the focus
// along the points. One point at a time is in the tab order, so the chart
// is one tab stop and not thirty.
//
// shown is the index whose tooltip is up, or null: the hovered point when
// there is one, otherwise the focused one.
export function usePointFocus(count: number) {
  const [focused, setFocused] = useState<number | null>(null);
  const [hovered, setHovered] = useState<number | null>(null);
  // The roving tab stop. It follows the focus, so tabbing back into the
  // chart lands on the point the reader left.
  const [stop, setStop] = useState(0);
  const targets = useRef<(SVGElement | null)[]>([]);

  const active = Math.min(stop, Math.max(0, count - 1));

  function move(from: number, by: number) {
    const to = Math.min(count - 1, Math.max(0, from + by));
    if (to === from) return;
    setStop(to);
    targets.current[to]?.focus();
  }

  // hit is the props a chart spreads onto the hit target of point i. The
  // element type is named at the call site, because a ref to SVGElement is
  // not assignable to a <circle>'s ref and each chart uses its own shape.
  function hit<E extends SVGElement>(i: number): SVGProps<E> {
    return {
      tabIndex: i === active ? 0 : -1,
      ref: (el: E | null) => {
        targets.current[i] = el;
      },
      onFocus: () => {
        setStop(i);
        setFocused(i);
      },
      onBlur: () => setFocused((cur) => (cur === i ? null : cur)),
      onMouseEnter: () => setHovered(i),
      onMouseLeave: () => setHovered((cur) => (cur === i ? null : cur)),
      onKeyDown: (e: KeyboardEvent<E>) => {
        if (e.key === "ArrowRight") move(i, 1);
        else if (e.key === "ArrowLeft") move(i, -1);
        else return;
        e.preventDefault();
      },
    };
  }

  return { shown: hovered ?? focused, hit };
}
