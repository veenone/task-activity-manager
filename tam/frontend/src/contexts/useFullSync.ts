import { useCallback } from "react";
import { useConfirm } from "@agile-suite/core";

// useFullSync is the Sync menu's Full sync and the native menu's, behind one
// confirmation. A full sync refetches every issue and can run for minutes on
// a large project, so it asks first, the way XTM's full resync does (#534).
// It does not ask while a sync cannot start: the answer would be thrown away.
export function useFullSync(runSync: (full: boolean) => Promise<void>, canSync: boolean): () => Promise<void> {
  const { confirm } = useConfirm();
  return useCallback(async () => {
    if (!canSync) return;
    const ok = await confirm({
      title: "Full sync",
      message:
        "A full sync fetches every issue again and drops the ones Jira no longer returns. " +
        "It can take a while on a large project. Continue?",
      confirmLabel: "Full sync",
      danger: false,
    });
    if (ok) await runSync(true);
  }, [canSync, confirm, runSync]);
}
