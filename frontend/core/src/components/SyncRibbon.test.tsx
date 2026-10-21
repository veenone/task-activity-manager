import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { SyncRibbon } from "./SyncRibbon";

describe("SyncRibbon", () => {
  it("announces the failure and keeps the glyph out of what is read", () => {
    render(<SyncRibbon message="Jira found nothing at that address." />);
    const ribbon = screen.getByRole("alert");
    expect(ribbon).toHaveTextContent("Sync failed. Jira found nothing at that address.");
    // The mark is decoration beside the sentence. Read out, "!" is either
    // nothing or "exclamation mark", and neither belongs in front of the one
    // thing in the status bar the user has to act on.
    const mark = ribbon.querySelector<HTMLElement>(".sync-ribbon-mark");
    expect(mark).toHaveAttribute("aria-hidden", "true");
  });
});
