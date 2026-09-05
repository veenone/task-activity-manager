import {
  createContext,
  useCallback,
  useContext,
  useMemo,
  useReducer,
} from "react";
import type { ReactNode } from "react";

export interface ModalApi<Id extends string> {
  current: Id | null;
  isOpen: (id: Id) => boolean;
  openModal: (id: Id) => void;
  closeModal: () => void;
}

// One root-level overlay is open at a time, so a single `current` id replaces
// a pile of booleans. Each app names its own modal ids and gets a typed
// provider and hook back.
export function createModalContext<Id extends string>(
  providerName = "ModalProvider",
) {
  const Ctx = createContext<ModalApi<Id> | null>(null);
  type Action = { type: "OPEN"; id: Id } | { type: "CLOSE" };

  function reducer(state: Id | null, action: Action): Id | null {
    switch (action.type) {
      case "OPEN":
        return action.id;
      case "CLOSE":
        return null;
      default:
        return state;
    }
  }

  function ModalProvider({ children }: { children: ReactNode }) {
    const [current, dispatch] = useReducer(reducer, null);
    const isOpen = useCallback((id: Id) => current === id, [current]);
    const openModal = useCallback(
      (id: Id) => dispatch({ type: "OPEN", id }),
      [],
    );
    const closeModal = useCallback(() => dispatch({ type: "CLOSE" }), []);
    const api = useMemo<ModalApi<Id>>(
      () => ({ current, isOpen, openModal, closeModal }),
      [current, isOpen, openModal, closeModal],
    );
    return <Ctx.Provider value={api}>{children}</Ctx.Provider>;
  }

  function useModal(): ModalApi<Id> {
    const ctx = useContext(Ctx);
    if (!ctx) {
      throw new Error(`useModal must be used within a ${providerName}`);
    }
    return ctx;
  }

  return { ModalProvider, useModal };
}
