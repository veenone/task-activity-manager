import { describe, expect, it } from "vitest";
import { getSchema } from "@tiptap/core";
import { ritualExtensions } from "../components/ritual-editor/extensions";
import { parseStorage } from "./storage/parse";
import { findTodaysEntry, localDay } from "./standupLog";

const schema = getSchema(ritualExtensions());

function docOf(body: string) {
  const parsed = parseStorage(body);
  if (!parsed.ok) throw new Error(parsed.reason);
  return schema.nodeFromJSON(parsed.doc);
}

describe("standup log", () => {
  const log = "<h2>Blockers and work in flight</h2><p>x</p><h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>";

  it("puts a new day straight under the Daily log heading", () => {
    const doc = docOf(log);
    const place = findTodaysEntry(doc, "Tue 15 Sep 2026");
    expect(place.kind).toBe("insert");
    if (place.kind === "insert") expect(doc.nodeAt(place.pos)?.textContent).toBe("Mon 14 Sep 2026");
  });

  it("refuses a second entry for the same day", () => {
    expect(findTodaysEntry(docOf(log), "Mon 14 Sep 2026").kind).toBe("exists");
  });

  it("does not count a same-named heading outside the log", () => {
    expect(findTodaysEntry(docOf("<h2>Daily log</h2><h2>Notes</h2><h3>Mon 14 Sep 2026</h3>"), "Mon 14 Sep 2026").kind).toBe("insert");
  });

  it("refuses when the Daily log heading is gone", () => {
    expect(findTodaysEntry(docOf("<h2>Something else</h2>"), "Mon 14 Sep 2026").kind).toBe("missing");
  });

  it("formats a local day for the binding", () => {
    expect(localDay(new Date(2026, 8, 5))).toBe("2026-09-05");
  });
});
