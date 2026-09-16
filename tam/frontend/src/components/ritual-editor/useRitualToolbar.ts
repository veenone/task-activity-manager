import { useEditorState } from "@tiptap/react";
import type { ChainedCommands, Editor } from "@tiptap/core";
import type { IconName, ToolbarGroup, ToolbarItem } from "@agile-suite/core";
import { LINK_REFUSED } from "../../lib/ritualText";
import { isAllowedLink } from "../../lib/sanitizeHtml";

type Chain = (chain: ChainedCommands) => ChainedCommands;

interface ToggleSpec {
  id: string;
  group: "marks" | "headings" | "lists";
  label: string;
  icon: IconName;
  shortcut: string;
  // requires is the schema mark or node this editor must carry for the
  // button to be offered at all.
  requires: string;
  active: (editor: Editor) => boolean;
  run: Chain;
}

// The shortcuts are the ones the TipTap extensions bind (Mod-b, Mod-Shift-s,
// Mod-Alt-2, Mod-Shift-8 and the rest), spelled for a Windows keyboard.
const TOGGLES: ToggleSpec[] = [
  { id: "bold", group: "marks", label: "Bold", icon: "bold", shortcut: "Ctrl+B", requires: "bold", active: (e) => e.isActive("bold"), run: (c) => c.toggleBold() },
  { id: "italic", group: "marks", label: "Italic", icon: "italic", shortcut: "Ctrl+I", requires: "italic", active: (e) => e.isActive("italic"), run: (c) => c.toggleItalic() },
  { id: "underline", group: "marks", label: "Underline", icon: "underline", shortcut: "Ctrl+U", requires: "underline", active: (e) => e.isActive("underline"), run: (c) => c.toggleUnderline() },
  { id: "strike", group: "marks", label: "Strikethrough", icon: "strike", shortcut: "Ctrl+Shift+S", requires: "strike", active: (e) => e.isActive("strike"), run: (c) => c.toggleStrike() },
  { id: "h2", group: "headings", label: "Heading 2", icon: "heading2", shortcut: "Ctrl+Alt+2", requires: "heading", active: (e) => e.isActive("heading", { level: 2 }), run: (c) => c.toggleHeading({ level: 2 }) },
  { id: "h3", group: "headings", label: "Heading 3", icon: "heading3", shortcut: "Ctrl+Alt+3", requires: "heading", active: (e) => e.isActive("heading", { level: 3 }), run: (c) => c.toggleHeading({ level: 3 }) },
  { id: "bullet", group: "lists", label: "Bullet list", icon: "bulletList", shortcut: "Ctrl+Shift+8", requires: "bulletList", active: (e) => e.isActive("bulletList"), run: (c) => c.toggleBulletList() },
  { id: "ordered", group: "lists", label: "Numbered list", icon: "orderedList", shortcut: "Ctrl+Shift+7", requires: "orderedList", active: (e) => e.isActive("orderedList"), run: (c) => c.toggleOrderedList() },
  { id: "tasks", group: "lists", label: "Task list", icon: "taskList", shortcut: "Ctrl+Shift+9", requires: "taskList", active: (e) => e.isActive("taskList"), run: (c) => c.toggleTaskList() },
];

const GROUP_LABEL: Record<ToggleSpec["group"], string> = { marks: "Text", headings: "Headings", lists: "Lists" };

const insertTable: Chain = (c) => c.insertTable({ rows: 3, cols: 3, withHeaderRow: true });

// Snapshot is plain data, so useEditorState's deep comparison re-renders the
// toolbar only on a transaction that changed something it draws. null in a
// field means this editor does not carry that feature, and the item is not
// offered.
interface Snapshot {
  toggles: { id: string; active: boolean; can: boolean }[];
  table: boolean | null;
  link: { href: string | null } | null;
  undo: boolean | null;
  redo: boolean | null;
}

