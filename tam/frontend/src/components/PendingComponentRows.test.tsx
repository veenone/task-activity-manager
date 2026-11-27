import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { DialogProvider } from "@agile-suite/core";
import type { CommitResult, PendingChange } from "../api";
import { groupPending } from "../queries/pending";
import { bannerLine } from "./CommitBanner";
import { summaryLine } from "./PendingChangesModal";
import { PendingGroupRows } from "./PendingGroupRows";

// Component changes are their own groups in Pending changes, read in words,
// and the Commit banner names them (#605).

const created: PendingChange = {
  id: 8, entityType: "component_create", entityKey: "-1", field: "create", beforeVal: "", baseVersion: "", createdAt: "",
  afterVal: JSON.stringify({ project: "PLAT", name: "Reports", description: "", leadName: "alee", leadDisplayName: "A. Lee", assigneeType: "COMPONENT_LEAD" }),
};
const edited: PendingChange = {
  id: 9, entityType: "component_edit", entityKey: "10000", field: "edit", baseVersion: "", createdAt: "",
  beforeVal: JSON.stringify({ name: "Auth", description: "", leadName: "", leadDisplayName: "", assigneeType: "PROJECT_DEFAULT" }),
  afterVal: JSON.stringify({ name: "Login", description: "", leadName: "alee", leadDisplayName: "A. Lee", assigneeType: "COMPONENT_LEAD" }),
};
const deleted: PendingChange = {
  id: 10, entityType: "component_delete", entityKey: "10001", field: "delete", beforeVal: "Board", baseVersion: "", createdAt: "",
  afterVal: JSON.stringify({ name: "Board", moveTo: 10000, moveToName: "Auth" }),
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

describe("component changes in Pending changes", () => {
  it("reads a draft component with its lead", () => {
    show(created);
    const group = screen.getByRole("rowgroup", { name: "Reports" });
    expect(group).toHaveTextContent("Draft component");
    expect(within(group).getByText("New component in PLAT, lead A. Lee, default assignee Component lead")).toBeInTheDocument();
    expect(within(group).getByRole("button", { name: "Discard Reports" })).toBeInTheDocument();
  });

  it("names what an edit changes", () => {
    show(edited);
    expect(screen.getByText("Edit component Auth: name to Login, lead to A. Lee, default assignee to Component lead")).toBeInTheDocument();
  });

  it("reads a delete and where its issues go", () => {
    show(deleted);
    const group = screen.getByRole("rowgroup", { name: "Board" });
    expect(within(group).getByText("Delete Board, its issues move to Auth")).toBeInTheDocument();
    expect(within(group).getByRole("button", { name: "Discard the change to Board" })).toBeInTheDocument();
  });

  it("keeps a draft component apart from a draft release with the same id", () => {
    const release: PendingChange = {
      ...created, id: 11, entityType: "version_create",
      afterVal: JSON.stringify({ project: "PLAT", name: "v2.6", description: "", startDate: "", releaseDate: "" }),
    };
    const groups = groupPending([created, release]);
    expect(groups.map((g) => g.kind)).toEqual(["release", "component"]);
    expect(summaryLine(groups, 2)).toBe("2 changes: 1 release change, 1 component change");
  });
});

describe("bannerLine", () => {
  it("names the component changes Commit pushed", () => {
    const r = { committed: [], created: [], linked: [], conflicts: [], failures: [], remaining: 0, componentsChanged: ["Reports created", "Board deleted"] } as CommitResult;
    expect(bannerLine(r)).toBe("Last commit: 2 component changes pushed (Reports created, Board deleted).");
  });
});
