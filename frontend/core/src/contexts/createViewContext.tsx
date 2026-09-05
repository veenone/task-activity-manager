import { createContext, useContext, useMemo, useState } from "react";
import type { Dispatch, ReactNode, SetStateAction } from "react";

export interface ViewApi<V extends string> {
  view: V;
  setView: Dispatch<SetStateAction<V>>;
}

// The active top-level view. Each app names its own views; anything else it
// wants to route on lives in its own context.
export function createViewContext<V extends string>(initial: V) {
  const Ctx = createContext<ViewApi<V> | null>(null);

  function ViewProvider({ children }: { children: ReactNode }) {
    const [view, setView] = useState<V>(initial);
    const api = useMemo<ViewApi<V>>(() => ({ view, setView }), [view]);
    return <Ctx.Provider value={api}>{children}</Ctx.Provider>;
  }

  function useView(): ViewApi<V> {
    const ctx = useContext(Ctx);
    if (!ctx) {
      throw new Error("useView must be used within a ViewProvider");
    }
    return ctx;
  }

  return { ViewProvider, useView };
}
