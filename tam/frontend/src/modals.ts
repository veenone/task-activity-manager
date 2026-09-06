import { createModalContext } from "@agile-suite/core";

export type ModalId = "profiles" | "about" | "pending" | "newIssue";

export const { ModalProvider, useModal } = createModalContext<ModalId>();
