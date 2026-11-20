import { useCallback } from "react";
import { useConfirm } from "@agile-suite/core";

// Whether the open issue panel holds an edit Save edit has not journalled.
// Only one panel is ever open, so one flag is enough. It is module state
// rather than context because what reads it (the shell's view tabs and
// profile select, and each view's row selection) sits far from the panel
// that writes it, and every one of those already renders the panel's only
// way in or out (#521).
let unsaved = false;

// markUnsavedEdit is EditableFields saying whether it holds unsaved text.
export function markUnsavedEdit(value: boolean): void {
  unsaved = value;
}

// useDiscardGuard returns a wrapper for anything that would unmount the
// issue panel. With nothing unsaved it runs the action straight away, in the
// same event, so every path that had nothing to lose behaves as before.
// Otherwise it asks the way the New issue dialog asks about its draft, and
// runs the action only on Discard.
export function useDiscardGuard(): (proceed: () => void) => void {
  const { confirm } = useConfirm();
  return useCallback(
    (proceed: () => void) => {
      if (!unsaved) {
        proceed();
        return;
      }
      void confirm({
        title: "Discard your edit?",
        message: "The fields you changed in the issue panel have not been saved. Nothing reaches Jira either way.",
        confirmLabel: "Discard",
        danger: true,
      }).then((ok) => {
        if (!ok) return;
        unsaved = false;
        proceed();
      });
    },
    [confirm],
  );
}

// useGuardedSelect wraps a view's row selection so that picking a different
// issue asks before the panel's unsaved edit goes with the old one. Picking
// the issue already open replaces nothing and asks nothing.
export function useGuardedSelect(selectedKey: string, select: (key: string) => void): (key: string) => void {
  const guard = useDiscardGuard();
  return useCallback(
    (key: string) => {
      if (key === selectedKey) return;
      guard(() => select(key));
    },
    [guard, selectedKey, select],
  );
}

// useGuardedCardSelect is the same guard for the Boards view, whose opener
// also moves keyboard focus to the card. Focus follows only when the panel
// does, so a cancelled switch leaves both on the issue being edited.
export function useGuardedCardSelect(
  selectedKey: string,
  selectKey: (key: string) => void,
  focus: (id: string) => void,
): (issue: { key: string }, id: string) => void {
  const guard = useDiscardGuard();
  return useCallback(
    (issue: { key: string }, id: string) => {
      if (issue.key === selectedKey) {
        focus(id);
        return;
      }
      guard(() => {
        selectKey(issue.key);
        focus(id);
      });
    },
    [guard, selectedKey, selectKey, focus],
  );
}

// useGuarded wraps a setter, such as the shell's view or profile switch, so
// it runs through the guard. The wrapper is stable while the setter is.
export function useGuarded<T>(set: (value: T) => void): (value: T) => void {
  const guard = useDiscardGuard();
  return useCallback((value: T) => guard(() => set(value)), [guard, set]);
}
