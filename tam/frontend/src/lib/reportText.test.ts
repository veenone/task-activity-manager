import { describe, it, expect } from "vitest";
import type { ReportSeries } from "../api";
import {
  agreementCountLine,
  agreementSourceLine,
  agreementStaleLine,
  amount,
  builtAtLine,
  chartAltLine,
  dayLine,
  emptyDaysLine,
  floorLine,
  kanbanEmptyLine,
  kanbanMetricsLine,
  kanbanScopeLine,
  kanbanUnsyncedLine,
  removedFloorLine,
  methodLine,
  mixedUnitsLine,
  nameList,
  outcomeLine,
  singleSprintLine,
  summarySentence,
  truncationLine,
  unavailableLine,
  unitLine,
  unitWord,
  velocityFloorLine,
  velocityLine,
} from "./reportText";
import { calendarDay } from "./format";

function series(over: Partial<ReportSeries> = {}): ReportSeries {
  return {
    sprintId: 11,
    sprintName: "Sprint 11",
    unit: "points",
    unitReason: "",
    committed: 34,
    added: 5,
    removed: 2,
    completed: 29,
    carriedOver: 10,
    days: [],
    truncated: [],
    issues: [],
    ...over,
  };
}

describe("unitWord", () => {
  it("says point and card in the singular and plural", () => {
    expect(unitWord("points", 1)).toBe("point");
    expect(unitWord("points", 2)).toBe("points");
    expect(unitWord("cards", 1)).toBe("card");
    expect(unitWord("cards", 0)).toBe("cards");
  });
  it("prints a unit it does not know rather than guessing at one", () => {
    expect(unitWord("hours", 3)).toBe("hours");
  });
});

describe("amount", () => {
  it("trims a whole number of its decimal and keeps a real fraction", () => {
    expect(amount(27, "points")).toBe("27 points");
    expect(amount(2.5, "points")).toBe("2.5 points");
  });
});

describe("summarySentence", () => {
  it("describes active sprint work as remaining rather than carried over", () => {
    expect(summarySentence(series(), true)).toBe(
      "Sprint 11 committed 34 points, added 5, removed 2, completed 29 so far and has 10 remaining.",
    );
  });
  it("names the sprint, the unit, and the five figures a review starts with", () => {
    expect(summarySentence(series())).toBe(
      "Sprint 11 committed 34 points, added 5, removed 2, completed 29 and carried over 10.",
    );
  });
  it("counts in cards when that is what the series counts in", () => {
    expect(summarySentence(series({ unit: "cards", committed: 12 }))).toContain("committed 12 cards");
  });
  it("still reads as a sentence when the sprint has no name", () => {
    expect(summarySentence(series({ sprintName: "" }))).toMatch(/^This sprint committed/);
  });
});

describe("floorLine", () => {
  it("says that committed is a floor, and only that", () => {
    // It qualifies the Committed tile, which every report draws, so it is
    // the half of the old sentence that is never dead text.
    const line = floorLine();
    expect(line).toContain("Committed is a minimum estimate");
    expect(line).not.toContain("left and later returned");
  });
});

// The other half of what floorLine used to say. A sprint that removed
// nothing has no invisible removals to warn about, so the sentence was two
// lines of 11px grey under the figures on every report that did not need it.
describe("removedFloorLine", () => {
  it("says nothing for a sprint that removed nothing", () => {
    expect(removedFloorLine(0)).toBe("");
  });

  it("warns that removed sees only the cards that came back, once one did", () => {
    expect(removedFloorLine(3)).toContain("left and later returned");
  });
});

describe("unitLine", () => {
  it("says nothing at all for a report counted in points", () => {
    expect(unitLine("points", "")).toBe("");
  });
  it("tells the team it can fix this one by estimating", () => {
    expect(unitLine("cards", "nothingEstimated")).toContain("Estimate the cards");
  });
  it("claims story points only where the backend looked, which is this sprint and its history", () => {
    const line = unitLine("cards", "nothingEstimated");
    expect(line).toContain("points appear in the sprint or its history");
  });
  it("does not claim the instance has no story points field, because the backend cannot know that", () => {
    const line = unitLine("cards", "noPointsFieldSeen");
    expect(line).toContain("no story points appear in the sprint or its history");
    expect(line).toContain("field is missing or unused");
  });
});

