import { createModalContext } from "@agile-suite/core";

export type ModalId = "profiles" | "settings" | "about" | "diagnostics" | "pending" | "newIssue" | "import";

export const { ModalProvider, useModal } = createModalContext<ModalId>();
