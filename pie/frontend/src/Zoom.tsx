import { type ReactNode, useEffect, useRef, useState } from "react";

const STEP = 1.25;
const MIN = 0.25;
const MAX = 4;

// Zoom lets a chart grow past its box: the chart is drawn at a width of its own
// and the box scrolls. A sankey of hundreds of projects can then be read a
// part at a time (#448). Ctrl and the wheel zoom too, as in a browser.
// minWidth, in pixels at 100%, keeps a chart from being squeezed below the
// width its labels need; it scales with the zoom, so zooming out still
// shrinks the chart (#455).
export function Zoom({ label, minWidth = 0, children }: { label: string; minWidth?: number; children: ReactNode }) {
  const [scale, setScale] = useState(1);
  const box = useRef<HTMLDivElement>(null);
  const by = (f: number) => setScale((s) => Math.min(MAX, Math.max(MIN, s * f)));

  // React listens to wheel passively, so the page would scroll as well. A
  // listener of our own can keep Ctrl+wheel for the chart.
  useEffect(() => {
    const el = box.current;
    if (!el) return;
    const onWheel = (e: WheelEvent) => {
      if (!e.ctrlKey) return;
      e.preventDefault();
      // Follow the wheel's distance, so a trackpad pinch, many tiny steps,
      // zooms smoothly while one mouse notch (about 100) is one STEP.
      by(Math.exp((-e.deltaY * Math.log(STEP)) / 100));
    };
    el.addEventListener("wheel", onWheel, { passive: false });
    return () => el.removeEventListener("wheel", onWheel);
  }, []);

  return (
    <div className="pie-zoom">
      <div className="pie-zoom-controls" role="group" aria-label={`${label} zoom`}>
        <button type="button" className="btn btn-ghost" aria-label="Zoom out" disabled={scale <= MIN} onClick={() => by(1 / STEP)}>
          −
        </button>
        <span className="muted">{`${Math.round(scale * 100)}%`}</span>
        <button type="button" className="btn btn-ghost" aria-label="Zoom in" disabled={scale >= MAX} onClick={() => by(STEP)}>
          +
        </button>
        <button type="button" className="btn btn-ghost" aria-label="Reset zoom" disabled={scale === 1} onClick={() => setScale(1)}>
          Reset
        </button>
      </div>
      <div className="pie-zoom-box" ref={box}>
        <div className="pie-zoom-area" style={{ width: `${scale * 100}%`, minWidth: `${minWidth * scale}px` }}>
          {children}
        </div>
      </div>
    </div>
  );
}
