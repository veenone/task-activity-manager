import { useId, useMemo, useRef, useState } from "react";
import type { KeyboardEvent, ReactElement, TextareaHTMLAttributes } from "react";
import type { RichFormat } from "./ast";
import { detectFormat } from "./detect";
import { RichText, SIZE_SENTENCE } from "./RichText";

// The name of each syntax, wherever one is named: the toggle's two buttons,
// the "Read as ..." hint, and the detail panel's chip saying a field is not
// being read the way it was detected. Exported so that chip reads this one
// record rather than a second copy of it.
export const FORMAT_LABEL: Record<RichFormat, string> = { wiki: "Jira markup", markdown: "Markdown" };

// Every sentence the field prints, exported so that its test can assert none
// of it carries an em dash, and so anything outside this file that has to
// print one of them reuses the exact wording rather than retyping it.
export const RICH_TEXT_SENTENCES = {
  detected: (f: RichFormat, labels: string[]): string =>
    f === "wiki"
      ? `Detected Jira markup (${labels.join(", ")}). Saved exactly as typed; Jira renders it the same way.`
      : `Detected Markdown (${labels.join(", ")}). Saved exactly as typed.`,
  none: "No markup detected, read as Jira markup. Saved exactly as typed.",
  picked: (f: RichFormat): string => `Read as ${FORMAT_LABEL[f]}, your choice. Saved exactly as typed.`,
  markdownWarning:
    "Jira Data Center renders this field as wiki markup, so Markdown will not look the same there. TAM renders it here; the text is sent to Jira unchanged.",
  sizeGuard: SIZE_SENTENCE,
};

interface SyntaxToggleProps {
  value: RichFormat;
  onChange: (f: RichFormat) => void;
  disabled?: boolean;
}

// Two aria-pressed buttons in one labelled group, the same shape as a radio
// pair without radio semantics: there is always exactly one current value,
// never a mixed or empty state, so "pressed" reads more plainly than
// "checked" would.
export function SyntaxToggle({ value, onChange, disabled }: SyntaxToggleProps): ReactElement {
  return (
    <span className="richfield-syntax" role="group" aria-label="Syntax">
      {(Object.keys(FORMAT_LABEL) as RichFormat[]).map((f) => (
        <button key={f} type="button" aria-pressed={value === f} disabled={disabled} onClick={() => onChange(f)}>
          {FORMAT_LABEL[f]}
        </button>
      ))}
    </span>
  );
}

interface Props {
  value: string;
  onChange: (v: string) => void;
  // "auto" until the user picks a format explicitly through the toggle;
  // once picked, the parent holds a concrete RichFormat and this field stops
  // running detection, so a later signal from the other syntax cannot flip
  // it back (Task 4 test: picking Markdown survives the text gaining wiki
  // markers).
  format: RichFormat | "auto";
  onFormatChange: (f: RichFormat) => void;
  onOpenLink: (url: string) => void;
  projectKey?: string;
  onIssueKey?: (key: string) => void;
  textarea?: TextareaHTMLAttributes<HTMLTextAreaElement>;
  minRows?: number;
}

type Tab = "write" | "preview";
const TABS: Tab[] = ["write", "preview"];
const TAB_LABEL: Record<Tab, string> = { write: "Write", preview: "Preview" };

// RichTextField is Write and Preview over one textarea: a tablist switches
// between the raw text and RichText's rendering of it, a syntax toggle picks
// wiki markup or Markdown when auto-detection needs overriding, and a hint
// line says which was used and why. The textarea is only ever unmounted by
// its parent, never by this component switching tabs (outside-voice fix 7):
// Preview hides it with the same clip technique .sr-only already uses
// elsewhere, not the hidden attribute and not display: none, so an outside
// <label htmlFor> still focuses it, which switches back to Write.
export function RichTextField({
  value,
  onChange,
  format,
  onFormatChange,
  onOpenLink,
  projectKey,
  onIssueKey,
  textarea,
  minRows = 4,
}: Props): ReactElement {
  const [tab, setTab] = useState<Tab>("write");
  const baseId = useId();
  const tabRefs = useRef<Record<Tab, HTMLButtonElement | null>>({ write: null, preview: null });

  const detected = useMemo(() => detectFormat(value), [value]);
  const resolvedFormat: RichFormat = format === "auto" ? detected.format : format;

  const hint =
    format === "auto"
      ? detected.signals.length > 0
        ? RICH_TEXT_SENTENCES.detected(detected.format, detected.signals)
        : RICH_TEXT_SENTENCES.none
      : RICH_TEXT_SENTENCES.picked(format);

  const lines = value.split("\n").length;
  const rows = Math.min(16, Math.max(minRows, lines));

  const focusTab = (t: Tab) => {
    setTab(t);
    tabRefs.current[t]?.focus();
  };

  const onTablistKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (e.key !== "ArrowRight" && e.key !== "ArrowLeft") return;
    e.preventDefault();
    const index = TABS.indexOf(tab);
    const delta = e.key === "ArrowRight" ? 1 : -1;
    focusTab(TABS[(index + delta + TABS.length) % TABS.length]);
  };

  // Bound on the wrapper, not the textarea: from Preview the focus sits on
  // the tab, and a shortcut that can only switch one way is not a toggle.
  const onKeyDown = (e: KeyboardEvent<HTMLDivElement>) => {
    if (!e.ctrlKey || !e.shiftKey || e.key.toLowerCase() !== "p") return;
    e.preventDefault();
    setTab((t) => (t === "write" ? "preview" : "write"));
  };

  const writePanelId = `${baseId}-panel-write`;
  const previewPanelId = `${baseId}-panel-preview`;

  return (
    <div className="richfield" onKeyDown={onKeyDown}>
      <div className="richfield-controls">
        <div className="richfield-tabs" role="tablist" aria-label="Write and Preview" onKeyDown={onTablistKeyDown}>
          {TABS.map((t) => (
            <button
              key={t}
              type="button"
              id={`${baseId}-tab-${t}`}
              role="tab"
              className="richfield-tab"
              aria-selected={tab === t}
              aria-controls={t === "write" ? writePanelId : previewPanelId}
              tabIndex={tab === t ? 0 : -1}
              ref={(el) => {
                tabRefs.current[t] = el;
              }}
              onClick={() => setTab(t)}
            >
              {TAB_LABEL[t]}
            </button>
          ))}
        </div>
        <SyntaxToggle value={resolvedFormat} onChange={onFormatChange} disabled={textarea?.disabled} />
      </div>

      <div className="richfield-hint">
        <p className="field-hint">{hint}</p>
        {resolvedFormat === "markdown" && <p className="field-hint field-hint-warn">{RICH_TEXT_SENTENCES.markdownWarning}</p>}
      </div>

      <div
        id={writePanelId}
        role={tab === "write" ? "tabpanel" : undefined}
        aria-labelledby={`${baseId}-tab-write`}
        className={tab === "write" ? "richfield-panel" : "richfield-panel sr-only"}
      >
        <textarea
          {...textarea}
          rows={rows}
          value={value}
          className={textarea?.className ? `richfield-textarea ${textarea.className}` : "richfield-textarea"}
          onChange={(e) => onChange(e.target.value)}
          onFocus={() => setTab("write")}
        />
      </div>

      {tab === "preview" && (
        <div id={previewPanelId} role="tabpanel" aria-labelledby={`${baseId}-tab-preview`} className="richfield-panel">
          <RichText text={value} format={resolvedFormat} projectKey={projectKey} onOpenLink={onOpenLink} onIssueKey={onIssueKey} />
        </div>
      )}
    </div>
  );
}
