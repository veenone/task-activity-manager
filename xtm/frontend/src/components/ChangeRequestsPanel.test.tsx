import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ChangeRequestsPanel } from "./ChangeRequestsPanel";

vi.mock("../api", () => ({
  ListChangeRequests: vi.fn(async () => [
    {
      id: "cr-1",
      crKey: "CR-1",
      title: "Raise the limit",
      status: "open",
      targetVersionId: "",
      risk: "low",
      description: "",
      createdAt: "",
      updatedAt: "",
    },
  ]),
  CreateChangeRequest: vi.fn(),
  UpdateChangeRequest: vi.fn(),
  DeleteChangeRequest: vi.fn(),
  GetCRImpact: vi.fn(async () => ({
    canAccept: 2,
    cannotAccept: 0,
    pending: 1,
    decisions: [],
  })),
  SetCRDecision: vi.fn(),
  errMsg: (e: unknown) => String(e),
}));

vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

vi.mock("./useConfirm", () => ({
  useConfirm: () => ({ confirm: vi.fn(async () => false) }),
}));

// A change request opened its impact grid on a click anywhere on its card,
// and on nothing else, so the grid was out of a keyboard user's reach.
describe("ChangeRequestsPanel keyboard", () => {
  it("opens a change request's impact with Enter, as a click does", async () => {
    render(<ChangeRequestsPanel canonicalId="c1" versions={[]} onChanged={() => {}} />);

    const toggle = await screen.findByRole("button", { name: /Raise the limit/ });
    toggle.focus();
    await userEvent.keyboard("{Enter}");

    expect(await screen.findByText("Can accept: 2")).toBeInTheDocument();
  });
});
