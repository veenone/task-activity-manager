import { useMemo, useState } from "react";
import type { ReactNode } from "react";
import { FORMAT_LABEL, RichText, detectFormat, errMsg, toPlainText, useNotice } from "@agile-suite/core";
import type { RichFormat } from "@agile-suite/core";
import { BrowserOpenURL } from "../api";
import type { Issue, IssueComment, IssueDetail, Link, SprintOption } from "../api";
import { useIssueDetail, useLinkedTests } from "../queries/issues";
import { useDiscardById } from "../queries/pending";
import { formatWhen } from "../lib/format";
import { useSync } from "../contexts/SyncContext";
import { TypeChip } from "./TypeChip";
import { IssueKeyLink, browseUrl } from "./IssueKeyLink";
import { NewIssueModal } from "./NewIssueModal";
import { useSubtaskType } from "../queries/people";
import { EditableFields, descriptionFormat } from "./EditableFields";
import { SprintField } from "./SprintField";
import { ActivityTab } from "./ActivityTab";
import { AddLinkForm } from "./AddLinkForm";

// Section is one collapsible block of the panel. XTM's detail sidebar stacks
// its sections under uppercase headings rather than hiding them behind tabs,
// so a reader scrolls one column instead of hunting three; this mirrors that,
// with everything but the fields closed on open so the panel still starts
// short.
function Section({
  title,
  count,
  open,
  onToggle,
  action,
  children,
}: {
  title: string;
  count?: number;
  open: boolean;
  onToggle: () => void;
  action?: ReactNode;
  children: ReactNode;
}) {
  const id = `detail-section-${title.toLowerCase().replace(/\s+/g, "-")}`;
  return (
    <section className="detail-section">
      <h4 className="detail-section-title">
        <button
          type="button"
          className="detail-section-toggle"
          aria-expanded={open}
          aria-controls={id}
          onClick={onToggle}
        >
          <span className="detail-section-chevron" aria-hidden="true">{open ? "▾" : "▸"}</span>
          {title}
          {count !== undefined && <span className="detail-section-count">{count}</span>}
        </button>
        {action}
      </h4>
      <div id={id} hidden={!open}>
        {children}
      </div>
    </section>
  );
}

const DEFAULT_WIDTH = 352;
const MIN_WIDTH = 300;
const MAX_WIDTH = 900;
const WIDTH_KEY = "tam.detailPanelWidth";

// The width is a per-machine reading preference, so it lives in the WebView's
// own storage rather than in the shared settings both apps read. Storage can
// throw (a locked-down WebView, a cleared profile), and a panel that cannot
// remember its width must still open at a sensible one.
function readStoredWidth(): string {
  try {
    return window.localStorage.getItem(WIDTH_KEY) ?? "";
  } catch {
    return "";
  }
}

function storeWidth(w: number) {
  try {
    window.localStorage.setItem(WIDTH_KEY, String(w));
  } catch {
    /* a width that cannot be remembered is not worth failing a drag over */
  }
}

interface Props {
  profileId: string;
  issue: Issue;
  jiraUrl?: string;
  // sprints are every open sprint the panel can offer: the board's own list
  // from BoardsView, or the profile-wide list from BacklogView and
  // EpicsView. With a list, however empty, Sprint is a choice that journals
  // a move; undefined keeps the panel's read-only fact, which is what a
  // caller passes while its own sprint query is still in flight.
  sprints?: SprintOption[];
  // emptyNote is what SprintField says beside its select when sprints is
  // empty. Only a caller reading the profile-wide list is in a position to
  // say why: a board's own empty list can mean a kanban board's disabled
  // sprint query, a scrum board still loading, or one whose sprints are all
  // closed, and none of those is "this profile has never synced a board".
  emptyNote?: string;
  onClose: () => void;
}

