import type { CSSProperties, ReactNode } from "react";

// RowPlace is where one row sits in its family.
//
// "detached" is the state every paged or truncated list has and none of them
// used to draw: the row is a subtask, but the parent it hangs off is not in
// the set being rendered, so there is nothing above it to be indented under.
// Drawn as a root it is indistinguishable from a top-level story, which is a
// lie about the backlog; drawn as a child it points at a parent that is not
// there. It gets the child's depth and a word saying why it stands alone.
export type RowPlace = "root" | "child" | "detached";

// One glyph for every list. Four tables each picked their own before, in
// three different places on the row.
export const BRANCH_GLYPH = "↳";

// What a detached child says. Short, because it shares a row with a summary,
// and the same words on every surface: four lists each wording this their own
// way is the drift this component exists to end.
export const DETACHED_TEXT = "Parent not shown";

interface Props {
  place: RowPlace;
  // The row's own expand control, for a row that has children. A row without
  // one still gets the slot the control would have filled.
  toggle?: ReactNode;
  // Whether to draw that slot at all. A surface where no row can expand, such
  // as a board card, passes false and reserves nothing.
  slot?: boolean;
}

const DEPTH: Record<RowPlace, number> = { root: 0, child: 1, detached: 1 };

// RowLead is the fixed lead every row of a family starts with: the slot the
// expand toggle lives in, then the branch glyph for a child, then the step
// that indents it.
//
// The step is one custom property, --subtask-indent, resolved once in the
// stylesheet, and the depth rides on the element. So a child's content starts
// to the right of its parent's by construction: both rows reserve the same
// slot, and only the child adds the step and the branch. Nothing here depends
// on two numbers in two rules happening to differ, which is how the toggle
// came to be wider than the indent and children came to render left of their
// parents.
export function RowLead({ place, toggle, slot = true }: Props) {
  const depth = DEPTH[place];
  return (
    <span
      className="row-lead"
      data-row-depth={depth}
      style={{ "--row-lead-depth": depth } as CSSProperties}
    >
      {slot && <span className="row-lead-slot">{toggle}</span>}
      {place !== "root" && <span className="row-lead-branch" aria-hidden="true">{BRANCH_GLYPH}</span>}
      {place === "detached" && <span className="row-lead-detached">{DETACHED_TEXT}</span>}
    </span>
  );
}
