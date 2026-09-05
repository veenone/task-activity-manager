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
  return { ...actual, GetIssueDetail: vi.fn(), ListLinkedTests: vi.fn() };
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
});

describe("IssueDetailPanel", () => {
  it("shows the cached fields at once and the description once fetched", async () => {
    renderPanel();
    expect(screen.getByRole("heading", { name: "PLAT-412" })).toBeInTheDocument();
    expect(screen.getByText("Checkout: apply promo code at payment step")).toBeInTheDocument();
    const details = screen.getByRole("tabpanel", { name: "Details" });
    expect(within(details).getByText("In Progress")).toBeInTheDocument();
    expect(within(details).getByText("R. Anand")).toBeInTheDocument();
    expect(within(details).getByText("Sprint 12 - Checkout polish")).toBeInTheDocument();
    expect(within(details).getByText("5")).toBeInTheDocument();
    expect(within(details).getByText("PLAT-350")).toBeInTheDocument();
    expect(within(details).getByText("checkout")).toBeInTheDocument();
    expect(within(details).getByText("Loading description")).toBeInTheDocument();
    await waitFor(() => expect(within(details).getByText(/As a shopper I can enter a promo code/)).toBeInTheDocument());
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
    expect(screen.getByText("R. Anand")).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Retry" }));
    await waitFor(() => expect(screen.getByText(/As a shopper I can enter a promo code/)).toBeInTheDocument());
    expect(api.GetIssueDetail).toHaveBeenCalledTimes(2);
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
