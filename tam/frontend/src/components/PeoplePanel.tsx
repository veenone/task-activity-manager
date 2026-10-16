import { Fragment, useState } from "react";
import { ProgressBar } from "@agile-suite/core";
import type { ReportPerson, ReportPersonCard, ReportSection } from "../api";
import { points, workDuration } from "../lib/format";
import { PEOPLE_COLUMNS, peopleCardLine, personBar, personKey, timeCells } from "../lib/reportPeople";
import { statusClass } from "../lib/statusClass";
import { ReportNotes } from "./ReportNotes";

// PeoplePanel is the People tab (#287): the section the outputs carry, drawn
// with what a screen can add to it. Each person's actual is the backlog's
// time bar against their estimate, and each row opens to the cards behind
// it. The figures in the cells are the outputs' own, from lib/reportPeople,
// so the tab and the published page read the same.
export function PeoplePanel({ section, people }: { section: ReportSection; people: ReportPerson[] }) {
  const [open, setOpen] = useState<Set<string>>(new Set());
  const toggle = (key: string) =>
    setOpen((was) => {
      const next = new Set(was);
      if (!next.delete(key)) next.add(key);
      return next;
    });

  return (
    <section className="report-people" role="group" aria-labelledby="report-people-heading">
      <h3 className="report-heading" id="report-people-heading">{section.heading}</h3>
      <div className="people-table-wrap">
        <table className="report-table people-table">
          <caption className="sr-only">{section.heading}</caption>
          <thead>
            <tr>
              <th scope="col">Person</th>
              <th scope="col">Username</th>
              <th scope="col">Email</th>
              <th scope="col" className="people-num">Cards</th>
              <th scope="col" className="people-num">Points</th>
              {PEOPLE_COLUMNS.map((c) => <th scope="col" className="people-num" key={c}>{c}</th>)}
              <th scope="col">Actual against estimate</th>
            </tr>
          </thead>
          <tbody>
            {people.map((p) => {
              const key = personKey(p);
              const name = p.name || "Unassigned";
              const isOpen = open.has(key);
              const listId = `people-cards-${key}`;
              return (
                <Fragment key={key}>
                  <tr className={`people-row${isOpen ? " people-row-open" : ""}`}>
                    <th scope="row">
                      <button
                        type="button"
                        className="people-toggle"
                        aria-expanded={isOpen}
                        aria-controls={isOpen ? listId : undefined}
                        aria-label={`${isOpen ? "Hide" : "Show"} ${name}'s cards`}
                        disabled={p.issues.length === 0}
                        onClick={() => toggle(key)}
                      >
                        <span aria-hidden="true">{isOpen ? "▾" : "▸"}</span>
                      </button>
                      <span className="people-name">{name}</span>
                    </th>
                    <td className="muted">{p.username}</td>
                    <td className="muted">{p.email}</td>
                    <td className="people-num">{p.cards}</td>
                    <td className="people-num">{points(p.points)}</td>
                    {timeCells(p).map((v, i) => <td className="people-num" key={PEOPLE_COLUMNS[i]}>{v}</td>)}
                    <td><TimeBarCell estimate={p.estimateSeconds} spent={p.spentSeconds} /></td>
                  </tr>
                  {isOpen && p.issues.map((c, i) => (
                    <CardRow key={c.key} card={c} id={i === 0 ? listId : undefined} />
                  ))}
                </Fragment>
              );
            })}
          </tbody>
        </table>
      </div>
      <ReportNotes notes={[...section.notes, peopleCardLine()]} />
    </section>
  );
}

// CardRow is one card under its person: the key and summary across the three
// identity columns, then the card's own figures in the person's columns.
function CardRow({ card: c, id }: { card: ReportPersonCard; id?: string }) {
  return (
    <tr className="people-card" id={id}>
      <td colSpan={3}>
        <span className="people-card-key">{c.key}</span> <span className="people-card-summary">{c.summary}</span>
      </td>
      <td className="people-num">
        <span className={`chip chip-status chip-status-${statusClass(c.status, c.statusCategory)}`} title={c.status}>
          {c.status}
        </span>
      </td>
      <td className="people-num">{c.points === null ? "" : points(c.points)}</td>
      {timeCells(c).map((v, i) => <td className="people-num" key={PEOPLE_COLUMNS[i]}>{v}</td>)}
      <td><TimeBarCell estimate={c.estimateSeconds} spent={c.spentSeconds} /></td>
    </tr>
  );
}

// TimeBarCell is the backlog's Time cell: the bar, or the unestimated stripe
// for time logged against no estimate, with the figures beside it.
function TimeBarCell({ estimate, spent }: { estimate: number; spent: number }) {
  const bar = personBar(estimate, spent);
  if (!bar && spent <= 0) return null;
  return (
    <span className="issue-time people-time">
      <span className="issue-time-bar">
        {bar ? (
          <ProgressBar value={bar.value} max={bar.max} label={bar.label} valueText={bar.valueText} tone={bar.tone} />
        ) : (
          <span className="progress-bar progress-bar-unestimated" aria-hidden="true" />
        )}
      </span>
      <span className="issue-time-text">{bar ? `${workDuration(spent)} of ${workDuration(estimate)}` : `${workDuration(spent)} logged`}</span>
    </span>
  );
}
