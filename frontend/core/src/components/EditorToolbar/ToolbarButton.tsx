import { useEffect, useId, useState } from "react";
import type { ReactNode, Ref } from "react";

interface Props {
  label: string;
  shortcut?: string;
  // pressed is set only on a toggle; an action carries no aria-pressed.
  pressed?: boolean;
  // active marks state that is not a toggle, such as the link button while
  // the caret sits in a link.
  active?: boolean;
  // expanded is set only on a button that opens a popover.
  expanded?: boolean;
  disabled: boolean;
  unavailable?: boolean;
  tabIndex: 0 | -1;
  buttonRef: Ref<HTMLButtonElement>;
  onFocus: () => void;
  onPress: () => void;
  children: ReactNode;
}

export function tooltipText(label: string, shortcut?: string): string {
  return shortcut ? `${label} (${shortcut})` : label;
}

// ToolbarButton is one toolbar button and its tooltip. The tooltip shows on
// hover and on keyboard focus alike, and describes the button whether or not
// it is showing. Escape hides it until the next hover or focus (WCAG 1.4.13),
// heard on the document so a tooltip shown by hover alone dismisses too.
export function ToolbarButton({
  label, shortcut, pressed, active, expanded, disabled, unavailable, tabIndex, buttonRef, onFocus, onPress, children,
}: Props) {
  const tipId = useId();
  const [hovered, setHovered] = useState(false);
  const [focused, setFocused] = useState(false);
  const [dismissed, setDismissed] = useState(false);
  const showing = (hovered || focused) && !dismissed;

  useEffect(() => {
    if (!showing) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setDismissed(true);
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [showing]);

  return (
    <span
      className="editor-toolbar-slot"
      onMouseEnter={() => {
        setHovered(true);
        setDismissed(false);
      }}
      onMouseLeave={() => setHovered(false)}
    >
      <button
        ref={buttonRef}
        type="button"
        className="editor-toolbar-button"
        data-toolbar-item=""
        data-active={active ? "true" : undefined}
        aria-label={label}
        aria-pressed={pressed}
        aria-expanded={expanded}
        aria-haspopup={expanded === undefined ? undefined : "dialog"}
        aria-disabled={unavailable ? true : undefined}
        aria-describedby={tipId}
        disabled={disabled}
        tabIndex={tabIndex}
        // A mouse press must not take focus out of the editor, or the
        // selection a formatting command acts on is gone before it runs.
        onMouseDown={(e) => e.preventDefault()}
        onFocus={() => {
          setFocused(true);
          setDismissed(false);
          onFocus();
        }}
        onBlur={() => setFocused(false)}
        onClick={() => {
          onFocus();
          if (!unavailable) onPress();
        }}
      >
        {children}
      </button>
      <span role="tooltip" id={tipId} className="editor-toolbar-tooltip" hidden={!showing}>
        {tooltipText(label, shortcut)}
      </span>
    </span>
  );
}