function read(editor: Editor): Snapshot {
  const has = (name: string) => name in editor.schema.marks || name in editor.schema.nodes;
  const commands = editor.extensionManager.commands;
  return {
    toggles: TOGGLES.filter((t) => has(t.requires)).map((t) => ({ id: t.id, active: t.active(editor), can: t.run(editor.can().chain()).run() })),
    table: has("table") ? insertTable(editor.can().chain()).run() : null,
    link: has("link") ? { href: editor.isActive("link") ? String(editor.getAttributes("link").href ?? "") : null } : null,
    undo: "undo" in commands ? editor.can().undo() : null,
    redo: "redo" in commands ? editor.can().redo() : null,
  };
}

// applyLink is the popover's Apply. An empty address removes the link, an
// address TAM does not allow is refused with the sentence the popover shows,
// and anything else links the selection. The Link mark's own URI check would
// refuse a javascript: address too, but silently; this is what says why.
function applyLink(editor: Editor, href: string): string | null {
  const value = href.trim();
  if (!value) {
    editor.chain().focus().extendMarkRange("link").unsetLink().run();
    return null;
  }
  if (!isAllowedLink(value)) return LINK_REFUSED;
  editor.chain().focus().extendMarkRange("link").setLink({ href: value }).run();
  return null;
}

// useRitualToolbar maps the ritual editor's TipTap state onto the shared
// EditorToolbar's groups, re-rendering on every transaction that changes what
// the toolbar draws. It offers only what this editor's extensions carry.
export function useRitualToolbar(editor: Editor | null, options: { standup: boolean; onAddEntry: () => void }): ToolbarGroup[] {
  const snap = useEditorState({ editor, selector: ({ editor: e }) => (e ? read(e) : null) });
  if (!editor || !snap) return [];

  const run = (chain: Chain) => () => {
    chain(editor.chain().focus()).run();
  };

  const toggles = (group: ToggleSpec["group"]): ToolbarItem[] =>
    snap.toggles.flatMap((state) => {
      const spec = TOGGLES.find((t) => t.id === state.id);
      if (!spec || spec.group !== group) return [];
      return [{ kind: "toggle" as const, id: spec.id, label: spec.label, icon: spec.icon, shortcut: spec.shortcut, active: state.active, disabled: !state.can, onToggle: run(spec.run) }];
    });

  const insert: ToolbarItem[] = [];
  if (snap.table !== null) {
    insert.push({ kind: "action", id: "table", label: "Insert table", icon: "table", disabled: !snap.table, onRun: run(insertTable) });
  }
  if (snap.link) {
    insert.push({
      kind: "link",
      id: "link",
      label: "Link",
      href: snap.link.href,
      onApply: (href) => applyLink(editor, href),
      onRemove: () => {
        editor.chain().focus().extendMarkRange("link").unsetLink().run();
      },
    });
  }

  const history: ToolbarItem[] = [];
  if (snap.undo !== null) history.push({ kind: "action", id: "undo", label: "Undo", icon: "undo", shortcut: "Ctrl+Z", disabled: !snap.undo, onRun: run((c) => c.undo()) });
  if (snap.redo !== null) history.push({ kind: "action", id: "redo", label: "Redo", icon: "redo", shortcut: "Ctrl+Shift+Z", disabled: !snap.redo, onRun: run((c) => c.redo()) });

  const groups: ToolbarGroup[] = [
    { id: "marks", label: GROUP_LABEL.marks, items: toggles("marks") },
    { id: "headings", label: GROUP_LABEL.headings, items: toggles("headings") },
    { id: "lists", label: GROUP_LABEL.lists, items: toggles("lists") },
    { id: "insert", label: "Insert", items: insert },
    { id: "history", label: "History", items: history },
  ];
  if (options.standup) {
    groups.push({ id: "ritual", label: "Standup", items: [{ kind: "action", id: "entry", label: "Add today's entry", text: "+ Add today's entry", onRun: options.onAddEntry }] });
  }
  return groups.filter((g) => g.items.length > 0);
}
