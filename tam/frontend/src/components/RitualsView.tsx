import { useCallback, useEffect, useRef, useState } from "react";
import { announce, errMsg, useConfirm, useProfile } from "@agile-suite/core";
import {
  BrowserOpenURL, DeleteRitualDocument, EnsureSprintRituals, ForgetRitualPage, GetConfluenceConfig,
  LastRitualSync, ListBoards, ListBoardSprints, ResolveRitualConflict,
} from "../api";
import type {
  Board, ConfluenceConfig, Profile, RitualDocument, RitualRootMissing, RitualRootResult, RitualSyncResult, Settings, Sprint,
} from "../api";
import { useSync } from "../contexts/SyncContext";
import {
  CLOSED_EMPTY_SENTENCE, GONE_SENTENCE, NO_SCRUM_BOARD_SENTENCE, RITUAL_LABEL, RITUAL_ORDER, STATUS_LABEL,
  UNCONFIGURED_SENTENCE, conflictSentence, pendingLine, rootDoneSentence, rootMissingSentence, syncSummary,
} from "../lib/ritualText";
import { useModal } from "../modals";
import { RitualEditor } from "./ritual-editor/RitualEditor";
import type { RitualEditorHandle } from "./ritual-editor/RitualEditor";
import { RitualRootDialog } from "./RitualRootDialog";

// isWebURL is whether a configured base URL may become a link: the page link
// goes to BrowserOpenURL, and a base with any other scheme is not Confluence.
function isWebURL(value: string): boolean {
  try {
    const url = new URL(value);
    return url.protocol === "http:" || url.protocol === "https:";
  } catch {
    return false;
  }
}

