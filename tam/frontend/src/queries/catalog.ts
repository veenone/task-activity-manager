import { useQuery } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { GetComponents, GetVersions } from "../api";
import { keys } from "./keys";

// useVersions and useComponents read the catalog the last sync cached. They
// are local reads, refreshed when a sync finishes.
export function useVersions(profileId: string) {
  return useQuery({
    queryKey: keys.versions(profileId),
    queryFn: () => call(() => GetVersions(profileId)),
    enabled: !!profileId,
  });
}

export function useComponents(profileId: string) {
  return useQuery({
    queryKey: keys.components(profileId),
    queryFn: () => call(() => GetComponents(profileId)),
    enabled: !!profileId,
  });
}
