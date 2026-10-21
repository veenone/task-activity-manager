import { STATUS_LABEL } from "../lib/ritualText";
import type { RitualNavItem } from "../lib/ritualNav";
import { formatWhen } from "../lib/format";

interface Props {
  // Every document the sprint has, and the subset the filter kept. Both,
  // because the two answer different questions: the page pane reads the
  // selection out of the full list, so a filter never changes what is open,
  // while the list draws only what matched.
  shown: RitualNavItem[];
  selected: string;
  onSelect: (key: string) => void;
  filter: string;
  onFilter: (v: string) => void;
  // The two documents nobody gets unasked. Each offer stands where the
  // document itself will, and goes once it exists.
  offerBoardAgreement: boolean;
  offerSprintAdditions: boolean;
  boardAgreementLabel: string;
  sprintAdditionsLabel: string;
  onAddBoardAgreement: () => void;
  onAddSprintAdditions: () => void;
}

// RitualDocList is the Rituals view's left sidebar, built on the shape XTM's
// requirements sidebar has (#324): a filter over the list, then one bordered
// block per document carrying its name, its state and when it last synced.
// It was markup inside RitualsView until that file went past the 400-line
// limit; a list with its own filter is a thing of its own anyway.
export function RitualDocList({
  shown, selected, onSelect, filter, onFilter,
  offerBoardAgreement, offerSprintAdditions,
  boardAgreementLabel, sprintAdditionsLabel,
  onAddBoardAgreement, onAddSprintAdditions,
}: Props) {
  return (
    <nav aria-label="Ritual documents" className="ritual-docs">
      <div className="ritual-docs-head">
        <input
          type="search"
          className="detail-input ritual-docs-filter"
          aria-label="Filter documents"
          placeholder="Filter documents"
          value={filter}
          onChange={(e) => onFilter(e.target.value)}
        />
      </div>
      <ul>
        {shown.map((item) => (
          <li key={item.key}>
            <button
              className={`ritual-doc${selected === item.key ? " ritual-doc-selected" : ""}`}
              aria-current={selected === item.key ? "page" : undefined}
              onClick={() => onSelect(item.key)}
            >
              <span className="ritual-doc-top">
                <span className="ritual-doc-name">{item.label}</span>
                <span className={`ritual-chip ritual-chip-${item.doc.status}`}>{STATUS_LABEL[item.doc.status]}</span>
              </span>
              {/* The line the flat row had no room for. A document Confluence
                  has never seen says so rather than leaving an empty slot
                  where a date would go. */}
              <span className="ritual-doc-when muted">
                {item.doc.syncedAt ? `last synced ${formatWhen(item.doc.syncedAt)}` : "Not synced yet"}
              </span>
            </button>
          </li>
        ))}
        {shown.length === 0 && (
          <li className="muted small ritual-docs-empty">No documents match this filter.</li>
        )}
        {offerBoardAgreement && (
          <li>
            <button className="ritual-doc ritual-doc-offer" onClick={onAddBoardAgreement}>
              <span className="ritual-doc-name">{boardAgreementLabel}</span>
            </button>
          </li>
        )}
        {offerSprintAdditions && (
          <li>
            <button className="ritual-doc ritual-doc-offer" onClick={onAddSprintAdditions}>
              <span className="ritual-doc-name">{sprintAdditionsLabel}</span>
            </button>
          </li>
        )}
      </ul>
    </nav>
  );
}