// IssueDetailPanel shows one issue beside the grid. The grid row's fields
// render at once; the description, links, and linked tests load through the
// backend's detail cache. Nothing here writes; the actions arrive in plan 1b.
export function IssueDetailPanel({ profileId, issue, jiraUrl, sprints, emptyNote, onClose }: Props) {
  // Fields open, everything else closed: the panel starts on what a reader
  // came for and lets them reach the rest without leaving the column.
  const [open, setOpen] = useState<Record<string, boolean>>({ fields: true });
  const toggle = (id: string) => setOpen((cur) => ({ ...cur, [id]: !cur[id] }));
  const detail = useIssueDetail(profileId, issue.key);
  const tests = useLinkedTests(profileId, issue.key);
  const discardLink = useDiscardById(profileId);
  const { notice } = useNotice();
  // A sync or commit running elsewhere must not race a save: the row refresh
  // that follows either one can overwrite an edit made while it was in flight.
  const { status } = useSync();
  const busy = status !== "idle";
  const subtaskType = useSubtaskType(profileId);
  const [drafting, setDrafting] = useState(false);
  // The comment chips read the description's syntax, which the toggle in
  // EditableFields can change; this is what carries that change up here.
  const [, bumpFormat] = useState(0);
  // The panel's width is the reader's, not the layout's: a 352px column is
  // right for a glance and wrong for a long description. Kept per machine so
  // it survives a restart, the way XTM's does.
  const [width, setWidth] = useState(() => {
    const saved = Number(readStoredWidth());
    return Number.isFinite(saved) && saved >= MIN_WIDTH ? Math.min(saved, MAX_WIDTH) : DEFAULT_WIDTH;
  });

  // The panel is anchored to the right, so dragging left widens it.
  function startResize(e: React.MouseEvent) {
    e.preventDefault();
    const startX = e.clientX;
    const startW = width;
    const onMove = (ev: MouseEvent) =>
      setWidth(Math.min(MAX_WIDTH, Math.max(MIN_WIDTH, startW - (ev.clientX - startX))));
    const onUp = () => {
      window.removeEventListener("mousemove", onMove);
      window.removeEventListener("mouseup", onUp);
      document.body.style.cursor = "";
      document.body.style.userSelect = "";
      setWidth((w) => {
        storeWidth(w);
        return w;
      });
    };
    document.body.style.cursor = "col-resize";
    document.body.style.userSelect = "none";
    window.addEventListener("mousemove", onMove);
    window.addEventListener("mouseup", onUp);
  }
  // The project key comes from the issue on screen (a ruling), not from the
  // profile: it is the prefix of its own key, and it is all a renderer needs
  // to tell one of this project's keys from a word with a number after it.
  const projectKey = issue.key.slice(0, issue.key.lastIndexOf("-"));
  // A key only becomes a button where there is an instance to open it on.
  // browseUrl answers "" for a demo profile, a profile with no URL and a
  // draft, and a button that opens nothing is worse than plain text.
  const openIssueKey = browseUrl(jiraUrl, issue.key)
    ? (key: string) => {
        const url = browseUrl(jiraUrl, key);
        if (url) BrowserOpenURL(url);
      }
    : undefined;
  // A link never navigates the WebView: that would take the whole app away
  // from TAM, the same reason IssueKeyLink opens in the user's own browser.
  const openLink = (url: string) => BrowserOpenURL(url);
  const descFormat = descriptionFormat(profileId, issue.key, detail.data?.description ?? "");

  // Jira allows a sub-task under any standard issue, and under neither an
  // epic nor another sub-task. A draft parent is fine: Commit creates it
  // first and the sub-task after it has a real key.
  const canHoldSubtasks =
    issue.type !== "epic" &&
    issue.type !== "subtask" &&
    (subtaskType.data ?? "") !== "";

  return (
    <aside className="detail-panel" style={{ width }} aria-labelledby="detail-title">
      <div className="detail-resizer" onMouseDown={startResize} title="Drag to resize" />
      {/* The dark instrument bar XTM's detail sidebar carries: the key and
          its status on the left, the actions on the right. */}
      <div className="detail-head">
        <div className="detail-head-id">
          <h2 id="detail-title" className="detail-key">
            <IssueKeyLink jiraUrl={jiraUrl} issueKey={issue.key} />
          </h2>
          {issue.draft ? (
            <span className="chip chip-draft">Draft</span>
          ) : (
            issue.status && <span className="detail-head-status">{issue.status}</span>
          )}
        </div>
        <div className="detail-head-actions">
          <TypeChip type={issue.type} subtaskLabel={subtaskType.data} />
          <button type="button" className="btn btn-ghost detail-close" onClick={onClose} aria-label="Close" title="Close">✕</button>
        </div>
      </div>

      <div className="detail-body">

      {issue.draft && (
        <p className="muted small detail-note">Commit creates this issue in Jira and gives it a real key.</p>
      )}
      {/* D5: the heading renders the summary's inline code and links and
          nothing else. Jira DC does not wiki-render this field, so a mark
          in it must read exactly as it was typed. */}
      <p className="detail-summary">
        <RichText inline text={issue.summary} projectKey={projectKey} onOpenLink={openLink} onIssueKey={openIssueKey} />
      </p>
      <dl className="detail-fields">
        <dt>Sprint</dt>
        <dd>
          {/* A sub-task is not a card a board carries either, the same
              reason an epic gets no picker in the New issue dialog: it has
              no sprint of its own, it follows its parent's, and the Agile
              move endpoint refuses one aimed at it. So it keeps the
              read-only fact regardless of what sprints the caller has,
              which is what this panel already falls back to when it has no
              list at all. */}
          {sprints && issue.type !== "subtask"
            ? <SprintField profileId={profileId} issue={issue} sprints={sprints} busy={busy} emptyNote={emptyNote} />
            : issue.sprintName || "-"}
        </dd>
        <dt>Updated</dt><dd>{formatWhen(issue.updated) || "-"}</dd>
        <dt>Reporter</dt><dd>{issue.reporter || "-"}</dd>
      </dl>

      {/* No Refresh of its own (D6): the shell's one Refresh covers the whole
          view, and this one went through GetIssueDetail, so it did nothing
          while the cached detail was fresh and nothing at all under
          detail_cache_minutes = 0. The Retry below is a different thing: it
          follows a failed read, where there is nothing cached to serve. */}
      <Section
        title="Fields"
        open={open.fields ?? false}
        onToggle={() => toggle("fields")}
      >
        {detail.isError && (
          <p className="error-text" data-testid="detail-error">
            Could not load the details: {detail.error.message}{" "}
            <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()}>Retry</button>
          </p>
        )}
        <EditableFields
          profileId={profileId}
          issue={issue}
          description={detail.data?.description ?? ""}
          descriptionReady={detail.isSuccess}
          busy={busy}
          projectKey={projectKey}
          onOpenLink={openLink}
          onIssueKey={openIssueKey}
          onSyntaxPicked={() => bumpFormat((n) => n + 1)}
        />
      </Section>

      {canHoldSubtasks && (
        <div className="detail-actions">
          <button
            type="button"
            className="btn"
            onClick={() => setDrafting(true)}
            title={`Draft a ${subtaskType.data} under ${issue.key}`}
          >
            + {subtaskType.data}
          </button>
        </div>
      )}

      <Section
        title="Comments"
        count={detail.data?.commentTotal}
        open={open.comments ?? false}
        onToggle={() => toggle("comments")}
        // TODO 1: a detail served from the cache while Jira is unreachable
        // is still the whole panel, so it says how old it is rather than
        // reading as if it had just been fetched.
        action={
          detail.data?.fetchedAt
            ? <span className="muted small detail-section-note">cached {formatWhen(detail.data.fetchedAt)}</span>
            : undefined
        }
      >
        {detail.isPending ? (
          <p className="muted">Loading comments</p>
        ) : detail.isError ? (
          <p className="error-text" data-testid="comments-error">
            Could not load the comments: {detail.error.message}{" "}
            <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()}>Retry</button>
          </p>
        ) : (
          <Comments
            key={issue.key}
            detail={detail.data}
            descFormat={descFormat}
            projectKey={projectKey}
            onOpenLink={openLink}
            onIssueKey={openIssueKey}
          />
        )}
      </Section>

      <Section
        title="Links"
        count={detail.data?.links.length}
        open={open.links ?? false}
        onToggle={() => toggle("links")}
      >
          {detail.isPending ? (
            <p className="muted">Loading links</p>
          ) : detail.isError ? (
            <p className="error-text" data-testid="links-error">
              Could not load the links: {detail.error.message}{" "}
              <button type="button" className="btn btn-ghost" onClick={() => void detail.refetch()}>Retry</button>
            </p>
          ) : detail.data.links.length === 0 ? (
            <p className="muted">No links.</p>
          ) : (
            <LinkGroups
              links={detail.data.links}
              discarding={discardLink.isPending}
              onDiscard={(id) =>
                discardLink.mutate(
                  { id, key: issue.key },
                  { onError: (e) => void notice({ title: "Discard failed", message: errMsg(e), tone: "error" }) },
                )
              }
            />
          )}
          <AddLinkForm profileId={profileId} issueKey={issue.key} onAdded={() => void detail.refetch()} />
      </Section>

      <Section
        title="Covered by tests"
        count={tests.data?.length}
        open={open.tests ?? false}
        onToggle={() => toggle("tests")}
        action={
          <button type="button" className="btn btn-ghost detail-section-action" onClick={() => void tests.refetch()} disabled={tests.isFetching}>
            {tests.isFetching ? "Refreshing" : "Refresh"}
          </button>
        }
      >
          {tests.data && tests.data.length > 0 && (
            <p className="muted small">via XTM, link: {tests.data[0].linkType}</p>
          )}
          {tests.isPending ? (
            <p className="muted">Loading tests</p>
          ) : tests.isError ? (
            <p className="error-text" data-testid="tests-error">
              Could not load the linked tests: {tests.error.message}{" "}
              <button type="button" className="btn btn-ghost" onClick={() => void tests.refetch()}>Retry</button>
            </p>
          ) : tests.data.length === 0 ? (
            <p className="muted">No linked tests.</p>
          ) : (
            <ul className="linked-list">
              {tests.data.map((t) => (
                <li key={t.key} className="linked-row">
                  <span className="accent-text linked-key">{t.key}</span>
                  <span>{toPlainText(t.summary, "summary")}</span>
                </li>
              ))}
            </ul>
          )}
      </Section>

      <Section title="Activity" open={open.activity ?? false} onToggle={() => toggle("activity")}>
        <ActivityTab profileId={profileId} issueKey={issue.key} />
      </Section>
      </div>

      {drafting && (
        <NewIssueModal
          onClose={() => setDrafting(false)}
          initialType="subtask"
          lockType
          parentKey={issue.key}
          parentSummary={issue.summary}
          onCreated={() => setDrafting(false)}
        />
      )}
    </aside>
  );
}

