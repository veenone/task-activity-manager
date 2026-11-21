import { useCallback, useEffect, useRef } from "react";
import { isModalOpen } from "@agile-suite/core";
import type { View } from "../contexts/NavContext";
import type { ViewDef } from "../views";

// useViewShortcuts is how a key or the View menu picks a view. Ctrl+1..9 open
// the Nth tab this profile shows; they used to be native accelerators tied to
// fixed view ids, so on a profile that hides some views the numbers no longer
// matched the tabs (#528). Neither path switches while a dialog is open,
// since switching under it unmounts the draft it holds.
//
// It returns the View menu's handler, which ignores an id the profile hides.
export function useViewShortcuts(shownViews: ViewDef[], setView: (v: View) => void): (id: string) => void {
  const latest = useRef({ shownViews, setView });
  latest.current = { shownViews, setView };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (!(e.ctrlKey || e.metaKey) || e.shiftKey || e.altKey) return;
      const n = Number(e.key);
      if (!Number.isInteger(n) || n < 1 || n > 9) return;
      const target = latest.current.shownViews[n - 1];
      e.preventDefault();
      if (target && !isModalOpen()) latest.current.setView(target.id);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, []);

  return useCallback((id: string) => {
    const target = latest.current.shownViews.find((v) => v.id === id);
    if (target && !isModalOpen()) latest.current.setView(target.id);
  }, []);
}
