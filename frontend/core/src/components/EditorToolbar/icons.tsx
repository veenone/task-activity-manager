import type { ReactNode } from "react";

export type IconName =
  | "bold" | "italic" | "underline" | "strike"
  | "heading2" | "heading3"
  | "bulletList" | "orderedList" | "taskList"
  | "table" | "link"
  | "undo" | "redo";

// Drawn in currentColor on a 16px grid, so dark mode and forced colors follow
// the text colour the button already has.
const SHAPES: Record<IconName, ReactNode> = {
  bold: <path d="M4.5 2.5h4a2.75 2.75 0 0 1 0 5.5h-4zM4.5 8h4.75a2.75 2.75 0 0 1 0 5.5H4.5z" strokeWidth="2" />,
  italic: <path d="M10 2.5H6.5M9.5 13.5H6M9 2.5 7 13.5" />,
  underline: <path d="M4.5 2.5v5a3.5 3.5 0 0 0 7 0v-5M3.5 13.5h9" />,
  strike: <path d="M2.5 8h11M11 4.5A3 2.25 0 0 0 8 2.75c-1.9 0-3 .9-3 2.1 0 .8.5 1.5 1.5 1.9M5 11.5a3 2.25 0 0 0 3 1.75c1.9 0 3-.9 3-2.1" />,
  heading2: <path d="M2 3.5v9M7 3.5v9M2 8h5M9.5 6a1.75 1.75 0 1 1 3.5 0c0 1.5-3.5 3-3.5 6.5H13" />,
  heading3: <path d="M2 3.5v9M7 3.5v9M2 8h5M9.5 4.5H13l-2 2.5a2 2 0 1 1-1.5 3.5" />,
  bulletList: (
    <>
      <path d="M6 4h7.5M6 8h7.5M6 12h7.5" />
      <circle cx="3" cy="4" r="0.75" fill="currentColor" />
      <circle cx="3" cy="8" r="0.75" fill="currentColor" />
      <circle cx="3" cy="12" r="0.75" fill="currentColor" />
    </>
  ),
  orderedList: <path d="M6.5 4h7M6.5 8h7M6.5 12h7M2.5 2.75l1-.5v3.5M2.25 10.25a.9.9 0 0 1 1.5-.25c.3.45-1.75 1.75-1.75 1.75h1.75" />,
  taskList: (
    <>
      <rect x="2" y="2.5" width="4" height="4" rx="1" />
      <path d="m2.5 11.5 1 1 1.75-2M8.5 4.5h5M8.5 11.5h5" />
    </>
  ),
  table: (
    <>
      <rect x="2" y="2.5" width="12" height="11" rx="1.5" />
      <path d="M2 6.5h12M2 10h12M6.5 6.5v7" />
    </>
  ),
  link: <path d="M6.75 9.25a2.5 2.5 0 0 0 3.5 0l2-2a2.5 2.5 0 0 0-3.5-3.5l-.75.75M9.25 6.75a2.5 2.5 0 0 0-3.5 0l-2 2a2.5 2.5 0 0 0 3.5 3.5l.75-.75" />,
  undo: <path d="M5.5 3.5 2.5 6.5l3 3M2.5 6.5h7a3.5 3.5 0 0 1 0 7H7" />,
  redo: <path d="m10.5 3.5 3 3-3 3M13.5 6.5h-7a3.5 3.5 0 0 0 0 7H9" />,
};

export function ToolbarIcon({ name }: { name: IconName }) {
  return (
    <svg
      className="editor-toolbar-icon"
      width="16"
      height="16"
      viewBox="0 0 16 16"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {SHAPES[name]}
    </svg>
  );
}
