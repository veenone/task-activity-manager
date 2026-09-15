import { useCallback, useEffect, useImperativeHandle, useMemo, useRef, useState } from "react";
import type { KeyboardEvent, Ref } from "react";
import { EditorContent, useEditor, useEditorState } from "@tiptap/react";
import type { ChainedCommands, Editor } from "@tiptap/core";
import { announce, errMsg } from "@agile-suite/core";
import { BrowserOpenURL, SaveRitualBody, StandupEntry } from "../../api";
import type { RitualDocument } from "../../api";
import { parseStorage } from "../../lib/storage/parse";
import { serializeStorage } from "../../lib/storage/serialize";
import { sanitizeHtml } from "../../lib/sanitizeHtml";
import { findTodaysEntry, localDay } from "../../lib/standupLog";
import { DAILY_LOG_MISSING, ENTRY_EXISTS, ENTRY_UNREADABLE, READ_ONLY_SENTENCE, editorStatusLine } from "../../lib/ritualText";
import { RitualEditorContext } from "./context";
import { ritualExtensions } from "./extensions";

export const SAVE_DELAY_MS = 800;

export interface RitualEditorHandle {
  flush: () => Promise<void>;
  editor: Editor | null;
}

interface Props {
  profileId: string;
  // The editor rebuilds itself (a fresh TipTap instance, cursor and undo
  // history reset) only when `editable` changes, never when `doc`/`body`
  // do: a local save hands the saved row back through onSaved, and if that
  // alone tore the editor down, every save would cost the caret, the undo
  // stack, and any keystroke typed while the save was still in flight. That
  // makes identity the caller's job: whoever renders RitualEditor must give
  // it a React `key` that changes whenever the page does (board, sprint,
  // ritual type, version, page id), so a real page change unmounts and
  // remounts rather than quietly reusing the old page's editor.
  doc: RitualDocument;
  // body defaults to doc.body; the conflict view passes conflictBody with readOnly.
  body?: string;
  readOnly?: boolean;
  pageUrl?: string;
  onSaved?: (doc: RitualDocument) => void;
  saveDelayMs?: number;
  ref?: Ref<RitualEditorHandle>;
}

type Run = (chain: ChainedCommands) => ChainedCommands;

