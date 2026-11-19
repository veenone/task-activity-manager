import { useMutation, useQuery, useQueryClient } from "@tanstack/react-query";
import { call } from "@agile-suite/core";
import { AttachFiles, ListAttachments, PickAttachmentFiles, SaveAttachment } from "../api";
import { invalidateWrites } from "./invalidate";
import { keys } from "./keys";

// useAttachments is one issue's files, Jira's and the pending ones together.
// open makes it fetch on expand, the way the work log does: the section is
// closed by default and a closed one has no reason to cost a round trip.
export function useAttachments(profileId: string, key: string, open: boolean) {
  return useQuery({
    queryKey: keys.attachments(profileId, key),
    queryFn: () => call(() => ListAttachments(profileId, key)),
    enabled: !!profileId && !!key && open,
    retry: false,
  });
}

// useAttachFiles opens the file dialog and journals what was picked against
// the issue. It resolves to how many files that was, none when the dialog
// was cancelled.
export function useAttachFiles(profileId: string) {
  const qc = useQueryClient();
  return useMutation({
    mutationFn: async (key: string) => {
      const paths = await call(() => PickAttachmentFiles());
      if (paths.length === 0) return 0;
      await call(() => AttachFiles(profileId, key, paths));
      return paths.length;
    },
    onSuccess: (_, key) => invalidateWrites(qc, profileId, key),
  });
}

// useSaveAttachment downloads one of Jira's files to where the save dialog
// says. It resolves to the path written, or "" when the dialog was cancelled.
export function useSaveAttachment(profileId: string) {
  return useMutation({
    mutationFn: ({ key, id, name }: { key: string; id: string; name: string }) =>
      call(() => SaveAttachment(profileId, key, id, name)),
  });
}
