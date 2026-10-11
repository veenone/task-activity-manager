import { useEffect, useRef } from "react";
import type { RefObject } from "react";

// useFocusWhen focuses the element behind the returned ref each time `when`
// changes to a truthy value. It is for a control that a user action reveals
// outside a dialog: a dropdown's search box, an inline editor, an inline form.
// Pass the state that action sets (an open flag, the key being edited), so
// focus moves once per action and never on unrelated re-renders.
//
// Inside a Modal, name the control with the Modal's initialFocus prop instead;
// the Modal owns initial focus there.
export function useFocusWhen<T extends HTMLElement>(
  when: unknown,
): RefObject<T | null> {
  const ref = useRef<T>(null);
  useEffect(() => {
    if (when) ref.current?.focus();
  }, [when]);
  return ref;
}