// RitualEditor edits one ritual page. It saves locally, 800 ms after an edit
// that actually changes the page's serialized text, on Ctrl+S, on flush, and
// on unmount; it never talks to Confluence. Whether something counts as an
// edit is decided by comparing serialized text to what was last saved, never
// by which DOM event carried it: a mouse click on a task checkbox, a
// context-menu Cut, or a spellcheck correction change the document exactly
// as a keystroke does, and none of them reliably raise one. Loading content,
// and anything a plugin does on load, serialize back to the same text the
// page already had, so opening a page never makes it unsynced.
export function RitualEditor({ profileId, doc, body = doc.body, readOnly = false, pageUrl, onSaved, saveDelayMs = SAVE_DELAY_MS, ref }: Props) {
  const parsed = useMemo(() => parseStorage(body), [body]);
  const editable = !readOnly && parsed.ok;
  const extensions = useMemo(() => ritualExtensions(), []);
  const pending = useRef<string | null>(null);
  // The text a successful save last actually persisted. This is the store's
  // side of the "does the screen match the store" question `onUpdate` and
  // `reconcile` both ask.
  const lastSaved = useRef<string>("");
  // The text the save currently in flight was sent with, or null when no
  // save is running. `onUpdate` compares against this, not `lastSaved`,
  // while a save is running: comparing against `lastSaved` alone let an
  // undo back to the stored text, arriving after the network call for a
  // newer edit had already started, read as "nothing changed" and clear
  // `pending` — and then the in-flight save would still land, moving the
  // store to the newer text with nothing left to correct it, so store and
  // screen would disagree until the next unrelated edit happened to notice.
  const inFlightText = useRef<string | null>(null);
  const editorRef = useRef<Editor | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  // Saves run one at a time, in the order they were asked for: a call
  // chains onto whatever is already in flight rather than firing beside it,
  // so Ctrl+S landing between two keystrokes can never let an older body or
  // an older onSaved arrive after a newer one.
  const inFlight = useRef<Promise<void>>(Promise.resolve());
  const [saving, setSaving] = useState(false);
  const [savedAt, setSavedAt] = useState("");
  const [saveError, setSaveError] = useState("");
  const [notice, setNotice] = useState("");

  const save = useCallback((): Promise<void> => {
    clearTimeout(timer.current);
    // Runs after a successful save, before `inFlightText` is cleared. The
    // store (`lastSaved`) and the screen can disagree even here, when
    // `onUpdate`'s own in-flight comparison already caught most such drift
    // (see the comment on `inFlightText`); this is the backstop, not the
    // primary mechanism, so it only acts when nothing is already queued and
    // it always saves what the editor currently holds, never the text that
    // was just attempted. Success only: a failed save gets different
    // handling in `runOne`'s `catch`, not this.
    const reconcile = () => {
      const current = editorRef.current;
      if (!current || current.isDestroyed || pending.current !== null) return;
      const now = serializeStorage(current.getJSON());
      if (now !== lastSaved.current) {
        pending.current = now;
        void saveRef.current();
      }
    };
    const runOne = async () => {
      const next = pending.current;
      if (next === null) return;
      pending.current = null;
      inFlightText.current = next;
      setSaving(true);
      setSaveError("");
      try {
        const saved = await SaveRitualBody(profileId, doc.boardId, doc.sprintId, doc.ritualType, next);
        lastSaved.current = next;
        setSavedAt(new Date().toISOString());
        onSaved?.(saved);
        inFlightText.current = null;
        reconcile();
      } catch (e) {
        inFlightText.current = null;
        setSaveError(errMsg(e));
        // A failed save never retries itself. Calling save() again here,
        // even once, means a save that keeps failing (a purged page, a
        // permanently rejected write) spins as fast as promises resolve:
        // measured at 20 calls in 300 ms against a mock failing after 5 ms,
        // and never returning at all against one that rejects immediately,
        // since the retry loop starves the event loop's timers. Instead
        // this only makes sure the *next* real trigger — the debounce timer
        // from another edit, Ctrl+S, flush, or unmount — has the right text
        // queued: the editor's current text, if it still differs from what
        // is actually stored (`lastSaved`, unchanged by a failed save), or
        // nothing at all if the editor already reads back to the stored
        // text, so a page the user undid back to what was already saved
        // does not queue a save that would change nothing.
        const current = editorRef.current;
        if (current && !current.isDestroyed) {
          const now = serializeStorage(current.getJSON());
          pending.current = now !== lastSaved.current ? now : null;
        }
      } finally {
        setSaving(false);
      }
    };
    const chained = inFlight.current.then(runOne);
    inFlight.current = chained;
    return chained;
  }, [profileId, doc.boardId, doc.sprintId, doc.ritualType, onSaved]);

  const saveRef = useRef(save);
  useEffect(() => { saveRef.current = save; }, [save]);

  const editor = useEditor({
    extensions,
    content: parsed.ok ? parsed.doc : "",
    editable,
    onCreate: ({ editor: created }) => {
      lastSaved.current = serializeStorage(created.getJSON());
    },
    onUpdate: ({ editor: current }) => {
      const next = serializeStorage(current.getJSON());
      const target = inFlightText.current ?? lastSaved.current;
      if (next === target) {
        pending.current = null;
        clearTimeout(timer.current);
        return;
      }
      pending.current = next;
      clearTimeout(timer.current);
      timer.current = setTimeout(() => void saveRef.current(), saveDelayMs);
    },
    // Not `[body, editable]`: see the comment on Props.doc about who owns
    // this editor's identity.
  }, [editable]);

  useEffect(() => { editorRef.current = editor; }, [editor]);

  useImperativeHandle(ref, () => ({ flush: () => saveRef.current(), editor }), [editor]);

  // Leaving a page, a sprint, or the view saves what was typed.
  useEffect(() => () => {
    if (pending.current !== null) void saveRef.current();
  }, []);

  const act = (run: Run) => {
    if (!editor) return;
    run(editor.chain().focus()).run();
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      void save();
    }
  };

  async function addTodaysEntry() {
    if (!editorRef.current) return;
    setNotice("");
    try {
      const raw = await StandupEntry(localDay(new Date()));
      // The editor this started with may have been unmounted, or torn down
      // and rebuilt because `editable` changed, while that call was in
      // flight; re-read it rather than trust the closure.
      const current = editorRef.current;
      if (!current || current.isDestroyed) return;
      const fragment = parseStorage(raw);
      if (!fragment.ok || !fragment.doc.content?.length) {
        setNotice(ENTRY_UNREADABLE);
        announce(ENTRY_UNREADABLE);
        return;
      }
      const heading = current.schema.nodeFromJSON(fragment.doc.content[0]).textContent.trim();
      const place = findTodaysEntry(current.state.doc, heading);
      if (place.kind !== "insert") {
        const sentence = place.kind === "exists" ? ENTRY_EXISTS : DAILY_LOG_MISSING;
        setNotice(sentence);
        announce(sentence);
        return;
      }
      current.chain().insertContentAt(place.pos, fragment.doc.content).run();
    } catch (e) {
      const current = editorRef.current;
      if (current && !current.isDestroyed) setNotice(errMsg(e));
    }
  }

  if (!parsed.ok) {
    return (
      <div className="ritual-readonly">
        <p className="warn-text">{READ_ONLY_SENTENCE}</p>
        {pageUrl && <button className="btn btn-ghost" onClick={() => BrowserOpenURL(pageUrl)}>Open in Confluence</button>}
        <div className="ritual-page-body" dangerouslySetInnerHTML={{ __html: sanitizeHtml(body) }} />
      </div>
    );
  }

  return (
    <RitualEditorContext.Provider value={{ profileId }}>
      <div
        className={`ritual-editor${readOnly ? " ritual-editor-readonly" : ""}`}
        onKeyDownCapture={onKeyDown}
      >
        {editable && editor && <Toolbar editor={editor} act={act} standup={doc.ritualType === "standup"} onAddEntry={() => void addTodaysEntry()} />}
        {notice && <p className="muted small ritual-notice" role="status">{notice}</p>}
        <EditorContent editor={editor} className="ritual-editor-content" />
        {editable && <p className="ritual-status-line muted small">{editorStatusLine({ doc, saving, saveError, savedAt })}</p>}
      </div>
    </RitualEditorContext.Provider>
  );
}

