import { ENTITY_SPRINT_COMPLETE, ENTITY_SPRINT_EDIT, ENTITY_SPRINT_START, ISSUE_TYPES } from "../api";
import type { DraftBoard, DraftSprint, IssueDraft, PendingChange, SprintComplete, SprintEdit, SprintStart } from "../api";
import { dayInput } from "../lib/format";

// The Pending changes dialog's words for the journal rows that are not a
// field and a value: draft boards, draft sprints, sprint changes and draft
// issues.

// boardDraftLine says what kind of board a draft is and what it collects.
export function boardDraftLine(b: DraftBoard | null): string {
  if (!b) return "A draft board that could not be read. Discard it and draft it again.";
  const type = b.type.charAt(0).toUpperCase() + b.type.slice(1);
  return `${type} board on the filter ${b.filterName}: ${b.jql}`;
}

// sprintChangeName is the sprint's name: an edit's before value, the name
// every other change carries.
export function sprintChangeName(row: PendingChange): string {
  try {
    const name = row.entityType === ENTITY_SPRINT_EDIT
      ? (JSON.parse(row.beforeVal) as { name?: string }).name
      : (JSON.parse(row.afterVal) as { name?: string }).name;
    return name || `sprint ${row.entityKey}`;
  } catch {
    return `sprint ${row.entityKey}`;
  }
}

// sprintChangeNote says what Commit does with a sprint change.
export function sprintChangeNote(row: PendingChange): string {
  switch (row.entityType) {
    case ENTITY_SPRINT_EDIT: return "Saved locally. Commit sends the change to Jira.";
    case ENTITY_SPRINT_START: return "Commit starts it in Jira.";
    case ENTITY_SPRINT_COMPLETE: return "Commit works out the unfinished cards from Jira again, moves them, and closes the sprint.";
    default: return "Commit deletes it in Jira, and Jira moves its cards to the backlog.";
  }
}

// sprintChangeLine reads a sprint change in words: "Edit sprint Sprint 12:
// name to Sprint 12b, goal removed", "Start sprint Sprint 13, 2026-09-14 to
// 2026-09-28", "Complete sprint Sprint 12, unfinished cards to the backlog".
export function sprintChangeLine(row: PendingChange): string {
  const name = sprintChangeName(row);
  if (row.entityType === ENTITY_SPRINT_START) {
    try {
      const s = JSON.parse(row.afterVal) as SprintStart;
      return `Start sprint ${name}, ${dayInput(s.startDate)} to ${dayInput(s.endDate)}`;
    } catch {
      return `Start sprint ${name}`;
    }
  }
  if (row.entityType === ENTITY_SPRINT_COMPLETE) {
    try {
      const c = JSON.parse(row.afterVal) as SprintComplete;
      return `Complete sprint ${name}, unfinished cards to ${c.moveToName || "the backlog"}`;
    } catch {
      return `Complete sprint ${name}`;
    }
  }
  if (row.entityType !== ENTITY_SPRINT_EDIT) return `Delete sprint ${name}`;
  try {
    const was = JSON.parse(row.beforeVal) as DraftSprint;
    const now = JSON.parse(row.afterVal) as SprintEdit;
    const changed: string[] = [];
    if (now.name !== was.name) changed.push(`name to ${now.name}`);
    if (now.clearGoal) changed.push("goal removed");
    else if (now.goal && now.goal !== was.goal) changed.push(`goal to ${now.goal}`);
    const from = dayInput(now.startDate);
    const to = dayInput(now.endDate);
    if (from !== dayInput(was.startDate) || to !== dayInput(was.endDate)) changed.push(`dates to ${from} to ${to}`);
    return `Edit sprint ${name}: ${changed.length ? changed.join(", ") : "nothing changed"}`;
  } catch {
    return `Edit sprint ${name}: the change could not be read. Discard it and edit the sprint again.`;
  }
}

// sprintDraftLine says where and when a draft sprint runs.
export function sprintDraftLine(s: DraftSprint | null): string {
  if (!s) return "A draft sprint that could not be read. Discard it and draft it again.";
  const board = s.boardName || `board ${s.boardId}`;
  return `New sprint on ${board}, ${dayInput(s.startDate)} to ${dayInput(s.endDate)}`;
}

export function draftLine(d: IssueDraft, project: string): string {
  const type = ISSUE_TYPES.find((t) => t.id === d.type)?.label ?? d.type;
  const bits = [`New ${type} in ${project}`];
  if (d.priority) bits.push(`priority ${d.priority}`);
  if (d.assignee) bits.push(`assignee ${d.assignee}`);
  if (d.storyPoints !== null && d.storyPoints !== undefined) bits.push(`${d.storyPoints} points`);
  // A draft dragged into a sprint journals nothing, since Commit sends the
  // draft rather than the move, so the draft's own sprint is the only place
  // that move shows up. Its column cannot be named the same way: a draft
  // carries the status id it was dropped on and no status name, since Jira
  // has never granted it one.
  if (d.sprintName) bits.push(`sprint ${d.sprintName}`);
  return bits.join(", ");
}
