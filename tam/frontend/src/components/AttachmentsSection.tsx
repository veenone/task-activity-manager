import { useState } from "react";
import { errMsg, useNotice } from "@agile-suite/core";
import type { Attachment } from "../api";
import { useAttachFiles, useAttachments, useSaveAttachment } from "../queries/attachments";
import { useDiscardById } from "../queries/pending";
import { day, fileSize } from "../lib/format";
import { PERM_CREATE_ATTACHMENTS } from "../api/permissions";
import { useDenial } from "../queries/permissions";

interface Props {
  profileId: string;
  issueKey: string;
  // open is the section's own state. Jira's files are read when the section
  // is expanded and not before.
  open: boolean;
}

// AttachmentsSection is the issue's files: what Jira holds, what the journal
// holds, and the button that adds more. A picked file is journalled like any
// other change and uploaded on Commit, so until then the list shows where it
// is on this machine and offers to discard it.
export function AttachmentsSection({ profileId, issueKey, open }: Props) {
  const files = useAttachments(profileId, issueKey, open);
  const attach = useAttachFiles(profileId);
  const save = useSaveAttachment(profileId);
  const discard = useDiscardById(profileId);
  const denied = useDenial(profileId)(PERM_CREATE_ATTACHMENTS, issueKey);
  const { notice } = useNotice();
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");

  async function onAttach() {
    setError("");
    try {
      await attach.mutateAsync(issueKey);
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function onSave(f: Attachment) {
    setError("");
    setSaved("");
    try {
      const path = await save.mutateAsync({ key: issueKey, id: f.id, name: f.name });
      if (path) setSaved(`Saved to ${path}`);
    } catch (e) {
      setError(errMsg(e));
    }
  }

  const list = files.data ?? [];
  return (
    <>
      {files.isPending ? (
        <p className="muted">Loading the attachments</p>
      ) : files.isError ? (
        <p className="error-text">
          Could not load the attachments: {files.error.message}{" "}
          <button type="button" className="btn btn-ghost" onClick={() => void files.refetch()}>Retry</button>
        </p>
      ) : list.length === 0 ? (
        <p className="muted">No attachments.</p>
      ) : (
        <ul className="linked-list">
          {list.map((f) => (
            <li key={f.pendingId ? `pending-${f.pendingId}` : f.id} className="linked-row attachment-row">
              <span className="attachment-name" title={f.pending ? f.path : f.name}>{f.name}</span>
              <span className="muted small">{fileSize(f.size)}</span>
              {f.pending ? (
                <>
                  <span className="pending-dot" role="img" aria-label="Pending changes" />
                  <span className="muted small">pending</span>
                  <button
                    type="button"
                    className="btn btn-discard btn-discard-row"
                    aria-label={`Discard ${f.name}`}
                    disabled={discard.isPending}
                    onClick={() =>
                      discard.mutate(
                        { id: f.pendingId ?? 0, key: issueKey },
                        { onError: (e) => void notice({ title: "Discard failed", message: errMsg(e), tone: "error" }) },
                      )
                    }
                  >
                    <span className="discard-mark" aria-hidden="true">✕</span>Discard
                  </button>
                </>
              ) : (
                <>
                  <span className="muted small">{[f.authorName || f.author, day(f.created)].filter(Boolean).join(", ")}</span>
                  <button
                    type="button"
                    className="btn btn-ghost attachment-save"
                    aria-label={`Save ${f.name}`}
                    disabled={save.isPending}
                    onClick={() => void onSave(f)}
                  >
                    Save
                  </button>
                </>
              )}
            </li>
          ))}
        </ul>
      )}

      {saved && <p className="muted small" role="status">{saved}</p>}
      {error && <p className="error-text small" role="alert">{error}</p>}
      <div className="edit-actions">
        <button type="button" className="btn" disabled={attach.isPending || !!denied} title={denied || undefined} onClick={() => void onAttach()}>Attach files</button>
        <span className="muted small">{denied || "Journaled now, uploaded on Commit."}</span>
      </div>
    </>
  );
}

// attachmentLine is the one phrase the Pending changes dialog reads a
// journalled file by: "trace.log (2 KB)".
export function attachmentLine(name: string, size: number): string {
  return `${name} (${fileSize(size)})`;
}