// INLINE_COMMENTS is how many of the newest a closed-by-default section
// shows before Show all; a long thread would otherwise be the whole panel.
const INLINE_COMMENTS = 5;

// initials are the avatar's whole content: a name's first and last letter,
// which is what the mockup draws and all TAM has, since Jira's avatars are
// images behind the instance's own authentication.
function initials(name: string): string {
  const parts = name.split(/\s+/).filter((p) => p !== "");
  if (parts.length === 0) return "?";
  const last = parts.length > 1 ? parts[parts.length - 1][0] : "";
  return `${parts[0][0]}${last}`.toUpperCase();
}

interface CommentProps {
  descFormat: RichFormat;
  projectKey: string;
  onOpenLink: (url: string) => void;
  onIssueKey?: (key: string) => void;
}

// Comments is the read-only thread, oldest first the way Jira reads it, with
// only the newest few drawn until the reader asks for the rest.
function Comments({ detail, ...rest }: { detail: IssueDetail } & CommentProps) {
  const [showAll, setShowAll] = useState(false);
  const all = detail.comments;

  if (all.length === 0) {
    // Truncated with nothing in hand is a failed read, never an issue with
    // no comments: an issue that genuinely has none answers a total of 0
    // with truncated false. Calling that "No comments." would be TAM
    // inventing an answer Jira never gave.
    return detail.commentsTruncated ? (
      <p className="error-text">The comments could not be read. Open in Jira to see them.</p>
    ) : (
      <p className="muted">No comments.</p>
    );
  }

  const shown = showAll ? all : all.slice(-INLINE_COMMENTS);
  return (
    <>
      {detail.commentsTruncated && (
        <p className="muted small">
          Showing {all.length} of {detail.commentTotal} comments. Open in Jira for the rest.
        </p>
      )}
      {shown.length < all.length && (
        // The count is what this button actually reveals, which is the
        // issue's total only when nothing was truncated; promising 812 and
        // drawing 500 would be the same lie the sentence above avoids.
        <button type="button" className="btn btn-ghost" onClick={() => setShowAll(true)}>Show all {all.length}</button>
      )}
      <ul className="comment-list">
        {shown.map((c) => (
          <li key={c.id}>
            <CommentCard comment={c} {...rest} />
          </li>
        ))}
      </ul>
    </>
  );
}

