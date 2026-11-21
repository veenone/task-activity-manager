import { createContext, useContext, useEffect, useRef, useState } from "react";
import { clearViewState, useViewState } from "@agile-suite/core";

// ViewScope is the active profile, under which the views keep their state.
export const ViewScope = createContext("");

// useKept is useState for a view's filters, page, selection and sub-view. The
// value lives in core's view-state store, as TAM's and XTM's views keep theirs,
// so a view that unmounts on a tab switch comes back as it was left (#546).
// view names the view and field the value within it.
export function useKept<T>(view: string, field: string, initial: T) {
  return useViewState(useContext(ViewScope), view, field, initial);
}

// useKeptIf keeps the value when view is given and is plain useState without
// one, for a hook such as usePaged that also serves lists nobody returns to.
export function useKeptIf<T>(view: string | undefined, field: string, initial: T) {
  const local = useState(initial);
  const kept = useKept(view ?? "", field, initial);
  return view ? kept : local;
}

// useClearOnSwitch drops a profile's view state once another profile is
// chosen, so going back to it starts every view clean, as TAM does.
export function useClearOnSwitch(profileId: string) {
  const held = useRef(profileId);
  useEffect(() => {
    if (held.current && held.current !== profileId) clearViewState(held.current);
    held.current = profileId;
  }, [profileId]);
}
