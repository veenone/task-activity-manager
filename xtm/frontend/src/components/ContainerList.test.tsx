import { useState } from "react";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it } from "vitest";
import type { Container } from "../api";
import { ContainerList } from "./ContainerList";

const testSet: Container = {
  key: "XTM-42",
  kind: "testset",
  summary: "Checkout tests",
  status: "Open",
  parentKey: "",
  parentSummary: "",
  issueType: "Test Set",
  labels: [],
  environments: [],
  fixVersions: [],
  description: "",
};

function ContainerListHarness() {
  const [selected, setSelected] = useState("");

  return (
    <ContainerList
      containers={[testSet]}
      selected={selected}
      emptyLabel="No test sets"
      onSelect={setSelected}
    />
  );
}

describe("ContainerList", () => {
  it("selects a focused container with Enter and Space", async () => {
    const user = userEvent.setup();
    render(<ContainerListHarness />);

    const allTests = screen.getByRole("button", { name: "All tests" });
    const checkoutTests = screen.getByRole("button", {
      name: "XTM-42 Checkout tests",
    });
    expect(allTests).toBeInstanceOf(HTMLButtonElement);
    expect(checkoutTests).toBeInstanceOf(HTMLButtonElement);

    for (const key of ["{Enter}", " "]) {
      checkoutTests.focus();
      expect(checkoutTests).toHaveFocus();

      await user.keyboard(key);

      expect(checkoutTests).toHaveAttribute("aria-pressed", "true");
      expect(allTests).toHaveAttribute("aria-pressed", "false");
      await user.click(allTests);
    }
  });
});
