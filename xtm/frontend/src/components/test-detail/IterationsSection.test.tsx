import { render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";

import { IterationsSection } from "./IterationsSection";
import type { RunIteration } from "../../api";

const getRunIterations = vi.fn();
vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, GetRunIterations: (...args: unknown[]) => getRunIterations(...args) };
});

function iterations(): RunIteration[] {
  return [
    {
      id: 11868,
      status: "PASS",
      parameters: [
        { name: "TIER", value: "gold" },
        { name: "REGION", value: "eu" },
      ],
    },
    {
      id: 11869,
      status: "FAIL",
      parameters: [
        { name: "TIER", value: "silver" },
        { name: "REGION", value: "us" },
      ],
    },
    {
      id: 11870,
      status: "TODO",
      parameters: [
        { name: "TIER", value: "bronze" },
        { name: "REGION", value: "ap" },
      ],
    },
  ];
}

beforeEach(() => {
  getRunIterations.mockReset();
  getRunIterations.mockResolvedValue(iterations());
});

describe("IterationsSection", () => {
  // A parameterized test runs once per dataset row, each with its own
  // result. One status for the whole run hides which row failed (#267).
  it("shows a row per iteration, with the values it ran with and its result", async () => {
    render(<IterationsSection profileId="p1" testKey="QA-1" containerKey="EXEC-1" />);

    expect(await screen.findByText("silver")).toBeInTheDocument();
    expect(screen.getByText("ap")).toBeInTheDocument();
    // The parameter names head the columns, so a value means something.
    expect(screen.getByRole("columnheader", { name: "TIER" })).toBeInTheDocument();
    expect(screen.getByRole("columnheader", { name: "REGION" })).toBeInTheDocument();
    expect(screen.getByText("FAIL")).toBeInTheDocument();
  });

  it("counts what passed, which is the question being asked", async () => {
    render(<IterationsSection profileId="p1" testKey="QA-1" containerKey="EXEC-1" />);

    expect(await screen.findByText(/1 of 3 iterations passed/)).toBeInTheDocument();
  });

  it("asks for nothing and shows nothing without an execution", () => {
    const { container } = render(<IterationsSection profileId="p1" testKey="QA-1" containerKey="" />);

    expect(getRunIterations).not.toHaveBeenCalled();
    expect(container).toBeEmptyDOMElement();
  });

  it("shows nothing for a run that is not parameterized", async () => {
    getRunIterations.mockResolvedValue([]);
    const { container } = render(
      <IterationsSection profileId="p1" testKey="QA-1" containerKey="EXEC-1" />,
    );

    await vi.waitFor(() => expect(getRunIterations).toHaveBeenCalled());
    expect(container).toBeEmptyDOMElement();
  });

  it("surfaces a read failure rather than showing an empty run", async () => {
    getRunIterations.mockRejectedValue(new Error("jira unreachable"));
    render(<IterationsSection profileId="p1" testKey="QA-1" containerKey="EXEC-1" />);

    expect(await screen.findByText(/jira unreachable/)).toBeInTheDocument();
  });
});
