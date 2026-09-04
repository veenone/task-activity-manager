import { useEffect, useState } from "react";
import type { ReactNode } from "react";

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
}

// Menu is a dark, bar-native dropdown: a trigger button that reveals a panel of
// actions anchored beneath it. Closes on outside click or Escape. Used to
// collapse the topbar's scattered buttons into grouped menus.
export function Menu({ label, items, title, align = "left", triggerClassName }: Props) {
  const [open, setOpen] = useState(false);

  useEffect(() => {
    if (!open) return;
    function onKey(e: KeyboardEvent) {
      if (e.key === "Escape") setOpen(false);
    }
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [open]);

  return (
    <div className="menu">
      <button
        className={triggerClassName ?? "topbar-btn"}
        onClick={() => setOpen((o) => !o)}
        title={title}
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
          <div className="menu-backdrop" onClick={() => setOpen(false)} />
          <div className={`menu-panel menu-panel-${align}`} role="menu">
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
                  onClick={() => {
                    setOpen(false);
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
