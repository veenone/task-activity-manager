import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { StatusBar } from "./StatusBar";
import type { AppState } from "./useAppState";
import type { SyncProgress } from "../api";

// StatusBar reads three fields off AppState. Building the whole thing would
// say nothing about the bar.
function state(s: {
  progress?: SyncProgress | null;
  syncError?: string;
  syncState?: { testCount: number; lastSyncedAt: string } | null;
  health?: { ok: boolean } | null;
  theme?: string;
}): AppState {
  return {
    progress: s.progress ?? null,
    syncError: s.syncError ?? "",
    syncState: s.syncState ?? null,
    health: s.health ?? { ok: true },
    theme: s.theme ?? "light",
  } as unknown as AppState;
}

describe("StatusBar", () => {
  // A failed sync used to be a bare red string in the chrome, which read as
  // a stray value rather than the app reporting a run had failed (#313).
  it("announces a failed sync as a ribbon instead of the synced count", () => {
    render(
      <StatusBar
        s={state({
          syncError: "Jira rejected this profile's token.",
          syncState: { testCount: 4210, lastSyncedAt: "2026-10-20T10:00:00Z" },
        })}
      />,
    );

    const ribbon = screen.getByRole("alert");
    expect(ribbon).toHaveTextContent("Sync failed");
    expect(ribbon).toHaveTextContent("Jira rejected this profile's token.");
    // The count is what the bar shows when nothing is wrong; a failure
    // replaces it, so finding it here would mean the ribbon never rendered.
    expect(screen.queryByText(/4,210 tests/)).toBeNull();
  });

  // A new run supersedes the last run's failure: the bar shows what is
  // happening now, not what went wrong before.
  it("shows progress rather than a previous failure while a sync runs", () => {
    render(
      <StatusBar
        s={state({
          progress: { phase: "tests", fetched: 120, total: 400, done: false, stage: "Tests" },
          syncError: "Jira rejected this profile's token.",
        })}
      />,
    );

    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByText(/Tests: 120 \/ 400/)).toBeInstanceOf(HTMLElement);
  });

  it("shows the synced count when the last sync worked", () => {
    render(
      <StatusBar
        s={state({ syncState: { testCount: 4210, lastSyncedAt: "2026-10-20T10:00:00Z" } })}
      />,
    );

    expect(screen.queryByRole("alert")).toBeNull();
    expect(screen.getByText(/4,210 tests/)).toBeInstanceOf(HTMLElement);
  });
});

describe("the health glyph and the theme", () => {
  // useAppState has held health since the backend check was added and nothing
  // on screen read it, so a backend that had not come up looked the same as
  // one that had (#330). TAM has shown this since it shipped.
  it("says whether the local store is ready", () => {
    const { rerender } = render(<StatusBar s={state({ health: { ok: true } })} />);
    expect(screen.getByText("Local store ready")).toBeInstanceOf(HTMLElement);

    rerender(<StatusBar s={state({ health: { ok: false } })} />);
    expect(screen.getByText("Starting up")).toBeInstanceOf(HTMLElement);
  });

  // The glyph is colour, and colour alone is not a reading. The words beside
  // it carry the state, so the dot stays out of what is announced (I3).
  it("keeps the glyph out of what is read", () => {
    const { container } = render(<StatusBar s={state({ health: { ok: true } })} />);
    const dot = container.querySelector(".dot");
    expect(dot).toHaveAttribute("aria-hidden", "true");
    expect(dot?.className).toContain("dot-ok");
  });

  // The More menu sets the theme and then closes. Without this nothing says
  // which one is in force, and on System nothing says what System resolved to.
  it("says which theme is in force", () => {
    render(<StatusBar s={state({ theme: "dark" })} />);
    expect(screen.getByText(/Theme: dark/)).toBeInstanceOf(HTMLElement);
  });

  // A failure takes the whole bar, so the furniture goes with the count.
  it("gives the whole bar to a failure, health and theme included", () => {
    render(<StatusBar s={state({ syncError: "Jira rejected this token." })} />);
    expect(screen.getByRole("alert")).toHaveTextContent("Sync failed");
    expect(screen.queryByText("Local store ready")).toBeNull();
    expect(screen.queryByText(/Theme:/)).toBeNull();
  });
});
