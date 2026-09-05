import { createModalContext } from "@agile-suite/core";

export type ModalId = "profiles" | "about";

export const { ModalProvider, useModal } = createModalContext<ModalId>();
