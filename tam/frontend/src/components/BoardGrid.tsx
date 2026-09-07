import type { KeyboardEvent } from "react";
import type { BoardView, Issue } from "../api";
import { posId } from "../lib/boardCells";
import { plural } from "../lib/format";
import { BoardCard } from "./BoardCard";

interface Props {
  view: BoardView;
  swimlane: string;
  selectedKey: string;
  focusId: string;
  onSelect: (issue: Issue) => void;
  onFocusCard: (id: string) => void;
  onKeyDown: (e: KeyboardEvent, id: string) => void;
}

// columnCount is the head's line under the column name: how many cards
// landed in it and how many points they carry, capped cards included, so a
// capped cell never makes its column look smaller than it is.
function columnCount(total: number, points: number): string {
  const cards = plural(total, "card", "cards");
  return points > 0 ? `${cards}, ${points} pts` : cards;
}

// BoardGrid draws the columns and the lane bands under them. It is one tab
// stop: exactly one card carries tabIndex 0, and the arrows move between
// cards from there.
//
// A lane band is a rowgroup holding one row, and the cards in that row are
// the cells, each carrying its column index; the cell that holds them is
// presentational, since a grid cell is the thing a reader lands on and that
// is the card. A screen reader reading a lane therefore hears every card
// with the column it sits in, which the card's own label repeats.
export function BoardGrid({ view, swimlane, selectedKey, focusId, onSelect, onFocusCard, onKeyDown }: Props) {
  return (
    <div className="board-scroll" role="grid" aria-label="Board" aria-colcount={view.columns.length}>
      <div className="board-columns" role="row">
        {view.columns.map((c, col) => (
          <div key={c.name} className="board-column-head" role="columnheader" aria-colindex={col + 1}>
            <span>{c.name}</span>
            <span className="board-column-count">{columnCount(c.total, c.points)}</span>
          </div>
        ))}
      </div>

      {view.lanes.map((lane, laneIndex) => (
        <div className="board-lane" key={lane.id || lane.label} role="rowgroup" aria-label={lane.label}>
          {swimlane !== "none" && (
            <div className="board-lane-head">
              <span>{lane.label}</span>
              <span className="muted small">{plural(lane.count, "card", "cards")}</span>
            </div>
          )}
          <div className="board-columns" role="row" aria-label={lane.label}>
            {view.columns.map((column, col) => (
              <div className="board-cell" key={column.name} role="presentation">
                {(lane.cells[col] ?? []).map((issue, index) => {
                  const id = posId({ lane: laneIndex, col, index });
                  return (
                    <BoardCard
                      key={issue.key}
                      issue={issue}
                      selected={issue.key === selectedKey}
                      focused={id === focusId}
                      columnName={column.name}
                      colIndex={col + 1}
                      posId={id}
                      onSelect={() => onSelect(issue)}
                      onFocus={() => onFocusCard(id)}
                      onKeyDown={(e) => onKeyDown(e, id)}
                    />
                  );
                })}
                {(lane.overflow[col] ?? 0) > 0 && (
                  <p className="board-overflow">{`+${lane.overflow[col]} more`}</p>
                )}
              </div>
            ))}
          </div>
        </div>
      ))}
    </div>
  );
}
