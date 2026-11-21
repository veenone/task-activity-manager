import { useEffect, useRef, useState } from "react";
import type { KeyboardEvent, ReactNode } from "react";

export interface MenuItem {
  key: string;
  label?: ReactNode;
  onClick?: () => void;
  checked?: boolean;
  disabled?: boolean;
  danger?: boolean;
  title?: string;
  divider?: boolean;
}

interface Props {
  label: ReactNode;
  items: MenuItem[];
  title?: string;
  align?: "left" | "right";
  triggerClassName?: string;
  // triggerTabIndex takes the trigger out of the tab order, for a menu
  // that lives inside a widget owning its own focus: a grid is one tab
  // stop, so a menu on every cell would add one stop per cell. Such a
  // trigger is opened from the widget's own keys instead.
  triggerTabIndex?: number;
  // triggerLabel names the trigger for a screen reader when the visible
  // label is the same on every one of them.
  triggerLabel?: string;
}

const ENABLED_ITEM = '[role="menuitem"]:not(:disabled)';

// Menu is a dark, bar-native dropdown: a trigger button that reveals a panel of
// actions anchored beneath it. Closes on outside click or Escape. Used to
// collapse the topbar's scattered buttons into grouped menus.
//
// The keys follow the ARIA menu button pattern: opening moves focus to the
// first enabled item, Up and Down wrap through the enabled items, Home and End
// jump to the ends, and Escape, Tab or a pick closes the menu with focus back
// on the trigger.
export function Menu({ label, items, title, align = "left", triggerClassName, triggerTabIndex, triggerLabel }: Props) {
  const [open, setOpen] = useState(false);
  const trigger = useRef<HTMLButtonElement>(null);
  const panel = useRef<HTMLDivElement>(null);

  const enabledItems = () =>
    Array.from(panel.current?.querySelectorAll<HTMLButtonElement>(ENABLED_ITEM) ?? []);

  useEffect(() => {
    // The panel itself takes focus when every item is disabled, so Escape
    // still reaches its handler.
    if (open) (panel.current?.querySelector<HTMLElement>(ENABLED_ITEM) ?? panel.current)?.focus();
  }, [open]);

  function close() {
    setOpen(false);
    trigger.current?.focus();
  }

  function onPanelKey(e: KeyboardEvent<HTMLDivElement>) {
    if (e.key === "Escape" || e.key === "Tab") {
      // Escape stops here so a detail panel or dialog around the menu stays
      // open; Tab goes on to move focus past the trigger.
      if (e.key === "Escape") {
        e.preventDefault();
        e.stopPropagation();
      }
      close();
      return;
    }
    const list = enabledItems();
    const at = list.indexOf(document.activeElement as HTMLButtonElement);
    const next: Record<string, number> = {
      ArrowDown: (at + 1) % list.length,
      ArrowUp: (at - 1 + list.length) % list.length,
      Home: 0,
      End: list.length - 1,
    };
    if (list.length > 0 && e.key in next) {
      e.preventDefault();
      list[next[e.key]].focus();
    }
  }

  return (
    <div className="menu">
      <button
        ref={trigger}
        className={triggerClassName ?? "topbar-btn"}
        onClick={() => setOpen((o) => !o)}
        title={title}
        tabIndex={triggerTabIndex}
        aria-label={triggerLabel}
        aria-haspopup="menu"
        aria-expanded={open}
      >
        {label}
        <span className="menu-caret" aria-hidden="true">
          ▾
        </span>
      </button>
      {open && (
        <>
          {/* A pointer shortcut only: Escape closes the menu from the keyboard. */}
          <div className="menu-backdrop" role="presentation" onClick={() => setOpen(false)} />
          <div ref={panel} className={`menu-panel menu-panel-${align}`} role="menu" tabIndex={-1} onKeyDown={onPanelKey}>
            {items.map((it) =>
              it.divider ? (
                <div key={it.key} className="menu-divider" />
              ) : (
                <button
                  key={it.key}
                  className={`menu-item${it.danger ? " menu-item-danger" : ""}`}
                  role="menuitem"
                  disabled={it.disabled}
                  title={it.title}
                  tabIndex={-1}
                  onClick={() => {
                    close();
                    it.onClick?.();
                  }}
                >
                  <span className="menu-check" aria-hidden="true">
                    {it.checked ? "✓" : ""}
                  </span>
                  <span className="menu-label">{it.label}</span>
                </button>
              ),
            )}
          </div>
        </>
      )}
    </div>
  );
}
