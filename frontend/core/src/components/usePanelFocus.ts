import { useEffect, useRef } from "react";
import type { RefObject } from "react";

// usePanelFocus gives a detail panel beside a list the Escape key and the
// focus return a dialog has, without trapping or taking focus: the panel is
// not modal, and a grid whose selection follows its arrow keys has to keep
// focus while the panel follows the selection.
//
// Escape calls onEscape whether focus is in the panel or on the row that
// opened it. It leaves alone an Escape something else already handled
// (preventDefault), one inside a dialog or menu, and one typed into a field
// outside the panel. When the panel closes with focus inside it, focus goes
// back to the element that had it when `itemKey` last changed, normally the
// row that opened it.
export function usePanelFocus<T extends HTMLElement>(
  onEscape: () => void,
  itemKey: unknown,
): RefObject<T | null> {
  const ref = useRef<T>(null);
  const escape = useRef(onEscape);
  useEffect(() => {
    escape.current = onEscape;
  });

  useEffect(() => {
    const node = ref.current;
    const active = document.activeElement;
    const opener = active instanceof HTMLElement && active !== document.body ? active : null;

    function onKey(e: KeyboardEvent) {
      if (e.key !== "Escape" || e.defaultPrevented) return;
      const target = e.target instanceof HTMLElement ? e.target : null;
      if (target?.closest('[role="dialog"], [role="alertdialog"], [role="menu"]')) return;
      const inPanel = !!target && !!node?.contains(target);
      const typing = !!target && (target.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(target.tagName));
      if (typing && !inPanel) return;
      e.preventDefault();
      escape.current();
    }
    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      // Hand focus back only when the panel held it, or held it until it was
      // removed. A click on another row has already put focus where the user
      // wants it.
      const now = document.activeElement;
      if (!now || now === document.body || node?.contains(now)) opener?.focus();
    };
  }, [itemKey]);

  return ref;
}
