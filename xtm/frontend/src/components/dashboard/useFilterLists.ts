import { useEffect, useState } from "react";
import { ListComponents, ListFolders, ListStatuses, errMsg } from "../../api";
import type { Bucket } from "../../api";
import type { testrepo } from "../../../wailsjs/go/models";

// The Dashboard's filter lists, in the order their pickers sit.
const LIST_NAMES = ["folder", "component", "status"] as const;
type ListName = (typeof LIST_NAMES)[number];

// useFilterLists loads the options behind the Dashboard's Folder, Component
// and Status pickers whenever the profile changes. A list that fails to load
// leaves its picker empty, so the failure comes back as a line to show beside
// the pickers rather than only a log entry (#542). "" means every list loaded.
export function useFilterLists(profileId: string) {
  const [folders, setFolders] = useState<testrepo.Folder[]>([]);
  const [components, setComponents] = useState<Bucket[]>([]);
  const [statuses, setStatuses] = useState<string[]>([]);
  const [errors, setErrors] = useState<Partial<Record<ListName, string>>>({});

  useEffect(() => {
    if (!profileId) return;
    let cancelled = false;
    setErrors({});
    const failed = (list: ListName) => (e: unknown) => {
      if (!cancelled) setErrors((prev) => ({ ...prev, [list]: errMsg(e) }));
    };
    ListFolders(profileId)
      .then((f) => {
        if (!cancelled) setFolders(f ?? []);
      })
      .catch(failed("folder"));
    ListComponents(profileId)
      .then((c) => {
        if (!cancelled) setComponents(c ?? []);
      })
      .catch(failed("component"));
    ListStatuses(profileId)
      .then((s) => {
        if (!cancelled) setStatuses(s ?? []);
      })
      .catch(failed("status"));
    return () => {
      cancelled = true;
    };
  }, [profileId]);

  const loadError = LIST_NAMES.filter((n) => errors[n])
    .map((n) => `Could not load the ${n} list: ${errors[n]}`)
    .join(". ");
  return { folders, components, statuses, loadError };
}
