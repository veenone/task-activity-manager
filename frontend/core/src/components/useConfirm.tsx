import type { ReactNode } from "react";
import { useDialogs } from "../contexts/DialogContext";

// useConfirm returns the app-wide async confirm dialog (an in-app, themed
// replacement for window.confirm, which WebView2 renders out-of-theme). The
// dialog itself is rendered once by DialogProvider (audit A2); this hook just
// exposes the trigger.

export interface ConfirmOptions {
  title: string;
  // A node rather than a string, because a confirmation is sometimes more
  // than one sentence about more than one thing: TAM's delete-a-sprint
  // confirmation says what Jira does with the issues, marks itself as a
  // write that does not wait for Commit, and warns that it cannot be undone,
  // and running those together as one paragraph of plain text buries the
  // half that matters. The notice dialog's message stays a string: it is
  // read out verbatim through the live region, where markup has no meaning.
  message?: ReactNode;
  confirmLabel?: string;
  cancelLabel?: string;
  // danger styles the confirm button as destructive (the default for deletes).
  danger?: boolean;
}

export function useConfirm(): {
  confirm: (opts: ConfirmOptions) => Promise<boolean>;
} {
  return { confirm: useDialogs().confirm };
}
