// ModalContext collapses App.tsx's ~16 modal-visibility booleans into one
// reducer (spec §5.5). Only one of these overlay modals is ever open at a time
// — they are all root-level blocking overlays, and the one case that opens a
// sibling (the bridge wizard → connections) closes itself first — so a single
// `openModal: ModalId | null` faithfully replaces the booleans.
//
// The New Test panel (showNewTest) is deliberately NOT here: it shares the
// detail slot with TestDetail rather than being an overlay, so it can be open
// alongside a modal. It lives in NavContext. `editingProfile` (the form's
// edit-target) stays App-local — it is meaningful only with the form modal and
// has no bearing on the visibility invariant this reducer models.

export type ModalId =
  | "form"
  | "profiles"
  | "connections"
  | "bridge"
  | "pending"
  | "bulkEdit"
  | "bulkRename"
  | "bulkTransition"
  | "bulkAllocate"
  | "bulkMove"
  | "bulkPreconditions"
  | "bulkRequirements"
  | "bulkReview"
  | "diagnostics"
  | "about"
  | "syncHistory"
  | "import";

import { createModalContext } from "@agile-suite/core";

export const { ModalProvider, useModal } = createModalContext<ModalId>();
