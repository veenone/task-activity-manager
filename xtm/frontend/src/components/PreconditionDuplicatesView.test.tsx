import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { PreconditionDuplicateReport } from "../api";
import { PreconditionDuplicatesView } from "./PreconditionDuplicatesView";

const queryMocks = vi.hoisted(() => ({
  usePreconditionDuplicates: vi.fn(),
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "profile-2" }),
}));

vi.mock("../queries/duplicates", () => queryMocks);

const report: PreconditionDuplicateReport = {
  groups: [
    {
      normalizedSummary: "signed in",
      displaySummary: "Signed in",
      definitionVerdict: "identical",
      members: [
        {
          key: "PRE-1",
          summary: "Signed in",
          type: "Manual",
          condition: "The user is signed in",
          description: "First copy",
          testCount: 2,
        },
        {
          key: "PRE-2",
          summary: "Signed in",
          type: "Manual",
          condition: "The user is signed in",
          description: "Second copy",
          testCount: 1,
        },
      ],
    },
  ],
  groupCount: 1,
  preconditionCount: 2,
  definitionIdentical: 1,
  definitionDiffer: 0,
  excluded: 0,
  scannedAt: "",
};

describe("PreconditionDuplicatesView", () => {
  beforeEach(() => {
    queryMocks.usePreconditionDuplicates.mockReturnValue({
      data: report,
      error: null,
      refetch: vi.fn(),
    });
  });

  it("expands and collapses a focused duplicate group from the keyboard", async () => {
    const user = userEvent.setup();
    render(<PreconditionDuplicatesView />);

    const disclosure = screen.getByRole("button", { name: /Signed in/ });
    expect(screen.queryByText("PRE-1")).not.toBeInTheDocument();

    disclosure.focus();
    await user.keyboard("{Enter}");

    expect(disclosure).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText("PRE-1")).toBeVisible();

    await user.keyboard(" ");

    expect(disclosure).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText("PRE-1")).not.toBeInTheDocument();
  });
});
