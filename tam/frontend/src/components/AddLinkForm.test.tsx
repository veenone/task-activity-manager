import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import { AddLinkForm } from "./AddLinkForm";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetLinkTypes: vi.fn(), LookupIssue: vi.fn(), AddLink: vi.fn() };
});

function renderForm(onAdded = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <AddLinkForm profileId="p1" issueKey="PLAT-412" onAdded={onAdded} />
    </QueryClientProvider>,
  );
  return onAdded;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetLinkTypes).mockResolvedValue([
    { name: "Blocks", inward: "is blocked by", outward: "blocks" },
    { name: "Relates", inward: "relates to", outward: "relates to" },
  ]);
  vi.mocked(api.LookupIssue).mockResolvedValue({
    key: "PAY-77", id: "9", project: "PAY", type: "task", summary: "Rotate gateway signing keys", status: "To Do", assignee: "", reporter: "",
    priority: "", labels: [], sprintId: "", sprintName: "", parentKey: "", storyPoints: null, rank: "", created: "", updated: "",
  });
  vi.mocked(api.AddLink).mockResolvedValue();
});

describe("AddLinkForm", () => {
  it("lists both phrasings of each type once, checks the target, and adds", async () => {
    const user = userEvent.setup();
    const onAdded = renderForm();
    const select = await screen.findByLabelText("Link");
    // The select renders before its options arrive; wait for the
    // data-dependent option text itself before reading the list.
    await screen.findByRole("option", { name: "blocks" });
    const labels = Array.from(select.querySelectorAll("option")).map((o) => o.textContent);
    expect(labels).toEqual(["blocks", "is blocked by", "relates to"]);
    await user.selectOptions(select, "Blocks|inward");
    await user.type(screen.getByLabelText("Issue key"), "pay-77");
    expect(screen.getByRole("button", { name: "Add" })).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Check" }));
    expect(await screen.findByText("PAY-77, Task, Rotate gateway signing keys")).toBeInTheDocument();
    expect(api.LookupIssue).toHaveBeenCalledWith("p1", "PAY-77");
    await user.click(screen.getByRole("button", { name: "Add" }));
    await waitFor(() => expect(api.AddLink).toHaveBeenCalledWith("p1", "PLAT-412", {
      type: "Blocks", direction: "inward", toKey: "PAY-77", toSummary: "Rotate gateway signing keys", toType: "Task",
    }));
    expect(onAdded).toHaveBeenCalled();
    expect(screen.getByLabelText("Issue key")).toHaveValue("");
  });

  it("shows lookup and add errors", async () => {
    const user = userEvent.setup();
    vi.mocked(api.LookupIssue).mockRejectedValueOnce(new Error("GET failed: 404"));
    renderForm();
    await screen.findByLabelText("Link");
    await user.type(screen.getByLabelText("Issue key"), "NOPE-1");
    await user.click(screen.getByRole("button", { name: "Check" }));
    expect(await screen.findByText(/404/)).toBeInTheDocument();
    await user.clear(screen.getByLabelText("Issue key"));
    await user.type(screen.getByLabelText("Issue key"), "PAY-77");
    await user.click(screen.getByRole("button", { name: "Check" }));
    await screen.findByText("PAY-77, Task, Rotate gateway signing keys");
    vi.mocked(api.AddLink).mockRejectedValueOnce(new Error("a link from PLAT-412 to PAY-77 is already pending"));
    await user.click(screen.getByRole("button", { name: "Add" }));
    expect(await screen.findByText(/already pending/)).toBeInTheDocument();
  });
});
