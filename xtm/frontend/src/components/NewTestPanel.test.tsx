import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { NewTestPanel } from "./NewTestPanel";

const api = vi.hoisted(() => ({
  CreateTest: vi.fn(async () => "NEW-1"),
  GetTestCreateFields: vi.fn(async () => [
    { id: "customfield_20000", name: "Team", required: true, type: "text", allowedValues: [] },
  ]),
  ListComponents: vi.fn(async () => []),
  ListPriorities: vi.fn(async () => []),
  ListAllPreconditions: vi.fn(async () => []),
}));
vi.mock("../api", () => ({
  ...api,
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

describe("NewTestPanel", () => {
  it("asks for the create screen's required fields and sends them", async () => {
    const onCreated = vi.fn();
    render(<NewTestPanel folders={[]} onCreated={onCreated} onCancel={vi.fn()} />);

    await userEvent.type(screen.getByPlaceholderText("What does this test verify?"), "Login works");
    const create = screen.getByRole("button", { name: "Create →" });
    const team = await screen.findByLabelText("Team *");
    expect(create).toBeDisabled();

    await userEvent.type(team, "Payments");
    await userEvent.click(create);

    expect(api.CreateTest).toHaveBeenCalledWith(
      "p1",
      expect.objectContaining({
        summary: "Login works",
        fields: { customfield_20000: "Payments" },
      }),
    );
    expect(onCreated).toHaveBeenCalledWith("NEW-1");
  });
});