describe("methodLine", () => {
  it("says what done means, where the history comes from, and what a removal misses", () => {
    const line = methodLine();
    expect(line).toContain("board's last column");
    expect(line).toContain("uses Jira history");
    expect(line).toContain("counts only once it comes back");
  });
});

describe("nameList and truncationLine", () => {
  it("joins two keys with an and", () => {
    expect(nameList(["PLAT-1", "PLAT-2"])).toBe("PLAT-1 and PLAT-2");
  });
  it("counts the keys past the eighth instead of printing a paragraph of them", () => {
    const keys = Array.from({ length: 11 }, (_, i) => `PLAT-${i + 1}`);
    expect(nameList(keys)).toBe(
      "PLAT-1, PLAT-2, PLAT-3, PLAT-4, PLAT-5, PLAT-6, PLAT-7, PLAT-8 and 3 more",
    );
  });
  it("is empty when every changelog came back whole", () => {
    expect(truncationLine([])).toBe("");
  });
  it("names the cards and refuses to call the figures exact", () => {
    const line = truncationLine(["PLAT-9"]);
    expect(line).toContain("PLAT-9");
    expect(line).toContain("not exact");
  });
});

describe("builtAtLine", () => {
  it("is empty for a report that carries no stamp", () => {
    expect(builtAtLine("")).toBe("");
  });
  it("claims the sprint's own age and says the velocity rows carry none", () => {
    const line = builtAtLine("2026-09-10T08:00:00Z");
    expect(line).toContain("TAM built this sprint's figures");
    expect(line).toContain("velocity rows carry no stamp of their own");
  });
});

describe("unavailableLine", () => {
  it("words each of the four reasons differently", () => {
    const lines = ["boardNotSynced", "sprintNotFound", "sprintHasNoDates", "noClosedSprint"].map(unavailableLine);
    expect(new Set(lines).size).toBe(4);
    expect(lines[0]).toContain("which statuses count as finished");
    expect(lines[1]).toContain("cached sprint list does not hold that sprint");
    expect(lines[2]).toContain("no start or end date");
    expect(lines[3]).toContain("no closed sprint a report can be built from");
  });
  // Go answers boardNotSynced for two shapes, and a message naming only
  // the first sends a user with the second to a Refresh that cannot help.
  it("gives both halves of the reason a board cannot say what finished means", () => {
    const line = unavailableLine("boardNotSynced");
    expect(line).toContain("board's columns are not in the cache");
    expect(line).toContain("last column collects no status");
  });
  // Go answers noClosedSprint whenever VelocitySprints is empty, which a
  // board with closed sprints and unreadable dates also is, and in that
  // shape the picker beside this sentence is listing those sprints.
  it("is true of a board that closed nothing and of one whose closed sprints have no readable dates", () => {
    const line = unavailableLine("noClosedSprint");
    expect(line).toContain("has never closed one");
    expect(line).toContain("carries a start or an end date TAM cannot read");
  });
  it("prints a reason it has no wording for rather than showing an empty pane", () => {
    expect(unavailableLine("somethingLater")).toContain("somethingLater");
  });
});

describe("velocityFloorLine", () => {
  // The table is published on its own by Phase 5's Rituals, so the
  // qualification cannot live in a paragraph beside it.
  it("says the Committed column is a floor in every row, not only in the sprint above it", () => {
    const line = velocityFloorLine();
    expect(line).toContain("Committed is a minimum estimate in every row");
    expect(line).toContain("Cards removed for good are not visible");
  });
});

