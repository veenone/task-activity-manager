import { describe, it, expect } from "vitest";
import type { ReportSeries } from "../api";
import {
  amount,
  builtAtLine,
  busyLine,
  floorLine,
  isBusyRefusal,
  methodLine,
  nameList,
  progressStage,
  summarySentence,
  truncationLine,
  unavailableLine,
  unitLine,
  unitWord,
  velocityFloorLine,
} from "./reportText";

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
  it("says that committed is a floor and that removed sees only the cards that came back", () => {
    const line = floorLine();
    expect(line).toContain("Committed is a floor");
    expect(line).toContain("left and came back");
  });
});

describe("unitLine", () => {
  it("says nothing at all for a report counted in points", () => {
    expect(unitLine("points", "")).toBe("");
  });
  it("tells the team it can fix this one by estimating", () => {
    expect(unitLine("cards", "nothingEstimated")).toContain("Estimating the cards");
  });
  it("claims story points only where the backend looked, which is this sprint and its history", () => {
    const line = unitLine("cards", "nothingEstimated");
    expect(line).toContain("in evidence somewhere in this sprint's issues or their history");
    expect(line).not.toContain("in evidence on this board");
  });
  it("does not claim the instance has no story points field, because the backend cannot know that", () => {
    const line = unitLine("cards", "noPointsFieldSeen");
    expect(line).toContain("nothing in this sprint's issues or their history mentions story points");
    expect(line).toContain("cannot tell those two apart");
  });
});

describe("methodLine", () => {
  it("says what done means, where the history comes from, and what a removal misses", () => {
    const line = methodLine();
    expect(line).toContain("board's last column");
    expect(line).toContain("public changelog");
    expect(line).toContain("only visible for a card that came back");
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
    expect(line).toContain("This sprint's figures were built");
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

describe("isBusyRefusal", () => {
  it("recognises the refusal the lock makes, whichever operation is holding it", () => {
    expect(isBusyRefusal("a sync is already running for this profile")).toBe(true);
    expect(isBusyRefusal("a report is already running for this profile")).toBe(true);
    expect(isBusyRefusal("a commit is already running for this profile")).toBe(true);
  });
  it("does not mistake an ordinary read failure for it", () => {
    expect(isBusyRefusal("Get \"https://jira.example/rest\": connection refused")).toBe(false);
  });
});

describe("busyLine", () => {
  it("quotes the refusal and says the lock refuses rather than queues", () => {
    const line = busyLine("a sync is already running for this profile");
    expect(line).toContain("sync is already running for this profile");
    expect(line).toContain("refuses rather than waits");
  });
  // The message is written to sit inside an error, and this is the first
  // thing the reader sees in the banner.
  it("opens the sentence with a capital rather than with Go's lowercase word", () => {
    expect(busyLine("a report is already running for this profile")).toMatch(/^A report is already running/);
  });
});

describe("velocityFloorLine", () => {
  // The table is published on its own by Phase 5's Rituals, so the
  // qualification cannot live in a paragraph beside it.
  it("says the Committed column is a floor in every row, not only in the sprint above it", () => {
    const line = velocityFloorLine();
    expect(line).toContain("Committed is a floor in every row");
    expect(line).toContain("never fetched");
  });
});

describe("progressStage", () => {
  const frame = { phase: "sprint", sprintId: 11, sprintName: "Sprint 11", fetched: 25, total: 200, done: false };
  it("names the sprint being read", () => {
    expect(progressStage(frame)).toBe("Reading Sprint 11");
  });
  it("says when the sprint being read is one the velocity table needs", () => {
    expect(progressStage({ ...frame, phase: "velocity" })).toBe("Reading Sprint 11 for the velocity table");
  });
  it("falls back to the sprint's id when the frame carries no name", () => {
    expect(progressStage({ ...frame, sprintName: "" })).toBe("Reading sprint 11");
  });
  it("says a report is being built for a phase it does not know", () => {
    expect(progressStage({ ...frame, phase: "later" })).toBe("Building the sprint report");
  });
});
