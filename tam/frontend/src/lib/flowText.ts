// flowText is every word a kanban board's report prints, the way
// lib/reportText is every word a sprint's report prints. Nothing here is
// written in a component, and nothing here carries an em dash.
//
// A kanban board has no sprint, so none of these sentences may borrow the
// sprint report's framing. The difference that matters most is scope: a
// sprint report describes a window that has closed, and a column count
// describes this moment. Wording either as the other is the one mistake a
// reader cannot catch, because both are just numbers on a page.

// flowHeadingLine names what the reader is looking at. The board's own name
// sits above it in the head, so this says what kind of report it is rather
// than repeating the board.
export function flowHeadingLine(): string {
  return "Column capacity";
}

// flowModeLine is the counterpart of the sprint report's modeLine, and it
// says the one thing that separates these figures from that report's: they
// are true now, not for a period that has ended. A reader who takes them for
// a window's summary reads every one of them wrong.
export function flowModeLine(): string {
  return "As it stands now. These counts are the board as the last sync left it.";
}

// flowScopeLine is capacityScopeLine's kanban twin. The sprint version says
// "this sprint's cards"; a kanban board has no sprint, so this says what was
// actually counted, which is the board's own issue list.
//
// The second clause is the same honesty the sprint report carries: a board
// filter routinely reaches past what TAM has synced, and a count that
// quietly omitted those cards would be a board that lies without saying so.
export function flowScopeLine(): string {
  return "The counts are this board's cards as the last sync left them, so a card Jira holds and TAM has not read is not among them.";
}

// flowEmptyLine is a board whose columns hold nothing TAM has synced. It is
// a fact about the cache rather than an empty table, the same distinction
// emptyVelocityLine draws.
export function flowEmptyLine(): string {
  return "No card on this board is in the local store yet, so there is nothing to count. Sync the project, then open this again.";
}

// flowNoColumnsLine is a board whose column configuration nothing has read.
// Without columns there is nothing to place a card in, so this is not an
// empty report but a board that has never been synced.
export function flowNoColumnsLine(): string {
  return "This board's columns have not been synced, so TAM cannot say which column a card is in. Open Boards to sync it.";
}

// flowPendingLine names the metrics a kanban report does not carry yet, so a
// reader who came looking for throughput or cycle time is told where they
// are rather than left to conclude the report is broken.
//
// It is here rather than left out because a report showing one section reads
// as a report that failed to build the rest. Saying which are coming is the
// difference between a partial view and an unexplained one.
export function flowPendingLine(): string {
  return "Throughput and cycle time are not built yet. This report carries the board's column capacity.";
}
