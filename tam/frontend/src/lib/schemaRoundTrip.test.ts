import { describe, expect, it } from "vitest";
import { getSchema } from "@tiptap/core";
import { ritualExtensions } from "../components/ritual-editor/extensions";
import { parseStorage } from "./storage/parse";
import { serializeStorage } from "./storage/serialize";
import { normalizeStorage } from "./storage/normalize";
import sprint from "../../../internal/ritualtemplate/testdata/sprint.xml?raw";
import planning from "../../../internal/ritualtemplate/testdata/planning.xml?raw";
import standup from "../../../internal/ritualtemplate/testdata/standup.xml?raw";
import review from "../../../internal/ritualtemplate/testdata/review.xml?raw";
import retro from "../../../internal/ritualtemplate/testdata/retro.xml?raw";

const schema = getSchema(ritualExtensions());
const templates: Record<string, string> = { sprint, planning, standup, review, retro };

// A page read by the editor's real schema (not just parseStorage/serializeStorage
// directly, the way storage.test.ts checks) must come back unchanged: the
// schema fills every global attribute's default onto a node the parser left
// it off, and a default that differs from what serialize.ts treats as "not
// there" rewrites the page on the very first edit, forever.
describe("schema round trip", () => {
  for (const [name, xml] of Object.entries(templates)) {
    it(`keeps ${name} unchanged through the editor's schema`, () => {
      const parsed = parseStorage(xml);
      if (!parsed.ok) throw new Error(parsed.reason);
      const node = schema.nodeFromJSON(parsed.doc);
      const out = serializeStorage(node.toJSON());
      expect(normalizeStorage(out)).toBe(normalizeStorage(xml));
    });
  }
});
