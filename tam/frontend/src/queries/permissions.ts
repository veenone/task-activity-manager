import { useCallback } from "react";
import { useQuery } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { GetPermissions } from "../api";
import { denial } from "../lib/permissions";
import { keys } from "./keys";

// usePermissions is what the last sync learned about the token. It is a
// local read, refreshed when a sync finishes.
export function usePermissions(profileId: string) {
  return useQuery({
    queryKey: keys.permissions(profileId),
    queryFn: () => call(() => GetPermissions(profileId)),
    enabled: !!profileId,
    retry: false,
  });
}

// useDenial answers, for a permission and optionally the issue it is acted
// on, why the action is not offered, or "" when it is. A component puts the
// answer in the control's title and disables the control when it is not
// empty. While the read is loading or has failed, everything is offered.
export function useDenial(profileId: string): (perm: string, issueKey?: string) => string {
  const { data } = usePermissions(profileId);
  return useCallback((perm: string, issueKey?: string) => denial(data, perm, issueKey), [data]);
}
