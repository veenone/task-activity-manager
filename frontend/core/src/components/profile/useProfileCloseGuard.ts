import { useState } from "react";
import { useConfirm } from "../useConfirm";

// useProfileCloseGuard stands between a dialog holding a profile form and its
// close: Escape, a click outside and the close button. The form reports
// through onDirtyChange whether it holds unsaved input, and requestClose asks
// before dropping it, so a typed URL or token is never lost without a word
// (#542). resetDirty is for a form that goes away by its own Cancel.
export function useProfileCloseGuard(onClose: () => void) {
  const { confirm } = useConfirm();
  const [dirty, setDirty] = useState(false);

  async function requestClose() {
    if (
      dirty &&
      !(await confirm({
        title: "Discard your changes to this profile?",
        message: "What you typed in the form has not been saved.",
        confirmLabel: "Discard",
        cancelLabel: "Keep editing",
        danger: true,
      }))
    ) {
      return;
    }
    setDirty(false);
    onClose();
  }

  return { onDirtyChange: setDirty, requestClose, resetDirty: () => setDirty(false) };
}
