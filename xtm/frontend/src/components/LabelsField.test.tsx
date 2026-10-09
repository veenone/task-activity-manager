import { describe, it, expect, vi } from "vitest";
import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { LabelsField } from "./LabelsField";

vi.mock("../api", () => ({
  ListLabels: vi.fn(async () => [
    { label: "smoke", count: 2 },
    { label: "login", count: 1 },
  ]),
}));

function Harness({
  readOnly = false,
  onSave = vi.fn(),
}: {
  readOnly?: boolean;
  onSave?: (v: string) => void;
}) {
  const [value, setValue] = useState("smoke");
  return (
    <LabelsField
      profileId="p1"
      value={value}
      onChange={setValue}
      onSave={onSave}
      readOnly={readOnly}
    />
  );
}

function renderField(props: Parameters<typeof Harness>[0] = {}) {
  const client = new QueryClient({ defaultOptions: { queries: { retry: false } } });
  return render(
    <QueryClientProvider client={client}>
      <Harness {...props} />
      <button>elsewhere</button>
    </QueryClientProvider>,
  );
}

describe("LabelsField", () => {
  it("adds a suggested label and saves the joined value on blur", async () => {
    const onSave = vi.fn();
    renderField({ onSave });
    const picker = screen.getByRole("combobox", { name: "Labels" });
    await userEvent.type(picker, "log");
    await userEvent.click(await screen.findByRole("option", { name: "login" }));
    await userEvent.click(screen.getByRole("button", { name: "elsewhere" }));
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith("smoke login");
  });

  it("saves the remaining labels when a chip is removed with its button", async () => {
    const onSave = vi.fn();
    renderField({ onSave });
    await userEvent.click(screen.getByRole("button", { name: "Remove smoke" }));
    expect(onSave).toHaveBeenCalledTimes(1);
    expect(onSave).toHaveBeenCalledWith("");
  });

  it("saves typed text that was never confirmed when focus leaves", async () => {
    const onSave = vi.fn();
    renderField({ onSave });
    await userEvent.type(screen.getByRole("combobox", { name: "Labels" }), "draft");
    await userEvent.click(screen.getByRole("button", { name: "elsewhere" }));
    expect(onSave).toHaveBeenLastCalledWith("smoke draft");
  });

  it("read-only shows the labels as text with no picker", () => {
    renderField({ readOnly: true });
    expect(screen.getByText("smoke").tagName).toBe("SPAN");
    expect(screen.queryByRole("combobox", { name: "Labels" })).toBeNull();
  });
});
