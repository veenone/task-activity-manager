import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent } from "react";
import { LinkPopover } from "./LinkPopover";
import { ToolbarButton } from "./ToolbarButton";
import { ToolbarIcon } from "./icons";
import type { ToolbarGroup, ToolbarItem } from "./types";

interface Props {
  label: string;
  groups: ToolbarGroup[];
  // disabled renders every button disabled without hiding any, so a toolbar
  // locked for a while (a Sync running) keeps its place and the page under
  // it does not jump.
  disabled?: boolean;
}

// EditorToolbar is a formatting toolbar for any rich text editor, and knows
// nothing about the editor behind it. The keyboard model is the WAI-ARIA
// toolbar pattern: one tab stop, Left and Right move between items with wrap,
// Home and End jump to either end. The tab stop is remembered by item id, not
// position, so an item that appears or vanishes between renders never moves
// it onto a different button. Item ids must be unique across groups.
export function EditorToolbar({ label, groups, disabled = false }: Props) {
  const items = groups.flatMap((g) => g.items);
  const positions = new Map(items.map((item, i) => [item.id, i]));
  const [focusId, setFocusId] = useState<string | null>(null);
  const [openLink, setOpenLink] = useState<string | null>(null);
  const buttons = useRef(new Map<string, HTMLButtonElement>());
  const current = (focusId !== null && positions.get(focusId)) || 0;

  // Controller ruling F2: a toolbar disabled while the link popover is open
  // (a Sync starting mid-edit) must close it, so it cannot reappear and
  // steal focus from the editor once the lock lifts.
  useEffect(() => {
    if (disabled) setOpenLink(null);
  }, [disabled]);

  const moveTo = (index: number) => {
    if (items.length === 0) return;
    const item = items[(index + items.length) % items.length];
    setFocusId(item.id);
    buttons.current.get(item.id)?.focus();
  };

  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    // Keys typed into something inside the toolbar that is not one of its
    // buttons (the link popover's address box) belong to that control.
    if (!(e.target instanceof HTMLElement) || !e.target.hasAttribute("data-toolbar-item")) return;
    const targets: Record<string, number> = { ArrowRight: current + 1, ArrowLeft: current - 1, Home: 0, End: items.length - 1 };
    if (!(e.key in targets)) return;
    e.preventDefault();
    moveTo(targets[e.key]);
  };

  const register = (id: string) => (el: HTMLButtonElement | null) => {
    if (el) buttons.current.set(id, el);
    else buttons.current.delete(id);
  };

  const renderItem = (item: ToolbarItem) => {
    const common = {
      label: item.label,
      disabled,
      tabIndex: (positions.get(item.id) === current ? 0 : -1) as 0 | -1,
      buttonRef: register(item.id),
      onFocus: () => setFocusId(item.id),
    };
    switch (item.kind) {
      case "toggle":
        return (
          <ToolbarButton key={item.id} {...common} shortcut={item.shortcut} pressed={item.active} unavailable={item.disabled} onPress={item.onToggle}>
            <ToolbarIcon name={item.icon} />
          </ToolbarButton>
        );
      case "action":
        return (
          <ToolbarButton key={item.id} {...common} shortcut={item.shortcut} unavailable={item.disabled} onPress={item.onRun}>
            {item.icon && <ToolbarIcon name={item.icon} />}
            {item.text && <span className="editor-toolbar-text">{item.text}</span>}
          </ToolbarButton>
        );
      case "link": {
        const open = openLink === item.id && !disabled;
        return (
          <span key={item.id} className="editor-toolbar-link">
            <ToolbarButton {...common} active={item.href !== null} expanded={open} onPress={() => setOpenLink(open ? null : item.id)}>
              <ToolbarIcon name="link" />
            </ToolbarButton>
            {open && (
              <LinkPopover
                href={item.href}
                onApply={item.onApply}
                onRemove={item.onRemove}
                onClose={() => {
                  setOpenLink(null);
                  buttons.current.get(item.id)?.focus();
                }}
              />
            )}
          </span>
        );
      }
    }
  };

  return (
    <div className="editor-toolbar" role="toolbar" aria-label={label} aria-disabled={disabled || undefined} onKeyDown={onKeyDown}>
      {groups.map((group) => (
        <div key={group.id} className="editor-toolbar-group" role="group" aria-label={group.label} data-group={group.id}>
          {group.items.map(renderItem)}
        </div>
      ))}
    </div>
  );
}
