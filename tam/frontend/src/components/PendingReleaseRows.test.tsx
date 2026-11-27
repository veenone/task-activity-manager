import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { DialogProvider } from "@agile-suite/core";
import type { CommitResult, PendingChange } from "../api";
import { groupPending } from "../queries/pending";
import { bannerLine } from "./CommitBanner";
import { summaryLine } from "./PendingChangesModal";
import { PendingGroupRows } from "./PendingGroupRows";

// Release changes are their own groups in Pending changes, read in words
// rather than JSON, and the Commit banner names them (#605).

const release: PendingChange = {
  id: 7, entityType: "version_release", entityKey: "10100", field: "release", beforeVal: "", baseVersion: "", createdAt: "",
  afterVal: JSON.stringify({ name: "v2.4", releaseDate: "2026-12-15", moveTo: 10101, moveToName: "v2.5" }),
};
const created: PendingChange = {
  id: 8, entityType: "version_create", entityKey: "-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "",
  afterVal: JSON.stringify({ project: "PLAT", name: "v2.6", description: "", startDate: "", releaseDate: "2026-11-01" }),
};
const edited: PendingChange = {
  id: 9, entityType: "version_edit", entityKey: "10100", field: "edit", baseVersion: "", createdAt: "",
  beforeVal: JSON.stringify({ name: "v2.4", description: "", startDate: "", releaseDate: "2026-12-15" }),
  afterVal: JSON.stringify({ name: "v2.4.0", description: "", startDate: "", releaseDate: "2026-12-20" }),
};

function show(row: PendingChange) {
  render(
    <DialogProvider>
      <table>
        <PendingGroupRows group={groupPending([row])[0]} project="PLAT" heldReasons={[]} unpushableIds={new Set()} busy={false} onDiscard={vi.fn()} />
      </table>
    </DialogProvider>,
  );
}

describe("release changes in Pending changes", () => {
  it("reads a release, its carry-over and its Discard", () => {
    show(release);
    const group = screen.getByRole("rowgroup", { name: "v2.4" });
    expect(within(group).getByText("Release v2.4 on 2026-12-15, unfinished issues Jira still finds go to v2.5")).toBeInTheDocument();
    expect(within(group).getByRole("button", { name: "Discard the change to v2.4" })).toBeInTheDocument();
  });

  it("reads a draft release", () => {
    show(created);
    expect(screen.getByRole("rowgroup", { name: "v2.6" })).toHaveTextContent("Draft release");
    expect(screen.getByText("New release in PLAT, release 2026-11-01")).toBeInTheDocument();
  });

  it("names what an edit changes", () => {
    show(edited);
    expect(screen.getByText("Edit release v2.4: name to v2.4.0, release date to 2026-12-20")).toBeInTheDocument();
  });

  it("keeps a draft release apart from a draft sprint with the same id", () => {
    const sprint: PendingChange = {
      ...created, id: 10, entityType: "sprint_create",
      afterVal: JSON.stringify({ boardId: 1, boardName: "B", name: "S1", goal: "", startDate: "", endDate: "" }),
    };
    const groups = groupPending([created, sprint]);
    expect(groups.map((g) => g.kind)).toEqual(["sprint", "release"]);
    expect(summaryLine(groups, 2)).toBe("2 changes: 1 new sprint, 1 release change");
  });
});

describe("bannerLine", () => {
  it("names the release changes Commit pushed", () => {
    const r = { committed: [], created: [], linked: [], conflicts: [], failures: [], remaining: 0, versionsChanged: ["v2.6 created", "v2.4 released"] } as CommitResult;
    expect(bannerLine(r)).toBe("Last commit: 2 release changes pushed (v2.6 created, v2.4 released).");
  });
});
