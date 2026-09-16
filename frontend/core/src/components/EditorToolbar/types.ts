import type { IconName } from "./icons";

// A toolbar item says what it looks like, whether it is on, and what to run.
// It says nothing about which editor it runs against: the caller maps its own
// editor's state onto these, which is what keeps EditorToolbar editor
// agnostic.
export type ToolbarToggle = {
  kind: "toggle";
  id: string;
  label: string;
  icon: IconName;
  shortcut?: string;
  active: boolean;
  // disabled means this one command cannot run here (it stays focusable);
  // disabling the whole toolbar is the toolbar's own prop.
  disabled?: boolean;
  onToggle: () => void;
};

export type ToolbarAction = {
  kind: "action";
  id: string;
  label: string;
  icon?: IconName;
  text?: string;
  shortcut?: string;
  disabled?: boolean;
  onRun: () => void;
};

export type ToolbarLink = {
  kind: "link";
  id: string;
  label: string;
  // href is the link under the caret, or null when there is none.
  href: string | null;
  // onApply answers with a refusal to show under the address box, or null
  // once the link is applied.
  onApply: (href: string) => string | null;
  onRemove: () => void;
};

export type ToolbarItem = ToolbarToggle | ToolbarAction | ToolbarLink;

export type ToolbarGroup = { id: string; label: string; items: ToolbarItem[] };