function Toolbar({ editor, act, standup, onAddEntry }: { editor: Editor; act: (run: Run) => void; standup: boolean; onAddEntry: () => void }) {
  const active = useEditorState({
    editor,
    selector: ({ editor: e }) => ({
      bold: e.isActive("bold"),
      italic: e.isActive("italic"),
      h2: e.isActive("heading", { level: 2 }),
      h3: e.isActive("heading", { level: 3 }),
      bullet: e.isActive("bulletList"),
      ordered: e.isActive("orderedList"),
      tasks: e.isActive("taskList"),
      link: e.isActive("link"),
    }),
  });
  const [href, setHref] = useState<string | null>(null);

  const buttons: { label: string; text: string; run: Run; pressed?: boolean }[] = [
    { label: "Bold", text: "B", run: (c) => c.toggleBold(), pressed: active.bold },
    { label: "Italic", text: "I", run: (c) => c.toggleItalic(), pressed: active.italic },
    { label: "Heading 2", text: "H2", run: (c) => c.toggleHeading({ level: 2 }), pressed: active.h2 },
    { label: "Heading 3", text: "H3", run: (c) => c.toggleHeading({ level: 3 }), pressed: active.h3 },
    { label: "Bullet list", text: "• List", run: (c) => c.toggleBulletList(), pressed: active.bullet },
    { label: "Numbered list", text: "1. List", run: (c) => c.toggleOrderedList(), pressed: active.ordered },
    { label: "Task list", text: "☐ Tasks", run: (c) => c.toggleTaskList(), pressed: active.tasks },
    { label: "Insert table", text: "Table", run: (c) => c.insertTable({ rows: 3, cols: 3, withHeaderRow: true }) },
    { label: "Undo", text: "Undo", run: (c) => c.undo() },
    { label: "Redo", text: "Redo", run: (c) => c.redo() },
  ];

  function applyLink() {
    const value = (href ?? "").trim();
    act((c) => (value ? c.extendMarkRange("link").setLink({ href: value }) : c.extendMarkRange("link").unsetLink()));
    setHref(null);
  }

  return (
    <div className="ritual-toolbar" role="toolbar" aria-label="Formatting">
      {buttons.map((b) => (
        <button key={b.label} type="button" className="btn btn-ghost" aria-label={b.label}
          aria-pressed={b.pressed === undefined ? undefined : b.pressed}
          onMouseDown={(e) => e.preventDefault()} onClick={() => act(b.run)}>
          {b.text}
        </button>
      ))}
      <button type="button" className="btn btn-ghost" aria-label="Link" aria-pressed={active.link}
        onMouseDown={(e) => e.preventDefault()}
        onClick={() => setHref(href === null ? String(editor.getAttributes("link").href ?? "") : null)}>
        Link
      </button>
      {href !== null && (
        <span className="ritual-link-input">
          <input aria-label="Link address" value={href} placeholder="https://" onChange={(e) => setHref(e.target.value)}
            onKeyDown={(e) => { if (e.key === "Enter") { e.preventDefault(); applyLink(); } if (e.key === "Escape") setHref(null); }} />
          <button type="button" className="btn btn-ghost" onClick={applyLink}>Apply</button>
        </span>
      )}
      {standup && <button type="button" className="btn btn-ghost ritual-add-entry" onClick={onAddEntry}>Add today's entry</button>}
    </div>
  );
}