// CommentCard reads one comment in its own syntax, and says so only when
// that is not the syntax the description above it is read in: a chip on
// every comment of an all-Markdown issue would be noise.
function CommentCard({ comment, descFormat, projectKey, onOpenLink, onIssueKey }: { comment: IssueComment } & CommentProps) {
  const format = useMemo(() => detectFormat(comment.body).format, [comment.body]);
  const name = comment.authorName || comment.author || "Unknown user";
  const edited = comment.updated !== "" && comment.updated !== comment.created;
  return (
    <article className="comment-card">
      <div className="comment-head">
        <span className="comment-avatar" aria-hidden="true">{initials(name)}</span>
        <h5 className="comment-author">{name}</h5>
        {format !== descFormat && <span className="chip chip-label">{FORMAT_LABEL[format]}</span>}
        {/* Only the restriction's value reaches TAM, so the chip names it
            and calls it nothing else: Jira restricts a comment to a role or
            to a group and does not say which one this is. */}
        {comment.restriction !== "" && <span className="chip chip-restricted">Restricted: {comment.restriction}</span>}
        <span className="muted small comment-when">{formatWhen(comment.created)}</span>
        {edited && <span className="muted small">edited</span>}
      </div>
      <RichText text={comment.body} format={format} projectKey={projectKey} onOpenLink={onOpenLink} onIssueKey={onIssueKey} />
    </article>
  );
}

