import React from "react";
import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Issue } from "../api";
import { IssueDetailPanel } from "./IssueDetailPanel";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn(), EditIssue: vi.fn(), ListActivity: vi.fn() };
});

const story: Issue = {
  key: "PLAT-412", id: "1", project: "PLAT", type: "story", summary: "Checkout: apply promo code at payment step",
  status: "In Progress", assignee: "R. Anand", reporter: "PO", priority: "High", labels: ["checkout", "promo"],
  sprintId: "12", sprintName: "Sprint 12 - Checkout polish", parentKey: "PLAT-350", storyPoints: 5, rank: "",
  created: "2026-08-01T09:00:00Z", updated: "2026-09-05T09:58:00Z",
};

function renderPanel(onClose = vi.fn()) {
  render(
    <QueryClientProvider client={createQueryClient()}>
      <IssueDetailPanel profileId="p1" issue={story} onClose={onClose} />
    </QueryClientProvider>,
  );
  return onClose;
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.GetIssueDetail).mockResolvedValue({
    key: "PLAT-412",
    description: "As a shopper I can enter a promo code on the payment step.",
    links: [
      { direction: "inward", type: "Tested By", key: "XT-1018", summary: "Promo code applies discount", issueType: "Test" },
      { direction: "outward", type: "Relates", key: "PLAT-388", summary: "Promo codes must be single-use", issueType: "Requirement" },
    ],
    fields: {},
  });
  vi.mocked(api.ListLinkedTests).mockResolvedValue([
    { key: "XT-1018", summary: "Promo code applies discount", linkType: "Tested By" },
    { key: "XT-1019", summary: "Expired promo code rejected", linkType: "Tested By" },
  ]);
  vi.mocked(api.EditIssue).mockResolvedValue();
  vi.mocked(api.ListActivity).mockResolvedValue([
    { id: 2, occurredAt: "2026-09-06T10:05:00Z", actor: "araha", entityType: "issue", entityKey: "PLAT-412", action: "edit", field: "storyPoints", beforeVal: "5", afterVal: "8", note: "" },
    { id: 1, occurredAt: "2026-09-06T10:00:00Z", actor: "araha", entityType: "issue", entityKey: "PLAT-412", action: "edit", field: "summary", beforeVal: "Checkout: apply promo code", afterVal: "Checkout: apply promo code at payment step", note: "" },
  ]);
});

describe("IssueDetailPanel", () => {
  it("shows the cached fields at once and the description once fetched", async () => {
    renderPanel();
    expect(screen.getByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
    expect(screen.getByText("Checkout: apply promo code at payment step")).toBeInTheDocument();
    const details = screen.getByRole("tabpanel", { name: "Details" });
    expect(within(details).getByText("In Progress")).toBeInTheDocument();
    expect(within(details).getByLabelText("Assignee")).toHaveValue("R. Anand");
    expect(within(details).getByText("Sprint 12 - Checkout polish")).toBeInTheDocument();
    expect(within(details).getByLabelText("Story points")).toHaveValue("5");
    expect(within(details).getByText("PLAT-350")).toBeInTheDocument();
    expect(within(details).getByLabelText("Labels")).toHaveValue("checkout, promo");
    await waitFor(() => expect(screen.getByLabelText("Description")).toHaveValue("As a shopper I can enter a promo code on the payment step."));
    expect(api.GetIssueDetail).toHaveBeenCalledWith("p1", "PLAT-412");
  });

  it("switches to Links and Tests", async () => {
    renderPanel();
    await userEvent.click(screen.getByRole("tab", { name: "Links" }));
    const links = screen.getByRole("tabpanel", { name: "Links" });
    await waitFor(() => expect(within(links).getByText("XT-1018")).toBeInTheDocument());
    expect(within(links).getByText("Tested By")).toBeInTheDocument();
    expect(within(links).getByText("PLAT-388")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("tab", { name: "Tests" }));
    const tests = screen.getByRole("tabpanel", { name: "Tests" });
    await waitFor(() => expect(within(tests).getByText("XT-1019")).toBeInTheDocument());
    expect(within(tests).getByText(/via XTM, link: Tested By/)).toBeInTheDocument();
    expect(api.ListLinkedTests).toHaveBeenCalledWith("p1", "PLAT-412");
  });

  it("keeps the cached fields and offers a retry when the detail fetch fails", async () => {
    vi.mocked(api.GetIssueDetail).mockRejectedValueOnce(new Error("jira: 502 Bad Gateway"));
    renderPanel();
    await waitFor(() => expect(screen.getByTestId("detail-error")).toHaveTextContent("jira: 502 Bad Gateway"));
    expect(screen.getByLabelText("Assignee")).toHaveValue("R. Anand");
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.getByLabelText("Description")).toHaveValue("As a shopper I can enter a promo code on the payment step."));
    expect(api.GetIssueDetail).toHaveBeenCalledTimes(2);
  });

  it("retries the linked tests after a failure", async () => {
    vi.mocked(api.ListLinkedTests).mockRejectedValueOnce(new Error("jira: 503"));
    renderPanel();
    await userEvent.click(screen.getByRole("tab", { name: "Tests" }));
    const tests = screen.getByRole("tabpanel", { name: "Tests" });
    await waitFor(() => expect(within(tests).getByTestId("tests-error")).toHaveTextContent("jira: 503"));
    await userEvent.click(within(tests).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(within(tests).getByText("XT-1018")).toBeInTheDocument());
    expect(api.ListLinkedTests).toHaveBeenCalledTimes(2);
  });

  it("keeps the Links tab recoverable", async () => {
    vi.mocked(api.GetIssueDetail).mockRejectedValueOnce(new Error("jira: 502"));
    renderPanel();
    await userEvent.click(screen.getByRole("tab", { name: "Links" }));
    const links = screen.getByRole("tabpanel", { name: "Links" });
    await waitFor(() => expect(within(links).getByTestId("links-error")).toHaveTextContent("jira: 502"));
    await userEvent.click(within(links).getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(within(links).getByText("XT-1018")).toBeInTheDocument());
  });

  it("says when there are no linked tests and closes", async () => {
    vi.mocked(api.ListLinkedTests).mockResolvedValue([]);
    const onClose = renderPanel();
    await userEvent.click(screen.getByRole("tab", { name: "Tests" }));
    await waitFor(() => expect(screen.getByText("No linked tests.")).toBeInTheDocument());
    await userEvent.click(screen.getByRole("button", { name: "Close" }));
    expect(onClose).toHaveBeenCalled();
  });
});

