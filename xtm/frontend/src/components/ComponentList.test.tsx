import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import { ComponentList } from "./ComponentList";

function ComponentListHarness() {
  const [selected, setSelected] = useState("");

  return (
    <ComponentList
      components={[{ label: "Payments", count: 3 }]}
      selected={selected}
      emptyLabel="No components"
      onSelect={setSelected}
    />
  );
}

describe("ComponentList", () => {
  it("selects a focused component with Enter and Space", async () => {
    const user = userEvent.setup();
    render(<ComponentListHarness />);

    const allTests = screen.getByRole("button", { name: "All tests" });
    const payments = screen.getByRole("button", { name: /^Payments\s*3$/ });
    expect(allTests).toBeInstanceOf(HTMLButtonElement);
    expect(payments).toBeInstanceOf(HTMLButtonElement);

    for (const key of ["{Enter}", " "]) {
      payments.focus();
      expect(payments).toHaveFocus();

      await user.keyboard(key);

      expect(payments).toHaveAttribute("aria-pressed", "true");
      expect(allTests).toHaveAttribute("aria-pressed", "false");
      await user.click(allTests);
    }
  });
});
