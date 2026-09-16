import { describe, expect, it } from "vitest";
import { isAllowedLink } from "./links";

// Moved from tam/frontend/src/lib/sanitizeHtml.test.ts: isAllowedLink now
// lives in core so the ritual editor and the rich text renderer share one
// definition of an allowed link.
describe("isAllowedLink", () => {
  it("allows http, https and mailto addresses, in any case", () => {
    for (const href of ["https://example.com", "http://example.com/a?b=1", "MAILTO:team@example.com", "HTTPS://EXAMPLE.COM"]) {
      expect(isAllowedLink(href)).toBe(true);
    }
  });

  it("refuses other schemes, disguised ones, and relative addresses", () => {
    for (const href of ["javascript:alert(1)", "java\tscript:alert(1)", " javascript:alert(1)", "data:text/html,x", "ftp://example.com", "example.com", "/wiki/x", ""]) {
      expect(isAllowedLink(href)).toBe(false);
    }
  });
});
