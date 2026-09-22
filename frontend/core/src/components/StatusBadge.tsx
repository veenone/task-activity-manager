// BadgeTone is what a badge means, not what colour it is. The four are the
// states a piece of work can be in on any surface in the suite: running,
// not started, finished, and not in the system of record yet.
export type BadgeTone = "active" | "future" | "closed" | "draft";

// One glyph per tone, so the badge survives a reader who cannot tell the
// four fills apart, a monochrome print, and a stylesheet that never loaded.
// Colour alone would be a distinction only some readers get.
const GLYPH: Record<BadgeTone, string> = {
  active: "●",
  future: "○",
  closed: "✓",
  draft: "◌",
};

interface Props {
  tone: BadgeTone;
  // The label as it should be read, in mixed case. The uppercase look a
  // badge has is text-transform in the stylesheet: an uppercased string
  // would be spelled out letter by letter by a screen reader, and any
  // accessible name built from the same string would shout too.
  label: string;
}

// StatusBadge is the one chip that says what state something is in. It takes
// a tone and a label and knows nothing about sprints, issues or boards; what
// each tone looks like is one token pair in tokens.css and one rule in
// primitives.css.
export function StatusBadge({ tone, label }: Props) {
  return (
    <span className={`status-badge status-badge-${tone}`}>
      <span className="status-badge-glyph" aria-hidden="true">{GLYPH[tone]}</span>
      <span className="status-badge-label">{label}</span>
    </span>
  );
}
