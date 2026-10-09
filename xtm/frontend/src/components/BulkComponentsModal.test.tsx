import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { BulkComponentsModal } from "./BulkComponentsModal";

const api = vi.hoisted(() => ({
  BulkEditComponents: vi.fn(),
  ListProjectComponents: vi.fn(async () => ["API", "Core", "User Management"]),
  ListTestComponents: vi.fn(async () => ({ "QA-1": ["API", "Core"], "QA-2": ["Core"], "QA-3": [] })),
  CreateComponent: vi.fn(),
}));
vi.mock("../api", () => ({ ...api, errMsg: (e: unknown) => String(e) }));
vi.mock("../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1", activeProfile: { projectKey: "QA" } }),
}));
vi.mock("@agile-suite/core", async (orig) => ({
  ...(await orig<typeof import("@agile-suite/core")>()),
  useConfirm: () => ({ confirm: vi.fn(async () => true) }),
}));
vi.mock("../features", async (orig) => ({
  ...(await orig<typeof import("../features")>()),
  useCapabilities: () => ({ supportsComponentAdmin: false }),
}));

function renderModal(onComplete = vi.fn()) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <BulkComponentsModal testKeys={["QA-1", "QA-2", "QA-3"]} onComplete={onComplete} onCancel={() => {}} />
    </QueryClientProvider>,
  );
  return onComplete;
}

async function pick(name: string, text: string) {
  await userEvent.type(screen.getByRole("combobox", { name }), text);
  await userEvent.click(await screen.findByRole("option", { name: text }));
}

beforeEach(() => {
  api.BulkEditComponents.mockReset();
  api.BulkEditComponents.mockResolvedValue({ succeeded: ["QA-1", "QA-2", "QA-3"], failed: [] });
});

describe("BulkComponentsModal", () => {
  it("adds and removes with a preview", async () => {
    const onComplete = renderModal();
    await pick("Add components", "User Management");
    await pick("Remove components", "Core");
    expect(await screen.findByText("3 of 3 selected tests will change.")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(api.BulkEditComponents).toHaveBeenCalledWith("p1", ["QA-1", "QA-2", "QA-3"], ["User Management"], ["Core"], false);
    expect(onComplete).toHaveBeenCalledTimes(1);
  });

  it("replace all with nothing clears every selected test", async () => {
    renderModal();
    await userEvent.click(screen.getByRole("radio", { name: "Replace all" }));
    expect(screen.queryByRole("combobox", { name: "Remove components" })).toBeNull();
    expect(await screen.findByText("2 of 3 selected tests will change.")).toBeTruthy();
    await userEvent.click(screen.getByRole("button", { name: "Apply" }));
    expect(api.BulkEditComponents).toHaveBeenCalledWith("p1", ["QA-1", "QA-2", "QA-3"], [], [], true);
  });

  it("offers no create row on a profile without component admin", async () => {
    renderModal();
    await userEvent.type(screen.getByRole("combobox", { name: "Add components" }), "Zzz");
    expect(screen.queryByText('Create "Zzz"')).toBeNull();
  });
});
