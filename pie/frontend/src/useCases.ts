import { useEffect, useRef, useState } from "react";
import { ListCases, OpenCases } from "../wailsjs/go/main/App";
import type { cases } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";

// useCases holds the profile's cases. A slow answer for a profile no longer
// selected is dropped, as App does for saved queries.
export function useCases(profileId: string, onError: (message: string) => void) {
  const [list, setList] = useState<cases.Case[]>([]);
  // The profile now selected, so an answer for another one is dropped.
  const selected = useRef(profileId);
  useEffect(() => {
    selected.current = profileId;
    setList([]);
    if (!profileId) return;
    let current = true;
    ListCases(profileId).then(
      (l) => current && setList(l),
      (e) => current && onError(errorText(e)),
    );
    return () => {
      current = false;
    };
    // App passes setError, which React keeps stable, so this runs per profile.
  }, [profileId, onError]);

  async function open(keys: string[]): Promise<boolean> {
    const from = profileId;
    try {
      const l = await OpenCases(from, keys);
      if (selected.current === from) setList(l);
      return true;
    } catch (e) {
      onError(errorText(e));
      return false;
    }
  }

  return { keys: new Set(list.map((c) => c.key)), open };
}
