import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { MemberFilters, keepMembers, memberLabels } from "./MemberFilters";
import * as api from "../../api";

vi.mock("../../api", () => ({
  ContainerTestsMatching: vi.fn(),
  errMsg: (e: unknown) => (e instanceof Error ? e.message : String(e)),
}));

const rows = [
  { testKey: "QA-1", labels: ["smoke", "auth"] },
  { testKey: "QA-2", labels: ["auth"] },
  { testKey: "QA-3", labels: [] },
];

beforeEach(() => vi.resetAllMocks());

it("keeps the tests with the label and the JQL matches, and lists the labels the tests carry", () => {
  expect(memberLabels(rows)).toEqual(["auth", "smoke"]);
  expect(keepMembers(rows, "auth", null).map((r) => r.testKey)).toEqual([
    "QA-1",
    "QA-2",
  ]);
  expect(
    keepMembers(rows, "", new Set(["QA-2", "QA-3"])).map((r) => r.testKey),
  ).toEqual(["QA-2", "QA-3"]);
  expect(
    keepMembers(rows, "smoke", new Set(["QA-2"])).map((r) => r.testKey),
  ).toEqual([]);
  expect(keepMembers(rows, "", null)).toHaveLength(3);
});

function show(onLabel = vi.fn(), onJqlKeys = vi.fn()) {
  render(
    <MemberFilters
      profileId="p1"
      containerKey="QA-TE-1"
      labels={["auth", "smoke"]}
      label=""
      onLabel={onLabel}
      jqlOn={false}
      onJqlKeys={onJqlKeys}
    />,
  );
  return { user: userEvent.setup(), onLabel, onJqlKeys };
}

it("picks a label", async () => {
  const { user, onLabel } = show();
  await user.selectOptions(screen.getByLabelText("Label"), "smoke");
  expect(onLabel).toHaveBeenCalledWith("smoke");
});

it("asks Jira which tests match the JQL and keeps those", async () => {
  vi.mocked(api.ContainerTestsMatching).mockResolvedValue(["QA-2"]);
  const { user, onJqlKeys } = show();
  await user.type(screen.getByLabelText("JQL"), "priority = High{Enter}");
  expect(api.ContainerTestsMatching).toHaveBeenCalledWith(
    "p1",
    "QA-TE-1",
    "priority = High",
  );
  expect(onJqlKeys).toHaveBeenLastCalledWith(new Set(["QA-2"]));
});

it("shows Jira's reason when the JQL is refused, and keeps every test", async () => {
  vi.mocked(api.ContainerTestsMatching).mockRejectedValue(
    new Error("Field 'bad' does not exist."),
  );
  const { user, onJqlKeys } = show();
  await user.type(screen.getByLabelText("JQL"), "bad = 1");
  await user.click(screen.getByRole("button", { name: "Apply JQL" }));
  expect(await screen.findByRole("alert")).toHaveTextContent(
    "Field 'bad' does not exist.",
  );
  expect(onJqlKeys).not.toHaveBeenCalledWith(expect.any(Set));
});

it("offers no JQL where the backend or the demo profile has none", () => {
  render(
    <MemberFilters
      profileId="p1"
      containerKey="QA-TE-1"
      labels={["auth"]}
      label=""
      onLabel={vi.fn()}
      jqlOn={false}
      jqlAvailable={false}
      onJqlKeys={vi.fn()}
    />,
  );
  expect(screen.queryByLabelText("JQL")).not.toBeInTheDocument();
  expect(screen.getByLabelText("Label")).toBeVisible();
});

it("clears the JQL filter", async () => {
  const onJqlKeys = vi.fn();
  render(
    <MemberFilters
      profileId="p1"
      containerKey="QA-TE-1"
      labels={[]}
      label=""
      onLabel={vi.fn()}
      jqlOn
      onJqlKeys={onJqlKeys}
    />,
  );
  await userEvent
    .setup()
    .click(screen.getByRole("button", { name: "Clear JQL" }));
  expect(onJqlKeys).toHaveBeenCalledWith(null);
});
