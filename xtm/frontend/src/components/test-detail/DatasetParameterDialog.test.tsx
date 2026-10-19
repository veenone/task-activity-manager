import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";

import type { DatasetParameter } from "../../api";
import { DatasetParameterDialog } from "./DatasetParameterDialog";

const confirm = vi.fn();
vi.mock("../useConfirm", () => ({ useConfirm: () => ({ confirm }) }));

function dialog(
  parameter: DatasetParameter = {
    name: "TIER",
    combinatorial: false,
    type: "LIST",
    options: ["gold", "silver", "bronze"],
    listScope: "PARAMETER_AD_HOC",
  },
  usedValues = ["gold", "silver", "gold"],
) {
  const onSave = vi.fn();
  const onCancel = vi.fn();
  render(
    <DatasetParameterDialog
      parameter={parameter}
      usedValues={usedValues}
      onSave={onSave}
      onCancel={onCancel}
    />,
  );
  return { onSave, onCancel };
}

describe("DatasetParameterDialog", () => {
  beforeEach(() => {
    confirm.mockReset();
    confirm.mockResolvedValue(true);
  });

  it("saves ordered unique options for an ad-hoc List", async () => {
    const { onSave } = dialog({ name: "", combinatorial: false, type: "TEXT" }, []);
    const user = userEvent.setup();

    await user.type(screen.getByLabelText("Parameter name"), "TIER");
    await user.selectOptions(screen.getByLabelText("Parameter type"), "LIST");
    await user.type(screen.getByLabelText("List options"), "gold\nsilver\ngold\n bronze ");
    await user.click(screen.getByRole("button", { name: "Save parameter" }));

    expect(onSave).toHaveBeenCalledWith({
      name: "TIER",
      combinatorial: false,
      type: "LIST",
      options: ["gold", "silver", "bronze"],
      listScope: "PARAMETER_AD_HOC",
    });
  });

  it("lists every affected row before allowing a used option to be removed", async () => {
    const { onSave } = dialog();
    const user = userEvent.setup();

    await user.clear(screen.getByLabelText("List options"));
    await user.type(screen.getByLabelText("List options"), "silver\nbronze");
    await user.click(screen.getByRole("button", { name: "Save parameter" }));

    expect(screen.getByText(/gold.*rows 1 and 3/i)).toBeVisible();
    expect(onSave).not.toHaveBeenCalled();
  });

  it("cancels without returning a changed parameter", async () => {
    const { onSave, onCancel } = dialog();
    const user = userEvent.setup();

    await user.clear(screen.getByLabelText("Parameter name"));
    await user.type(screen.getByLabelText("Parameter name"), "LEVEL");
    await user.click(screen.getByRole("button", { name: "Cancel" }));

    expect(onCancel).toHaveBeenCalledTimes(1);
    expect(onSave).not.toHaveBeenCalled();
  });

  it("shows a predefined binding read-only and confirms conversion", async () => {
    dialog({
      name: "ENVIRONMENT",
      combinatorial: false,
      type: "LIST",
      options: ["staging", "production"],
      listScope: "PROJECT",
      listId: 42,
    });
    const user = userEvent.setup();

    expect(screen.getByText("Project list 42")).toBeVisible();
    expect(screen.getByLabelText("List options")).toBeDisabled();
    await user.click(screen.getByRole("button", { name: "Convert to ad-hoc list" }));

    expect(confirm).toHaveBeenCalledTimes(1);
    expect(screen.getByLabelText("List options")).toBeEnabled();
  });
});
