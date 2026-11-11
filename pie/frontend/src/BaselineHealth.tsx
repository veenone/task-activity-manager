import type { Health } from "./baselineData";
import type { GEdge } from "./graphModel";
import { Card } from "./Statistics";

const pct = (n: number) => `${Math.round(n * 100)}%`;
const days = (n: number | null) => (n === null ? "-" : `${n} d`);

// ROWS name each health number, how to show it, and whether more is worse.
const ROWS: { key: keyof Health; label: string; show: (h: Health) => string; worse: "up" | "down" }[] = [
  { key: "inside", label: "issues inside", show: (h) => String(h.inside), worse: "up" },
  { key: "finishedShare", label: "finished", show: (h) => pct(h.finishedShare), worse: "down" },
  { key: "blockedFromOutside", label: "blocked from outside", show: (h) => String(h.blockedFromOutside), worse: "up" },
  { key: "external", label: "external dependencies", show: (h) => String(h.external), worse: "up" },
  { key: "outOfStep", label: "linked sets out of step", show: (h) => String(h.outOfStep), worse: "up" },
  { key: "medianAge", label: "median age, open", show: (h) => days(h.medianAge), worse: "up" },
];

function change(key: keyof Health, now: Health, then: Health): string {
  const [a, b] = [now[key], then[key]];
  if (a === null || b === null) return "";
  const d = key === "finishedShare" ? Math.round((a - b) * 100) : a - b;
  if (d === 0) return "no change";
  return `${d > 0 ? "+" : "−"}${Math.abs(d)}${key === "finishedShare" ? " pts" : ""}`;
}

// BaselineHealth is the baseline view's right-hand panel (#465): the health
// numbers for the chosen version, their change from a second version, and
// the blocker chain or impact being traced.
export function BaselineHealth({
  version,
  health,
  compareWith,
  then,
  walkTitle,
  steps,
  selected,
  onTrace,
}: {
  version: string;
  health: Health;
  compareWith: string;
  then: Health | null;
  walkTitle: string;
  steps: GEdge[];
  selected: string | null;
  onTrace: () => void;
}) {
  return (
    <aside className="pie-graph-details" aria-label="Baseline health">
      <h4>{`Baseline health${version ? ` · ${version}` : ""}`}</h4>
      <ul className="pie-stat-cards" aria-label="Health">
        {ROWS.map((r) => (
          <Card key={r.key} value={r.show(health)} label={r.label} />
        ))}
      </ul>
      {then && (
        <>
          <h4>{`${version || "All versions"} compared with ${compareWith}`}</h4>
          <dl className="pie-graph-facts" aria-label="Comparison">
            {ROWS.map((r) => {
              const c = change(r.key, health, then);
              // Whether a change is good depends on the number: more blocked
              // issues is worse, a larger share finished is better.
              const tone = !c || c === "no change" ? undefined : c.startsWith(r.worse === "up" ? "+" : "−") ? "pie-change-bad" : "pie-change-good";
              return [
                <dt key={`${r.key}-t`}>{r.label}</dt>,
                <dd key={`${r.key}-d`} className={tone}>{`${r.show(health)} (${c || "-"})`}</dd>,
              ];
            })}
          </dl>
        </>
      )}
      {selected && (
        <>
          <h4>{walkTitle}</h4>
          {steps.length === 0 ? (
            <p className="muted">Nothing on the chain.</p>
          ) : (
            <ul className="pie-graph-links" aria-label="Chain">
              {steps.map((e) => (
                <li key={e.id}>
                  <span className="mono">{e.source}</span>
                  <span className="muted">blocks</span>
                  <span className="mono">{e.target}</span>
                </li>
              ))}
            </ul>
          )}
          <button type="button" className="btn" onClick={onTrace}>
            {`Open ${selected} in Trace`}
          </button>
        </>
      )}
    </aside>
  );
}
