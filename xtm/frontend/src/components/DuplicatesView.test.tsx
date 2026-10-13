import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { DuplicateReport } from "../api";
import { DuplicatesView } from "./DuplicatesView";

const queryMocks = vi.hoisted(() => ({
  useDuplicates: vi.fn(),
  usePreconditionDuplicates: vi.fn(),
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "profile-1" }),
}));

vi.mock("../queries/duplicates", () => queryMocks);

const report: DuplicateReport = {
  groups: [
    {
      normalizedSummary: "repeated test",
      displaySummary: "Repeated test",
      stepsVerdict: "identical",
      members: [
        {
          key: "TEST-1",
          summary: "Repeated test",
          description: "First copy",
          status: "Draft",
          folderId: "folder-1",
        },
        {
          key: "TEST-2",
          summary: "Repeated test",
          description: "Second copy",
          status: "Draft",
          folderId: "folder-1",
        },
      ],
    },
  ],
  groupCount: 1,
  testCount: 2,
  stepsIdentical: 1,
  stepsDiffer: 0,
  stepsUnscanned: 0,
  excluded: 0,
  scannedAt: "",
};

describe("DuplicatesView", () => {
  beforeEach(() => {
    queryMocks.useDuplicates.mockReturnValue({
      data: report,
      error: null,
      refetch: vi.fn(),
    });
  });

  it("expands and collapses a focused duplicate group from the keyboard", async () => {
    const user = userEvent.setup();
    render(
      <DuplicatesView
        folders={[]}
        pendingByTestKey={new Map()}
        onChanged={vi.fn()}
      />,
    );

    const disclosure = screen.getByRole("button", { name: /Repeated test/ });
    expect(screen.queryByText("TEST-1")).not.toBeInTheDocument();

    disclosure.focus();
    await user.keyboard("{Enter}");

    expect(disclosure).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("TEST-1")).toBeVisible();

    await user.keyboard(" ");

    expect(disclosure).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("TEST-1")).not.toBeInTheDocument();
  });
});
