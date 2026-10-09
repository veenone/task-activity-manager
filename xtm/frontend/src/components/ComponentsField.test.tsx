import { describe, it, expect, vi, beforeEach } from "vitest";
import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ComponentsField } from "./ComponentsField";

const api = vi.hoisted(() => ({
  ListProjectComponents: vi.fn(async () => ["API", "User Management"]),
  CreateComponent: vi.fn(),
}));
vi.mock("../api", () => ({ ...api, errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)) }));
const confirm = vi.hoisted(() => vi.fn(async () => true));
vi.mock("@agile-suite/core", async (orig) => ({
  ...(await orig<typeof import("@agile-suite/core")>()),
  useConfirm: () => ({ confirm }),
}));
const caps = vi.hoisted(() => ({ supportsComponentAdmin: true }));
vi.mock("../features", async (orig) => ({
  ...(await orig<typeof import("../features")>()),
  useCapabilities: () => caps,
}));

function Harness({ onSave = vi.fn(), readOnly = false }) {
  const [value, setValue] = useState<string[]>(["API"]);
  return (
    <ComponentsField
      profileId="p1"
      projectKey="QA"
      value={value}
      readOnly={readOnly}
      onSave={(names) => {
        setValue(names);
        onSave(names);
      }}
    />
  );
}

function renderField(props: Parameters<typeof Harness>[0] = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <Harness {...props} />
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  api.CreateComponent.mockReset();
  confirm.mockReset();
  confirm.mockResolvedValue(true);
  caps.supportsComponentAdmin = true;
});

const picker = () => screen.getByRole("combobox", { name: "Components" });

describe("ComponentsField", () => {
  it("adds a component whose name has a space and saves the list", async () => {
    const onSave = vi.fn();
    renderField({ onSave });
    await userEvent.type(picker(), "User Man");
    await userEvent.click(await screen.findByRole("option", { name: "User Management" }));
    expect(onSave).toHaveBeenLastCalledWith(["API", "User Management"]);
  });

  it("creates an unknown component after confirming", async () => {
    api.CreateComponent.mockResolvedValue({ id: "9", name: "Data Platform" });
    const onSave = vi.fn();
    renderField({ onSave });
    await userEvent.type(picker(), "Data Platform{Enter}");
    expect(confirm).toHaveBeenCalled();
    expect(api.CreateComponent).toHaveBeenCalledWith("p1", expect.objectContaining({ name: "Data Platform" }));
    expect(onSave).toHaveBeenLastCalledWith(["API", "Data Platform"]);
  });

  it("adds nothing when the confirm is declined or Jira refuses", async () => {
    const onSave = vi.fn();
    renderField({ onSave });
    confirm.mockResolvedValueOnce(false);
    await userEvent.type(picker(), "Nope{Enter}");
    expect(api.CreateComponent).not.toHaveBeenCalled();
    api.CreateComponent.mockRejectedValueOnce(new Error("You need project admin rights in Jira to change components."));
    await userEvent.clear(picker());
    await userEvent.type(picker(), "Denied{Enter}");
    expect(await screen.findByText("You need project admin rights in Jira to change components.")).toBeTruthy();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("offers no create row without component admin", async () => {
    caps.supportsComponentAdmin = false;
    renderField();
    await userEvent.type(picker(), "Zzz");
    expect(screen.queryByText('Create "Zzz"')).toBeNull();
  });

  it("read-only shows the components as text", () => {
    renderField({ readOnly: true });
    expect(screen.getByText("API").tagName).toBe("SPAN");
    expect(screen.queryByRole("combobox", { name: "Components" })).toBeNull();
  });
});
