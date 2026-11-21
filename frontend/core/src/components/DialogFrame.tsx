import type { ReactNode } from "react";

// The frame every dialog shares inside <Modal>: a header bar with the title and
// a close button, a padded body, and a footer whose buttons sit grouped at the
// right (a status line, when there is one, sits at the left). Dialogs used to
// pick from three footer layouts and some put their content straight into
// `.modal`, which has no padding (#538).

interface HeaderProps {
  // The id <Modal labelledBy> points at.
  titleId: string;
  title: ReactNode;
  onClose: () => void;
  // Accessible name of the close button; "Cancel" where closing cancels.
  closeLabel?: string;
  // True while the dialog is busy and must not be closed.
  disabled?: boolean;
}

export function DialogHeader({ titleId, title, onClose, closeLabel = "Close", disabled }: HeaderProps) {
  return (
    <div className="pending-head">
      <h2 id={titleId}>{title}</h2>
      <button
        type="button"
        className="btn btn-ghost"
        onClick={onClose}
        disabled={disabled}
        title={closeLabel}
        aria-label={closeLabel}
      >
        ✕
      </button>
    </div>
  );
}

export function DialogBody({ children, className }: { children: ReactNode; className?: string }) {
  return <div className={className ? `bulk-body ${className}` : "bulk-body"}>{children}</div>;
}

export function DialogFooter({ children }: { children: ReactNode }) {
  return <div className="pending-actions">{children}</div>;
}
