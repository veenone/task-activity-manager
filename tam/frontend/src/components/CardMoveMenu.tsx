import { Menu } from "@agile-suite/core";
import type { MenuItem } from "@agile-suite/core";
import type { ColumnView, Sprint } from "../api";

// CARD_MENU_CLASS is what the card's keyboard path clicks: the menu key
// and Shift with F10 open the menu the mouse opens, and there is no other
// handle on a trigger the menu primitive owns.
export const CARD_MENU_CLASS = "board-card-menu";

interface Props {
  issueKey: string;
  columns: ColumnView[];
  // col is the card's own column, which is not offered as a destination.
  col: number;
  // sprints are the ones the board's picker offers. A kanban board has
  // none, and then the menu is columns alone.
  sprints: Sprint[];
  sprintId: string;
  disabled: boolean;
  onColumn: (col: number) => void;
  onSprint: (sprint: Sprint | null) => void;
}

// CardMoveMenu is the move a mouse can make without dragging and the one a
// keyboard can make without a simulated drag: every column the card can go
// to, and every sprint. Enter and Space are taken on a card, by the
// selection that opens the detail panel, so the menu has a trigger of its
// own rather than an activation key.
export function CardMoveMenu({ issueKey, columns, col, sprints, sprintId, disabled, onColumn, onSprint }: Props) {
  const items: MenuItem[] = [];
  columns.forEach((column, i) => {
    if (i === col || (column.statusIds ?? []).length === 0) return;
    items.push({ key: `col-${i}`, label: `Move to ${column.name}`, disabled, onClick: () => onColumn(i) });
  });
  const others = sprints.filter((s) => String(s.id) !== sprintId);
  if (others.length > 0 || sprintId) {
    if (items.length > 0) items.push({ key: "sprints", divider: true });
    for (const sprint of others) {
      items.push({
        key: `sprint-${sprint.id}`,
        label: `Move to ${sprint.name}`,
        disabled,
        onClick: () => onSprint(sprint),
      });
    }
    if (sprintId) {
      items.push({ key: "backlog", label: "Move to the backlog", disabled, onClick: () => onSprint(null) });
    }
  }
  if (items.length === 0) return null;

  return (
    // The card selects on click and answers its own keys, so the trigger
    // and the panel stop both before either reaches it: a click on Move
    // must not also open the detail panel, and Enter on a menu item must
    // not be read as Enter on the card.
    <span
      className="board-card-menu-wrap"
      onClick={(e) => e.stopPropagation()}
      onKeyDown={(e) => e.stopPropagation()}
    >
      <Menu
        label="Move"
        items={items}
        align="right"
        title={`Move ${issueKey}`}
        triggerLabel={`Move ${issueKey}`}
        triggerClassName={`btn btn-ghost ${CARD_MENU_CLASS}`}
        triggerTabIndex={-1}
      />
    </span>
  );
}
