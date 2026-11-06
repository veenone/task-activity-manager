import { useEffect, useState } from "react";
import { ListCases, OpenCases } from "../wailsjs/go/main/App";
import type { cases } from "../wailsjs/go/models";
import { errorText } from "./AddProfileDialog";

// useCases holds the profile's cases. A slow answer for a profile no longer
// selected is dropped, as App does for saved queries.
export function useCases(profileId: string, onError: (message: string) => void) {
  const [list, setList] = useState<cases.Case[]>([]);
  useEffect(() => {
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

  async function open(keys: string[]) {
    try {
      setList(await OpenCases(profileId, keys));
    } catch (e) {
      onError(errorText(e));
    }
  }

  return { keys: new Set(list.map((c) => c.key)), open };
}
