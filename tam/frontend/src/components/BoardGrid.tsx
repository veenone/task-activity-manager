import type { DragEvent, KeyboardEvent } from "react";
import type { BoardView, Issue, Sprint } from "../api";
import { posId } from "../lib/boardCells";
import { plural } from "../lib/format";
import { BoardCard } from "./BoardCard";
import { CardMoveMenu } from "./CardMoveMenu";
import type { BoardMoves } from "./useBoardMoves";

// EDGE is how close to the edge of the scroller a drag has to come before
// the board scrolls itself, and STEP is how far it scrolls each time.
// Native drag and drop does not auto-scroll a custom container, so a seven
// column board would have columns a mouse holding a card cannot reach.
const EDGE = 48;
const STEP = 24;

interface Props {
  view: BoardView;
  swimlane: string;
  selectedKey: string;
  focusId: string;
  moves: BoardMoves;
  flashKey: string;
  sprints: Sprint[];
  sprintId: string;
  // committing is a Commit in flight: the cards stop being draggable while
  // one is pushing.
  committing: boolean;
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

// scrollNearEdge keeps the column under the cursor reachable during a drag.
function scrollNearEdge(e: DragEvent<HTMLElement>) {
  const box = e.currentTarget.getBoundingClientRect();
  if (e.clientX < box.left + EDGE) e.currentTarget.scrollLeft -= STEP;
  else if (e.clientX > box.right - EDGE) e.currentTarget.scrollLeft += STEP;
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
export function BoardGrid({
  view, swimlane, selectedKey, focusId, moves, flashKey, sprints, sprintId, committing,
  onSelect, onFocusCard, onKeyDown,
}: Props) {
  const target = moves.target;
  return (
    <div
      className="board-scroll"
      role="grid"
      aria-label="Board"
      aria-colcount={view.columns.length}
      aria-describedby="board-move-keys"
      onDragOver={scrollNearEdge}
    >
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
            {view.columns.map((column, col) => {
              const over = target?.lane === laneIndex && target.col === col;
              const line = over ? target.index : null;
              // A cross-column drop lands the card at the rank it already
              // has, so the whole cell is outlined rather than given a gap
              // line: the outline says "it lands here" without claiming a
              // place in the order this move never chooses.
              const outlined = over && line === null;
              return (
                <div
                  className={`board-cell${over ? " board-cell-over" : ""}${outlined ? " board-cell-target" : ""}`}
                  key={column.name}
                  role="presentation"
                  onDragOver={(e) => moves.onCellDragOver(e, laneIndex, col)}
                  onDrop={(e) => moves.onCellDrop(e, laneIndex, col)}
                >
                  {(lane.cells[col] ?? []).map((issue, index) => {
                    const id = posId({ lane: laneIndex, col, index });
                    return (
                      <div className="board-card-slot" key={issue.key}>
                        {line === index && <div className="board-drop-line" />}
                        <BoardCard
                          issue={issue}
                          selected={issue.key === selectedKey}
                          focused={id === focusId}
                          columnName={column.name}
                          colIndex={col + 1}
                          posId={id}
                          move={moves.moves.get(issue.key) ?? { state: "", reason: "" }}
                          flashed={flashKey === issue.key}
                          dragging={moves.dragKey === issue.key}
                          draggable={!committing}
                          menu={(
                            <CardMoveMenu
                              issueKey={issue.key}
                              columns={view.columns}
                              col={col}
                              sprints={sprints}
                              sprintId={sprintId}
                              disabled={committing}
                              onColumn={(to) => moves.moveToColumn(issue.key, to)}
                              onSprint={(sprint) => moves.moveToSprint(issue.key, sprint)}
                            />
                          )}
                          onSelect={() => onSelect(issue)}
                          onFocus={() => onFocusCard(id)}
                          onKeyDown={(e) => onKeyDown(e, id)}
                          onDragStart={(e) => moves.onDragStart(e, issue.key)}
                          onDragEnd={moves.onDragEnd}
                        />
                      </div>
                    );
                  })}
                  {/* The line below the last card has no slot to be
                      positioned against, so it takes its own place in the
                      cell's flex column instead of escaping to whatever
                      ancestor happens to be positioned. */}
                  {line !== null && line >= (lane.cells[col] ?? []).length && (
                    <div className="board-drop-line board-drop-line-end" />
                  )}
                  {(lane.overflow[col] ?? 0) > 0 && (
                    <p className="board-overflow">{`+${lane.overflow[col]} more`}</p>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      ))}
    </div>
  );
}
