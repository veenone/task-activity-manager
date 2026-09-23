import { createModalContext } from "@agile-suite/core";

export type ModalId = "profiles" | "about" | "diagnostics" | "pending" | "newIssue" | "import";

export const { ModalProvider, useModal } = createModalContext<ModalId>();