describe("the chart sentences", () => {
  it("reads one day of the burndown with every value the tooltip carries", () => {
    const line = dayLine({ date: "2026-09-12", scope: 34, completed: 10, remaining: 24, ideal: 20.5 }, "points");
    expect(line).toContain(calendarDay("2026-09-12"));
    expect(line).toContain("34 points in scope");
    expect(line).toContain("10 completed");
    expect(line).toContain("24 remaining");
    expect(line).toContain("ideal 20.5");
  });

  it("reads a velocity row in its own unit", () => {
    expect(velocityLine({ sprintId: 1, sprintName: "Sprint 10", unit: "cards", unitReason: "", committed: 12, completed: 9, truncated: false }))
      .toBe("Sprint 10: committed 12 cards, completed 9.");
  });

  it("reads one outcome bar as its label and amount", () => {
    expect(outcomeLine("Carried over", 1, "points")).toBe("Carried over: 1 point.");
  });

  it("says why the velocity chart splits by unit, and why one sprint has no trend", () => {
    expect(mixedUnitsLine()).toMatch(/different units/);
    expect(singleSprintLine()).toMatch(/one closed sprint/i);
    expect(emptyDaysLine()).toBe("No days to draw yet.");
  });

  it("describes an exported chart by the caption it carries on screen", () => {
    expect(chartAltLine("Burndown")).toContain("Burndown");
    // The figures are in the table beside the picture in every output, so the
    // description says where to read them rather than listing them again.
    expect(chartAltLine("Burndown")).toMatch(/table/);
    expect(chartAltLine("Velocity in cards")).toContain("Velocity in cards");
  });
});

describe("the done agreement's sentences", () => {
  it("says how many cards the counts beside the items are counted over", () => {
    expect(agreementCountLine(20)).toContain("held 20 cards");
    expect(agreementCountLine(1)).toContain("held 1 card ");
  });

  it("says the items and the ticks are not Jira's", () => {
    // The page is read at a review, away from TAM, where somebody who takes
    // these figures for Jira's own goes looking for a field that is not there.
    expect(agreementSourceLine()).toContain("Neither is held in Jira");
  });

  it("counts the ticks made against wording the agreement has since changed", () => {
    expect(agreementStaleLine(0)).toBe("");
    expect(agreementStaleLine(1)).toBe(
      "One tick was made against wording the agreement has since changed, so it is not counted above.",
    );
    expect(agreementStaleLine(3)).toContain("3 ticks were made against wording the agreement has since changed");
    expect(agreementStaleLine(3)).toContain("not counted above");
  });
});

// The kanban report's sentences. A kanban board has no sprint, so its report
// has no window its figures belong to, no stamp in the sense builtAtLine
// makes one, and two empty boards that mean two different things.
describe("the kanban report's sentences", () => {
  it("puts the counts now rather than in a period that closed", () => {
    expect(kanbanScopeLine()).toContain("the board as the last sync left it");
    expect(kanbanScopeLine()).toContain("not a period that has closed");
    // A card Jira holds and TAM has not read is the one thing a count off the
    // cache cannot see, and the caveat capacityScopeLine carries for a sprint.
    expect(kanbanScopeLine()).toContain("TAM has not read");
  });

  it("names the three metrics a kanban team will not find in it", () => {
    expect(kanbanMetricsLine()).toContain("Throughput");
    expect(kanbanMetricsLine()).toContain("cycle time");
    expect(kanbanMetricsLine()).toContain("cumulative flow diagram");
    expect(kanbanMetricsLine()).toContain("not built yet");
  });

  it("tells a board whose columns never arrived from one whose columns hold nothing", () => {
    // Two facts and two fixes: the first is the Boards view's refresh and the
    // second is an issue sync, so a reader sent to the wrong one refreshes the
    // boards and reads the same empty table again.
    expect(kanbanUnsyncedLine()).toContain("columns are not in the cache");
    expect(kanbanUnsyncedLine()).toContain("Open Boards");
    expect(kanbanEmptyLine()).toContain("columns are synced");
    expect(kanbanEmptyLine()).toContain("Sync this project's issues");
    expect(kanbanEmptyLine()).not.toContain("not in the cache");
  });
});