describe("IssueDetailPanel write path", () => {
  it("prefills the editable fields and saves only what changed, in field order", async () => {
    const user = userEvent.setup();
    renderPanel();
    const summary = await screen.findByLabelText("Summary");
    expect(summary).toHaveValue("Checkout: apply promo code at payment step");
    expect(screen.getByLabelText("Labels")).toHaveValue("checkout, promo");
    expect(screen.getByLabelText("Story points")).toHaveValue("5");
    const description = await screen.findByLabelText("Description");
    await waitFor(() => expect(description).toHaveValue("As a shopper I can enter a promo code on the payment step."));
    const save = screen.getByRole("button", { name: "Save edit" });
    expect(save).toBeDisabled();

    await user.clear(screen.getByLabelText("Story points"));
    await user.type(screen.getByLabelText("Story points"), "8");
    await user.clear(summary);
    await user.type(summary, "Checkout: promo code at payment");
    expect(save).toBeEnabled();
    await user.click(save);
    await waitFor(() => expect(api.EditIssue).toHaveBeenCalledTimes(2));
    expect(vi.mocked(api.EditIssue).mock.calls[0]).toEqual(["p1", "PLAT-412", "summary", "Checkout: promo code at payment"]);
    expect(vi.mocked(api.EditIssue).mock.calls[1]).toEqual(["p1", "PLAT-412", "storyPoints", "8"]);
    expect(await screen.findByText("Saved. Commit pushes it to Jira.")).toBeInTheDocument();
  });

  it("refuses a blank summary and non-numeric points before calling the backend", async () => {
    const user = userEvent.setup();
    renderPanel();
    const summary = await screen.findByLabelText("Summary");
    await user.clear(summary);
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText("Summary cannot be empty.")).toBeInTheDocument();
    await user.type(summary, "x");
    await user.clear(screen.getByLabelText("Story points"));
    await user.type(screen.getByLabelText("Story points"), "eight");
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText("Story points must be a number.")).toBeInTheDocument();
    expect(api.EditIssue).not.toHaveBeenCalled();
  });

  it("shows the backend's error and keeps the edits in the form", async () => {
    const user = userEvent.setup();
    vi.mocked(api.EditIssue).mockRejectedValueOnce(new Error("field \"priority\" cannot be edited"));
    renderPanel();
    const priority = await screen.findByLabelText("Priority");
    await user.clear(priority);
    await user.type(priority, "Highest");
    await user.click(screen.getByRole("button", { name: "Save edit" }));
    expect(await screen.findByText(/cannot be edited/)).toBeInTheDocument();
    expect(priority).toHaveValue("Highest");
  });

  it("lists the activity newest first on the Activity tab", async () => {
    const user = userEvent.setup();
    renderPanel();
    await user.click(await screen.findByRole("tab", { name: "Activity" }));
    const items = await screen.findAllByRole("listitem");
    expect(items).toHaveLength(2);
    expect(items[0]).toHaveTextContent("araha edited Story points: 5 to 8");
    expect(items[1]).toHaveTextContent("araha edited Summary");
    expect(api.ListActivity).toHaveBeenCalledWith("p1", "PLAT-412", 200);
  });

  it("marks a draft in the panel head", async () => {
    render(
      <QueryClientProvider client={createQueryClient()}>
        <IssueDetailPanel profileId="p1" issue={{ ...story, key: "TAM-NEW-1", status: "Draft", draft: true, pending: true }} onClose={vi.fn()} />
      </QueryClientProvider>,
    );
    expect(await screen.findByText("Draft", { selector: "span.chip-draft" })).toBeInTheDocument();
    expect(screen.getByText("Commit creates this issue in Jira and gives it a real key.")).toBeInTheDocument();
  });
});
