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
import { DAILY_LOG_MISSING, ENTRY_EXISTS, READ_ONLY_SENTENCE, editorStatusLine } from "../../lib/ritualText";
import { RitualEditorContext } from "./context";
import { ritualExtensions } from "./extensions";

export const SAVE_DELAY_MS = 800;

export interface RitualEditorHandle {
  flush: () => Promise<void>;
  editor: Editor | null;
}

interface Props {
  profileId: string;
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

// RitualEditor edits one ritual page. It saves locally, 800 ms after the last
// edit, on Ctrl+S, on flush, and on unmount; it never talks to Confluence.
// Only an edit a person made counts: loading content, and anything a plugin
// does on load, never saves, so opening a page never makes it unsynced.
export function RitualEditor({ profileId, doc, body = doc.body, readOnly = false, pageUrl, onSaved, saveDelayMs = SAVE_DELAY_MS, ref }: Props) {
  const parsed = useMemo(() => parseStorage(body), [body]);
  const editable = !readOnly && parsed.ok;
  const extensions = useMemo(() => ritualExtensions(), []);
  const touched = useRef(false);
  const pending = useRef<string | null>(null);
  const timer = useRef<ReturnType<typeof setTimeout> | undefined>(undefined);
  const [saving, setSaving] = useState(false);
  const [savedAt, setSavedAt] = useState("");
  const [saveError, setSaveError] = useState("");
  const [notice, setNotice] = useState("");

  const save = useCallback(async () => {
    clearTimeout(timer.current);
    const next = pending.current;
    if (next === null) return;
    pending.current = null;
    setSaving(true);
    setSaveError("");
    try {
      const saved = await SaveRitualBody(profileId, doc.boardId, doc.sprintId, doc.ritualType, next);
      setSavedAt(new Date().toISOString());
      onSaved?.(saved);
    } catch (e) {
      // Keep the text for the next try, unless a newer edit already replaced it.
      if (pending.current === null) pending.current = next;
      setSaveError(errMsg(e));
    } finally {
      setSaving(false);
    }
  }, [profileId, doc.boardId, doc.sprintId, doc.ritualType, onSaved]);

  const saveRef = useRef(save);
  useEffect(() => { saveRef.current = save; }, [save]);

  const editor = useEditor({
    extensions,
    content: parsed.ok ? parsed.doc : "",
    editable,
    onUpdate: ({ editor: current }) => {
      if (!editable || !touched.current) return;
      pending.current = serializeStorage(current.getJSON());
      clearTimeout(timer.current);
      timer.current = setTimeout(() => void saveRef.current(), saveDelayMs);
    },
  }, [body, editable]);

  useImperativeHandle(ref, () => ({ flush: () => saveRef.current(), editor }), [editor]);

  // Leaving a page, a sprint, or the view saves what was typed.
  useEffect(() => () => {
    if (pending.current !== null) void saveRef.current();
  }, []);

  const act = (run: Run) => {
    if (!editor) return;
    touched.current = true;
    run(editor.chain().focus()).run();
  };

  const onKeyDown = (e: KeyboardEvent) => {
    if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === "s") {
      e.preventDefault();
      void save();
      return;
    }
    touched.current = true;
  };

  async function addTodaysEntry() {
    if (!editor) return;
    setNotice("");
    const fragment = parseStorage(await StandupEntry(localDay(new Date())));
    if (!fragment.ok || !fragment.doc.content?.length) return;
    const heading = editor.schema.nodeFromJSON(fragment.doc.content[0]).textContent.trim();
    const place = findTodaysEntry(editor.state.doc, heading);
    if (place.kind !== "insert") {
      const sentence = place.kind === "exists" ? ENTRY_EXISTS : DAILY_LOG_MISSING;
      setNotice(sentence);
      announce(sentence);
      return;
    }
    touched.current = true;
    editor.chain().insertContentAt(place.pos, fragment.doc.content).run();
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
        onPasteCapture={() => { touched.current = true; }}
        onDropCapture={() => { touched.current = true; }}
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
