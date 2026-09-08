import type { Board, Sprint, Swimlane } from "../api";
import { SWIMLANES } from "../api";

interface Props {
  boards: Board[];
  board: Board | undefined;
  onBoard: (id: number) => void;
  // sprints are the ones the picker offers: the active and future ones,
  // which is exactly what the sync fetched membership for. A picker offering
  // a sprint whose cards were never fetched is a promise the view cannot
  // keep; closed sprints belong to Reports.
  sprints: Sprint[];
  sprint: Sprint | undefined;
  onSprint: (id: string) => void;
  swimlane: Swimlane;
  onSwimlane: (s: Swimlane) => void;
  refreshing: boolean;
  canRefresh: boolean;
  onRefresh: () => void;
  // filter narrows the cards drawn, by key, assignee, or issue type.
  filter: string;
  onFilter: (v: string) => void;
}

// sprintOption labels a sprint "Name (State)". Jira sends the state
// lowercase, so it is capitalised here for display only.
function sprintOption(s: Sprint): string {
  return `${s.name} (${s.state.charAt(0).toUpperCase()}${s.state.slice(1)})`;
}

// BoardsToolbar is XTM's board head, class for class: the pickers on the
// left, the actions pushed right. The board picker keeps XTM's 280px; the
// sprint and swimlane pickers take the narrow modifier, since 280px is right
// for a board name and absurd for three swimlane options.
export function BoardsToolbar({
  boards, board, onBoard, sprints, sprint, onSprint, swimlane, onSwimlane,
  refreshing, canRefresh, onRefresh, filter, onFilter,
}: Props) {
  return (
    <div className="board-head">
      <label className="board-picker">
        <span>Board</span>
        <select
          aria-label="Board"
          value={board?.id ?? ""}
          onChange={(e) => onBoard(Number(e.target.value))}
        >
          {boards.map((b) => (
            <option key={b.id} value={b.id}>{b.name}</option>
          ))}
        </select>
      </label>

      <label className="board-picker board-filter">
        <span>Filter</span>
        <input
          type="search"
          aria-label="Filter cards"
          placeholder="key, assignee, or type"
          value={filter}
          onChange={(e) => onFilter(e.target.value)}
        />
      </label>

      {board?.type === "scrum" && (
        <label className="board-picker">
          <span>Sprint</span>
          <select
            aria-label="Sprint"
            className="board-select-narrow"
            value={sprint ? String(sprint.id) : ""}
            onChange={(e) => onSprint(e.target.value)}
          >
            {sprints.length === 0 && <option value="" disabled>No open sprint</option>}
            {sprints.map((s) => (
              <option key={s.id} value={String(s.id)}>{sprintOption(s)}</option>
            ))}
          </select>
        </label>
      )}

      <label className="board-picker">
        <span>Swimlanes</span>
        <select
          aria-label="Swimlanes"
          className="board-select-narrow"
          value={swimlane}
          onChange={(e) => onSwimlane(e.target.value as Swimlane)}
        >
          {SWIMLANES.map((s) => (
            <option key={s.id} value={s.id}>{s.label}</option>
          ))}
        </select>
      </label>

      <div className="board-head-actions">
        {refreshing && <span className="muted small">Refreshing</span>}
        <button type="button" className="btn" disabled={!canRefresh} onClick={onRefresh}>
          Refresh
        </button>
      </div>
    </div>
  );
}
