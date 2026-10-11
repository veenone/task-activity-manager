import type { KeyboardEvent } from "react";

// onActivateKey gives an element that cannot be a <button> the two keys a
// button answers: Enter and Space run the same action as a click. Space
// would otherwise scroll the page, so both keys are consumed. A key pressed
// on a control inside the element belongs to that control, so only keys
// aimed at the element itself count.
export function onActivateKey(action: () => void) {
  return (e: KeyboardEvent) => {
    if (e.target !== e.currentTarget) return;
    if (e.key !== "Enter" && e.key !== " ") return;
    e.preventDefault();
    action();
  };
}