// RitualsView reads nothing but tam.db. Opening it, picking a sprint, and
// editing a page make no Confluence call; Sync rituals is the one press that
// does, through the profile lock.
export function RitualsView() {
  const { activeId } = useProfile<Profile, Settings>();
  const { confirm } = useConfirm();
  const { runRitualsSync, runRitualRoot, running } = useSync();
  const { openModal } = useModal();
  const [config, setConfig] = useState<ConfluenceConfig | null>(null);
  const [boards, setBoards] = useState<Board[] | null>(null);
  const [boardId, setBoardId] = useState(0);
  const [sprints, setSprints] = useState<Sprint[]>([]);
  const [sprintId, setSprintId] = useState(0);
  const [docs, setDocs] = useState<RitualDocument[] | null>(null);
  const [selected, setSelected] = useState<string>("planning");
  const [lastSync, setLastSync] = useState("");
  const [result, setResult] = useState<RitualSyncResult | null>(null);
  const [error, setError] = useState("");
  const [viewTheirs, setViewTheirs] = useState(false);
  // True from the Sync press until its reload lands. The shared lock alone
  // releases before the reload remounts the editor on what the pass brought
  // back, and a keystroke in that gap would be saved against the version the
  // Sync just replaced, and refused.
  const [syncPressed, setSyncPressed] = useState(false);
  const editorRef = useRef<RitualEditorHandle>(null);
  // rootMissing is set when a Sync found the configured root page gone, and
  // opens the dialog. rootAt is the board and sprint that Sync was started
  // on, so the create and the Sync after it answer for those, not for
  // whatever the pickers moved to while the dialog was open.
  const [rootMissing, setRootMissing] = useState<RitualRootMissing | null>(null);
  const rootAt = useRef<Captured | null>(null);
  // sync() captures the board and sprint it was started on, and checks these
  // refs (kept in step with the pickers on every render) before applying
  // anything it read back. Without that, a user free to switch board or
  // sprint mid-sync would have the old sprint's result banner, last-synced
  // line, and reloaded documents silently overwrite whatever is on screen
  // once the request resolves.
  const boardIdRef = useRef(boardId);
  const sprintIdRef = useRef(sprintId);
  useEffect(() => { boardIdRef.current = boardId; sprintIdRef.current = sprintId; }, [boardId, sprintId]);

  useEffect(() => {
    let live = true;
    setConfig(null); setBoards(null); setBoardId(0); setError(""); setResult(null); setRootMissing(null);
    GetConfluenceConfig(activeId).then((c) => { if (live) setConfig(c); }).catch((e) => { if (live) setError(errMsg(e)); });
    ListBoards(activeId).then((all) => {
      if (!live) return;
      const scrum = all.filter((b) => b.type === "scrum");
      setBoards(scrum);
      setBoardId(scrum[0]?.id ?? 0);
    }).catch((e) => { if (live) { setBoards([]); setError(errMsg(e)); } });
    return () => { live = false; };
  }, [activeId]);

  useEffect(() => {
    let live = true;
    setSprints([]); setSprintId(0); setLastSync("");
    if (!boardId) return;
    ListBoardSprints(activeId, boardId).then((list) => {
      if (!live) return;
      // A draft sprint gets no ritual pages until Commit creates it.
      const held = list.filter((s) => !s.draft);
      setSprints(held);
      setSprintId(held.find((s) => s.state === "active")?.id ?? held[0]?.id ?? 0);
    }).catch((e) => { if (live) setError(errMsg(e)); });
    LastRitualSync(activeId, boardId).then((at) => { if (live) setLastSync(at); }).catch(() => {});
    return () => { live = false; };
  }, [activeId, boardId]);

  // capture remembers the board and sprint an action started on, and answers
  // whether they are still the ones on screen. Every action below that
  // applies something once an await resolves checks it first, so a switch
  // made meanwhile is never painted over with the old sprint's answer.
  const capture = () => {
    const startBoardId = boardId;
    const startSprintId = sprintId;
    return {
      boardId: startBoardId,
      sprintId: startSprintId,
      current: () => boardIdRef.current === startBoardId && sprintIdRef.current === startSprintId,
    };
  };
  type Captured = ReturnType<typeof capture>;

  const reloadDocs = async (at: Captured) => {
    if (!at.boardId || !at.sprintId) return;
    const list = await EnsureSprintRituals(activeId, at.boardId, at.sprintId);
    if (at.current()) setDocs(list);
  };

  useEffect(() => {
    let live = true;
    setDocs(null); setViewTheirs(false);
    if (!boardId || !sprintId) return;
    EnsureSprintRituals(activeId, boardId, sprintId)
      .then((list) => { if (live) setDocs(list); })
      .catch((e) => { if (live) setError(errMsg(e)); });
    return () => { live = false; };
  }, [activeId, boardId, sprintId]);

  // Controller ruling: the editor saves on unmount, so a save started for
  // the page the user just switched away from (a different sprint, or a
  // different board) can still resolve after the switch. Matching by
  // ritualType alone would let that stale save overwrite the same ritual in
  // whatever sprint's list is on screen now, so this matches boardId,
  // sprintId AND ritualType, and drops a saved document that matches
  // nothing in the current list rather than grafting it in.
  const onSaved = useCallback((saved: RitualDocument) => {
    setDocs((list) => {
      if (!list) return list;
      const idx = list.findIndex(
        (d) => d.boardId === saved.boardId && d.sprintId === saved.sprintId && d.ritualType === saved.ritualType,
      );
      if (idx === -1) return list;
      const next = list.slice();
      next[idx] = saved;
      return next;
    });
  }, []);

  async function sync() {
    // Captured at call time, not read again below: every write below is
    // gated on still matching what boardIdRef/sprintIdRef hold at the moment
    // it would apply, not on what this closure started with.
    const at = capture();
    setError(""); setResult(null);
    setSyncPressed(true);
    try {
      await editorRef.current?.flush();
      const res = await runRitualsSync(at.boardId);
      if (res.rootMissing) {
        // Nothing was synced, so there is no summary, no sync time, and
        // nothing to reload; the dialog is what happens next.
        if (at.current()) {
          rootAt.current = at;
          setRootMissing(res.rootMissing);
        }
        announce(rootMissingSentence(res.rootMissing.pageId));
        return;
      }
      if (at.current()) {
        setResult(res);
        setLastSync(res.syncedAt);
      }
      announce(syncSummary(res));
      await reloadDocs(at);
    } catch (e) {
      if (at.current()) setError(errMsg(e));
    } finally {
      setSyncPressed(false);
    }
  }

  // createRoot is the dialog's create (or adoption), through the rituals lock,
  // and applies the Sync Go ran on the new root the way sync() applies its
  // own. The editor stays locked from the press until the reload lands, for
  // the reason syncPressed exists. A failed reload is shown on the view, not
  // handed back to the dialog: the root is already set by then, and a dialog
  // offering Create again would only meet its own page as a taken title.
  async function createRoot(title: string, adopt: boolean): Promise<RitualRootResult> {
    const at = rootAt.current ?? capture();
    setSyncPressed(true);
    try {
      await editorRef.current?.flush();
      const out = await runRitualRoot(at.boardId, title, adopt);
      if (out.root.outcome === "created" || out.root.outcome === "adopted") {
        setConfig((c) => (c ? { ...c, rootPageID: out.root.pageId } : c));
        announce(rootDoneSentence(out.root));
        if (at.current()) {
          if (out.sync) {
            setResult(out.sync);
            setLastSync(out.sync.syncedAt);
          }
          setError(out.syncError);
        }
        try {
          await reloadDocs(at);
        } catch (e) {
          if (at.current()) setError(errMsg(e));
        }
      }
      return out;
    } finally {
      setSyncPressed(false);
    }
  }

  const closeRootDialog = () => {
    setRootMissing(null);
    rootAt.current = null;
  };

  async function resolve(doc: RitualDocument, choice: "mine" | "theirs") {
    const at = capture();
    if (choice === "theirs") {
      const ok = await confirm({
        title: "Take the Confluence version?",
        message: "Your local edits to this page will be replaced by the version in Confluence. This cannot be undone.",
        confirmLabel: "Take theirs",
        cancelLabel: "Keep editing",
        danger: true,
      });
      if (!ok) return;
    }
    setError("");
    try {
      await editorRef.current?.flush();
      await ResolveRitualConflict(activeId, doc.boardId, doc.sprintId, doc.ritualType, choice);
      announce(choice === "mine" ? "Kept your version. The next Sync pushes it." : "Took the Confluence version.");
      setViewTheirs(false);
      await reloadDocs(at);
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function forget(doc: RitualDocument) {
    const at = capture();
    setError("");
    try {
      await ForgetRitualPage(activeId, doc.boardId, doc.sprintId, doc.ritualType);
      announce("The page will be created again on the next Sync.");
      await reloadDocs(at);
    } catch (e) {
      setError(errMsg(e));
    }
  }

  async function removeLocal(doc: RitualDocument) {
    const at = capture();
    const ok = await confirm({
      title: `Remove the local ${RITUAL_LABEL[doc.ritualType] ?? doc.ritualType}?`,
      message: "This deletes TAM's copy of the page. A fresh page from the template takes its place. It cannot be undone.",
      confirmLabel: "Remove",
      cancelLabel: "Keep it",
      danger: true,
    });
    if (!ok) return;
    setError("");
    try {
      await DeleteRitualDocument(activeId, doc.boardId, doc.sprintId, doc.ritualType);
      announce("Removed the local copy.");
      await reloadDocs(at);
    } catch (e) {
      setError(errMsg(e));
    }
  }

  if (boards === null) {
    return <section className="backlog rituals-view" aria-label="Rituals"><p className="muted" role="status">Loading rituals</p></section>;
  }
  if (boards.length === 0) {
    return (
      <section className="backlog rituals-view" aria-label="Rituals">
        {error && <p className="error-text" role="alert">{error}</p>}
        <p className="muted">{NO_SCRUM_BOARD_SENTENCE}</p>
      </section>
    );
  }

  const configured = !!config && !!config.baseURL.trim() && !!config.spaceKey.trim() && !!config.rootPageID.trim();
  const demoSpace = config?.baseURL.trim().toLowerCase() === "demo";
  const sprint = sprints.find((s) => s.id === sprintId);
  // selected can point at a type the current sprint's list no longer has: it
  // never resets on a board/sprint switch, and a closed sprint's Ensure does
  // not backfill a type removeLocal just deleted. effectiveSelected falls
  // back to the first RITUAL_ORDER type the list actually has, so the nav
  // and the article pane agree on something real rather than both going
  // blank with no explanation.
  const docList = docs ?? [];
  const effectiveSelected = docList.some((d) => d.ritualType === selected)
    ? selected
    : (RITUAL_ORDER.find((t) => docList.some((d) => d.ritualType === t)) ?? selected);
  const doc = docList.find((d) => d.ritualType === effectiveSelected) ?? null;
  const base = config?.baseURL.trim().replace(/\/+$/, "") ?? "";
  const pageUrl = doc?.pageId && configured && !demoSpace && isWebURL(base)
    ? `${base}/pages/viewpage.action?pageId=${encodeURIComponent(doc.pageId)}`
    : "";
  const syncing = running === "rituals";

  return (
    <section className="backlog rituals-view" aria-label="Rituals">
      <div className="board-head rituals-toolbar">
        {boards.length > 1 ? (
          <label className="board-picker">
            <span>Board</span>
            <select aria-label="Ritual board" value={boardId} onChange={(e) => setBoardId(Number(e.target.value))} disabled={syncing}>
              {boards.map((b) => <option key={b.id} value={b.id}>{b.name}</option>)}
            </select>
          </label>
        ) : <h3 className="board-head-name">{boards[0].name}</h3>}
        {sprints.length > 0 && (
          <label className="board-picker">
            <span>Sprint</span>
            <select aria-label="Ritual sprint" value={sprintId} onChange={(e) => setSprintId(Number(e.target.value))} disabled={syncing}>
              {sprints.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}
            </select>
          </label>
        )}
        <button className="btn" onClick={() => void sync()} disabled={!configured || !boardId || syncing}>
          {syncing ? "Syncing rituals" : "Sync rituals"}
        </button>
        <span className="muted small">{configured ? pendingLine(docs ?? [], lastSync) : UNCONFIGURED_SENTENCE}</span>
      </div>

      {error && <p className="error-text" role="alert">{error}</p>}
      {result && (
        <div className="ritual-sync-result" role="status">
          <p>{syncSummary(result)}</p>
          {result.failed.length > 0 && (
            <ul>{result.failed.map((f) => <li key={`${f.sprintName}|${f.title}`}><strong>{f.title}</strong>: {f.reason}</li>)}</ul>
          )}
        </div>
      )}

      {docs === null ? (
        sprintId ? <p className="muted" role="status">Loading this sprint's rituals</p> : null
      ) : docs.length === 0 ? (
        <p className="muted">{sprint?.state === "closed" ? CLOSED_EMPTY_SENTENCE : "This sprint has no ritual pages yet."}</p>
      ) : (
        <div className="rituals-layout">
          <nav aria-label="Ritual documents" className="ritual-docs">
            <ul>
              {RITUAL_ORDER.map((type) => {
                const d = docs.find((x) => x.ritualType === type);
                if (!d) return null;
                return (
                  <li key={type}>
                    <button
                      className={`folder-item ritual-doc${effectiveSelected === type ? " folder-selected" : ""}`}
                      aria-current={effectiveSelected === type ? "page" : undefined}
                      onClick={() => { setSelected(type); setViewTheirs(false); }}
                    >
                      <span className="ritual-doc-label">{RITUAL_LABEL[type]}</span>
                      <span className={`ritual-chip ritual-chip-${d.status}`}>{STATUS_LABEL[d.status]}</span>
                    </button>
                  </li>
                );
              })}
            </ul>
          </nav>
          <article aria-label="Ritual page" className="ritual-page">
            {doc && (
              <>
                <div className="ritual-page-head">
                  <h3>{doc.title}</h3>
                  {pageUrl && <button className="btn btn-ghost" onClick={() => BrowserOpenURL(pageUrl)}>Open in Confluence</button>}
                </div>
                {doc.status === "conflict" && (
                  <div className="ritual-banner" role="alert">
                    <p>{conflictSentence(doc.conflictVersion)}</p>
                    <div className="row">
                      <button className="btn btn-ghost" aria-pressed={viewTheirs} onClick={() => setViewTheirs((v) => !v)}>
                        {viewTheirs ? "View mine" : "View theirs"}
                      </button>
                      <button className="btn" onClick={() => void resolve(doc, "mine")}>Keep mine</button>
                      <button className="btn btn-ghost" onClick={() => void resolve(doc, "theirs")}>Take theirs</button>
                    </div>
                  </div>
                )}
                {doc.status === "gone" && (
                  <div className="ritual-banner" role="alert">
                    <p>{GONE_SENTENCE}</p>
                    <div className="row">
                      <button className="btn" onClick={() => void forget(doc)}>Recreate on next Sync</button>
                      <button className="btn btn-ghost" onClick={() => void removeLocal(doc)}>Remove local copy</button>
                    </div>
                  </div>
                )}
                {viewTheirs && doc.status === "conflict" ? (
                  <RitualEditor key={`theirs:${doc.ritualType}:${doc.conflictVersion}`} profileId={activeId} doc={doc}
                    body={doc.conflictBody} readOnly pageUrl={pageUrl} />
                ) : (
                  // Keyed by version and page id, so a Sync that pulls, pushes or
                  // creates remounts the editor on the new body, and a local save,
                  // which changes neither, does not.
                  // Locked while a Sync runs and until its reload lands, so no
                  // keystroke is typed over a page the pass is replacing.
                  <RitualEditor key={`${doc.boardId}:${doc.sprintId}:${doc.ritualType}:${doc.version}:${doc.pageId}`}
                    ref={editorRef} profileId={activeId} doc={doc} pageUrl={pageUrl} onSaved={onSaved}
                    locked={syncing || syncPressed} />
                )}
              </>
            )}
          </article>
        </div>
      )}
      {rootMissing && (
        <RitualRootDialog
          missing={rootMissing}
          create={createRoot}
          onDone={closeRootDialog}
          onClose={closeRootDialog}
          onOpenProfiles={() => openModal("profiles")}
        />
      )}
    </section>
  );
}