// LinkGroups lists links grouped by type, then direction, in the order the
// store returns them, and marks a link still pending a Commit.
function LinkGroups({ links, onDiscard, discarding }: { links: Link[]; onDiscard: (id: number) => void; discarding: boolean }) {
  const groups = new Map<string, Link[]>();
  for (const l of links) {
    const k = `${l.type} (${l.direction})`;
    groups.set(k, [...(groups.get(k) ?? []), l]);
  }
  return (
    <div className="link-groups">
      {[...groups.entries()].map(([label, items]) => (
        <div key={label} className="link-group">
          <h3 className="link-group-title">{label.replace(/ \((inward|outward)\)$/, "")} <span className="muted small">{label.match(/\((inward|outward)\)$/)?.[1]}</span></h3>
          <ul className="linked-list">
            {items.map((l) => (
              <li key={`${l.type}-${l.direction}-${l.key}`} className="linked-row">
                <span className="accent-text linked-key">{l.key}</span>
                <span>{toPlainText(l.summary, "summary")}</span>
                <span className="muted small">{l.issueType}</span>
                {l.pending && (
                  <>
                    <span className="pending-dot" role="img" aria-label="Pending changes" />
                    <span className="muted small">pending</span>
                    <button type="button" className="btn btn-discard btn-discard-row" aria-label={`Discard link to ${l.key}`} disabled={discarding} onClick={() => onDiscard(l.pendingId ?? 0)}>
                      <span className="discard-mark" aria-hidden="true">✕</span>Discard
                    </button>
                  </>
                )}
              </li>
            ))}
          </ul>
        </div>
      ))}
    </div>
  );
}
