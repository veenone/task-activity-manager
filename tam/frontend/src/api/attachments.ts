import * as App from "../../wailsjs/go/main/App";

// Attachment is one file on an issue. pending and pendingId mark a file the
// journal still holds, for the reason Worklog carries them: the section
// draws Jira's files and TAM's own in one list and only the second kind can
// be taken back. path is where a pending file sits on this machine, and is
// empty for a file Jira holds.
export interface Attachment {
  id: string;
  name: string;
  author: string;
  authorName: string;
  created: string;
  size: number;
  mimeType: string;
  path: string;
  pending?: boolean;
  pendingId?: number;
}

// AttachmentDraft is the afterVal of a pending attachment's journal row.
export interface AttachmentDraft {
  path: string;
  name: string;
  size: number;
}

// ENTITY_ATTACHMENT is the journal entity of a file to upload to an issue.
// Its key is the issue key, its field the file's path, and its afterVal an
// AttachmentDraft as JSON.
export const ENTITY_ATTACHMENT = "attachment";

// ListAttachments is Jira's files for the issue followed by the ones the
// journal holds.
export const ListAttachments: (profileId: string, key: string) => Promise<Attachment[]> =
  App.ListAttachments as unknown as (profileId: string, key: string) => Promise<Attachment[]>;
// PickAttachmentFiles opens the system file dialog and answers the paths
// chosen, none when it was cancelled.
export const PickAttachmentFiles: () => Promise<string[]> = App.PickAttachmentFiles;
// AttachFiles journals the files. Nothing reaches Jira until Commit.
export const AttachFiles: (profileId: string, key: string, paths: string[]) => Promise<void> = App.AttachFiles;
// SaveAttachment asks where to save one of Jira's files and downloads it
// there. It answers the path written, or "" when the dialog was cancelled.
export const SaveAttachment: (profileId: string, key: string, id: string, name: string) => Promise<string> =
  App.SaveAttachment;
