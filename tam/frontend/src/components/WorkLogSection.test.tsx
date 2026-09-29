import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Worklog } from "../api";
import { calendarDay } from "../lib/format";
import { WorkLogSection, worklogEntryLine } from "./WorkLogSection";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return { ...actual, ListWorklogs: vi.fn(), LogWork: vi.fn(), CheckWorkDuration: vi.fn(), DiscardPendingChange: vi.fn() };
});

// jiras is what the instance already holds: an hour logged by someone else,
// which is the ordinary case and the reason the section is not limited to the
// reader's own issues.
const jiras: Worklog = {
  id: "10001", author: "ranand", authorName: "R. Anand",
  started: "2026-09-28T09:00:00.000+0700", timeSpent: "1h", seconds: 3600, comment: "Reproduced the bug",
};

const mine: Worklog = {
  id: "", author: "", authorName: "",
  started: "2026-09-29T01:00:00.000+0700", timeSpent: "2h 30m", seconds: 9000, comment: "Pairing",
  pending: true, pendingId: 77,
};

function renderSection(open = true) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <WorkLogSection profileId="p1" issueKey="PLAT-412" open={open} />
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListWorklogs).mockResolvedValue([jiras, mine]);
  vi.mocked(api.LogWork).mockResolvedValue();
  vi.mocked(api.CheckWorkDuration).mockResolvedValue();
  vi.mocked(api.DiscardPendingChange).mockResolvedValue();
});

describe("WorkLogSection", () => {
  it("reads nothing while the section is closed", async () => {
    renderSection(false);
    await screen.findByRole("button", { name: "Log work" });
    expect(api.ListWorklogs).not.toHaveBeenCalled();
  });

  it("shows Jira's entries, the pending one, and the total of both", async () => {
    renderSection();
    expect(await screen.findByText("3h 30m logged over 2 entries")).toBeInTheDocument();
    expect(api.ListWorklogs).toHaveBeenCalledWith("p1", "PLAT-412");
    expect(screen.getByText("1h")).toBeInTheDocument();
    expect(screen.getByText("R. Anand")).toBeInTheDocument();
    expect(screen.getByText("Reproduced the bug")).toBeInTheDocument();
    // The pending entry is marked as such and offers its own Discard.
    expect(screen.getByText("2h 30m")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: "Discard the 2h 30m entry" })).toBeInTheDocument();
    // The day comes off the stamp Jira dated the entry by, 29 Sep at +0700,
    // and not from whatever the reader's own zone makes of that instant.
    expect(screen.getByText(calendarDay("2026-09-29"))).toBeInTheDocument();
    expect(screen.getByText(calendarDay("2026-09-28"))).toBeInTheDocument();
  });

  it("says so when the issue has no work logged", async () => {
    vi.mocked(api.ListWorklogs).mockResolvedValue([]);
    renderSection();
    expect(await screen.findByText("No work logged.")).toBeInTheDocument();
  });

  it("journals an entry and clears the form", async () => {
    const user = userEvent.setup();
    renderSection();
    await screen.findByText("3h 30m logged over 2 entries");
    await user.type(screen.getByLabelText("Time spent"), "45m");
    await user.type(screen.getByLabelText("Comment"), "Rebasing");
    const add = screen.getByRole("button", { name: "Log work" });
    await waitFor(() => expect(add).toBeEnabled());
    await user.click(add);
    await waitFor(() => expect(api.LogWork).toHaveBeenCalledWith("p1", "PLAT-412", "45m", "Rebasing"));
    expect(screen.getByLabelText("Time spent")).toHaveValue("");
    expect(screen.getByLabelText("Comment")).toHaveValue("");
  });

  it("refuses a duration Jira would refuse, where it was typed, and journals nothing", async () => {
    const user = userEvent.setup();
    vi.mocked(api.CheckWorkDuration).mockRejectedValue(
      new Error(`"2 hrs" is not a duration Jira understands. Use w, d, h or m, for example 2h 30m, 90m or 1d`),
    );
    renderSection();
    await screen.findByText("3h 30m logged over 2 entries");
    await user.type(screen.getByLabelText("Time spent"), "2 hrs");
    await user.click(screen.getByRole("button", { name: "Log work" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("is not a duration Jira understands");
    expect(api.LogWork).not.toHaveBeenCalled();
    // The typed value is kept, so a typo is corrected rather than retyped.
    expect(screen.getByLabelText("Time spent")).toHaveValue("2 hrs");
  });

  it("discards a pending entry by its journal row", async () => {
    const user = userEvent.setup();
    renderSection();
    await screen.findByText("3h 30m logged over 2 entries");
    await user.click(screen.getByRole("button", { name: "Discard the 2h 30m entry" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 77));
  });

  it("offers a retry when the read fails", async () => {
    vi.mocked(api.ListWorklogs).mockRejectedValueOnce(new Error("GET failed: 403"));
    renderSection();
    expect(await screen.findByTestId("worklog-error")).toHaveTextContent("403");
  });
});

describe("worklogEntryLine", () => {
  it("reads an entry as its duration, its day and its comment", () => {
    const when = calendarDay("2026-09-29");
    expect(worklogEntryLine(mine)).toBe(`2h 30m on ${when}, Pairing`);
    expect(worklogEntryLine({ ...mine, comment: "" })).toBe(`2h 30m on ${when}`);
    expect(worklogEntryLine({ ...mine, started: "", comment: "" })).toBe("2h 30m");
  });
});
