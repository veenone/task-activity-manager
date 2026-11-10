import { expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { StatusBar } from "./StatusBar";

it("shows how far a running query is", () => {
  render(
    <StatusBar
      progress={{ stage: "Fetching issues", done: 40, total: 250 }}
      notice={null}
      onDismiss={() => {}}
      theme="dark"
    />,
  );
  const bar = screen.getByRole("progressbar", { name: "Run progress" });
  expect(bar).toHaveAttribute("aria-valuenow", "40");
  expect(bar).toHaveAttribute("aria-valuemax", "250");
  expect(screen.getByText("Fetching issues: 40 / 250")).toBeVisible();
});

it("shows the profile, the last run and the theme when nothing runs", () => {
  render(
    <StatusBar
      profile="Old Jira"
      summary="12 issues"
      runAt="Last run 2026-11-10 09:30"
      progress={null}
      notice={null}
      onDismiss={() => {}}
      theme="dark"
    />,
  );
  expect(screen.queryByRole("progressbar")).toBeNull();
  expect(screen.getByRole("contentinfo")).toHaveTextContent(
    "Old Jira12 issuesLast run 2026-11-10 09:30Theme: dark",
  );
});
