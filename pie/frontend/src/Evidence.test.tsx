import { beforeEach, expect, it, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DialogProvider } from "@agile-suite/core";
import { EvidencePanel } from "./EvidencePanel";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({
  Evidence: vi.fn(),
  FindEvidence: vi.fn(),
  SetEvidenceState: vi.fn(),
  AddEvidenceNote: vi.fn(),
  AddEvidenceIssue: vi.fn(),
  RemoveEvidence: vi.fn(),
}));

const items = [
  { id: 1, project: "ALPHA", kind: "link", ref: "ALPHA-7", detail: "clones ALPHA-7: Reset (Open)", state: "suggested", weak: false },
  { id: 2, project: "ALPHA", kind: "component", ref: "Reset", detail: '2 issues with component "Reset"', state: "dismissed", weak: true },
  { id: 3, project: "ALPHA", kind: "note", ref: "Seen by field team", detail: "Seen by field team", state: "accepted", weak: false },
  { id: 4, project: "DELTA", kind: "version", ref: "2.4", detail: '1 issues with fix version "2.4"', state: "suggested", weak: true },
];

function show(onAddProject = vi.fn()) {
  render(
    <DialogProvider>
      <EvidencePanel profileId="p1" caseId={9} projects={["ALPHA"]} onAddProject={onAddProject} />
    </DialogProvider>,
  );
  return userEvent.setup();
}

beforeEach(() => {
  vi.resetAllMocks();
  vi.mocked(api.Evidence).mockResolvedValue(items as never);
});

it("groups evidence by project, says which is weak, and accepts a suggestion", async () => {
  vi.mocked(api.SetEvidenceState).mockResolvedValue(items as never);
  const user = show();
  const alpha = await screen.findByRole("group", { name: "Evidence for ALPHA" });
  expect(api.Evidence).toHaveBeenCalledWith("p1", 9);
  expect(within(alpha).getByText("Link")).toBeVisible();
  expect(within(alpha).getByText("Component, weak")).toBeVisible();
  expect(within(alpha).getByText(/dismissed/)).toBeVisible();
  await user.click(within(alpha).getByRole("button", { name: "Accept ALPHA-7" }));
  expect(api.SetEvidenceState).toHaveBeenCalledWith("p1", 9, 1, "accepted");
});

it("finds evidence with the analyst's phrases, one per line", async () => {
  vi.mocked(api.FindEvidence).mockResolvedValue(items as never);
  const user = show();
  await screen.findByRole("group", { name: "Evidence for ALPHA" });
  await user.type(screen.getByLabelText("Phrases to search, one per line"), "PIN retry{Enter} reset {Enter}");
  await user.click(screen.getByRole("button", { name: "Find evidence" }));
  expect(api.FindEvidence).toHaveBeenCalledWith("p1", 9, ["PIN retry", "reset"]);
});

it("says why finding evidence failed", async () => {
  vi.mocked(api.FindEvidence).mockRejectedValue("read ALPHA-1 from Jira: the token was refused");
  const user = show();
  await user.click(await screen.findByRole("button", { name: "Find evidence" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("the token was refused");
});

it("offers to add a project the evidence points at", async () => {
  const onAddProject = vi.fn();
  const user = show(onAddProject);
  const delta = await screen.findByRole("group", { name: "Evidence for DELTA" });
  await user.click(within(delta).getByRole("button", { name: "Add DELTA to the case" }));
  expect(onAddProject).toHaveBeenCalledWith("DELTA");
  expect(within(await screen.findByRole("group", { name: "Evidence for ALPHA" })).queryByRole("button", { name: /to the case/ })).toBeNull();
});

it("adds a note by hand and removes evidence added by hand", async () => {
  vi.mocked(api.AddEvidenceNote).mockResolvedValue(items as never);
  vi.mocked(api.RemoveEvidence).mockResolvedValue(items as never);
  const user = show();
  await screen.findByRole("group", { name: "Evidence for ALPHA" });
  await user.type(screen.getByLabelText("Note or issue key"), "Same chip in the field");
  await user.click(screen.getByRole("button", { name: "Add note" }));
  expect(api.AddEvidenceNote).toHaveBeenCalledWith("p1", 9, "ALPHA", "Same chip in the field");
  await user.click(screen.getByRole("button", { name: "Remove Seen by field team" }));
  expect(api.RemoveEvidence).toHaveBeenCalledWith("p1", 9, 3);
});
