import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { ComponentFormModal } from "./ComponentFormModal";

vi.mock("../../api", () => ({
  SearchUsers: vi.fn(async () => [{ name: "alice", displayName: "Alice Andersen" }]),
  errMsg: (e: unknown) => String(e),
}));
vi.mock("../../contexts/ProfileContext", () => ({
  useProfile: () => ({ activeId: "p1" }),
}));

function renderForm(props: Partial<Parameters<typeof ComponentFormModal>[0]> = {}) {
  const onSubmit = props.onSubmit ?? vi.fn(async () => {});
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  render(
    <QueryClientProvider client={client}>
      <ComponentFormModal takenNames={["Core"]} onSubmit={onSubmit} onCancel={() => {}} {...props} />
    </QueryClientProvider>,
  );
  return onSubmit;
}

describe("ComponentFormModal", () => {
  it("submits name, description, picked lead and assignee type", async () => {
    const onSubmit = renderForm();
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "API");
    await userEvent.type(screen.getByRole("textbox", { name: "Description" }), "REST layer");
    await userEvent.type(screen.getByRole("searchbox", { name: "Lead" }), "al");
    await userEvent.click(await screen.findByRole("button", { name: "Alice Andersen (alice)" }));
    await userEvent.selectOptions(screen.getByRole("combobox", { name: "Default assignee" }), "COMPONENT_LEAD");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(onSubmit).toHaveBeenCalledWith({
      name: "API",
      description: "REST layer",
      leadUserName: "alice",
      assigneeType: "COMPONENT_LEAD",
    });
  });

  it("refuses a blank or taken name without submitting", async () => {
    const onSubmit = renderForm();
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "   ");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(screen.getByText("A component needs a name.")).toBeTruthy();
    await userEvent.clear(screen.getByRole("textbox", { name: "Name" }));
    await userEvent.type(screen.getByRole("textbox", { name: "Name" }), "core");
    await userEvent.click(screen.getByRole("button", { name: "Create" }));
    expect(screen.getByText('A component named "core" already exists.')).toBeTruthy();
    expect(onSubmit).not.toHaveBeenCalled();
  });

  it("edits an existing component and can clear its lead", async () => {
    const onSubmit = renderForm({
      initial: { id: "1", name: "Core", description: "d", leadName: "bob", leadDisplayName: "Bob Brown", assigneeType: "PROJECT_DEFAULT" },
      takenNames: ["API"],
    });
    expect((screen.getByRole("textbox", { name: "Name" }) as HTMLInputElement).value).toBe("Core");
    await userEvent.click(screen.getByRole("button", { name: "Clear lead" }));
    await userEvent.click(screen.getByRole("button", { name: "Save" }));
    expect(onSubmit).toHaveBeenCalledWith({
      name: "Core", description: "d", leadUserName: "", assigneeType: "PROJECT_DEFAULT",
    });
  });
});
