import { useEffect } from "react";

// useEscapeKey calls onEscape when Escape is pressed anywhere while `active`
// holds. It is for a popover or panel that is not a Modal (the Modal handles
// Escape itself): focus may still sit on the trigger, so the listener goes on
// the window rather than on the panel.
export function useEscapeKey(active: boolean, onEscape: () => void) {
  useEffect(() => {
    if (!active) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") onEscape();
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [active, onEscape]);
}
