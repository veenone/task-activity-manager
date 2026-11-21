import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import { QueryClientProvider } from "@tanstack/react-query";
import { createQueryClient } from "@agile-suite/core";
import { ConflictCard } from "./ConflictCard";

vi.mock("../contexts/SyncContext", () => ({ useSync: () => ({ dismissConflict: vi.fn() }) }));

describe("ConflictCard (#537)", () => {
  it("marks a field Jira changed in words, not colour alone", () => {
    render(
      <QueryClientProvider client={createQueryClient()}>
        <ConflictCard
          profileId="p1"
          disabled={false}
          conflict={{
            key: "PLAT-9", summary: "Retry", remoteVersion: "v2",
            fields: [
              { field: "priority", base: "Medium", mine: "High", remote: "Low" },
              { field: "assignee", base: "Ana", mine: "Bo", remote: "Ana" },
            ],
          }}
        />
      </QueryClientProvider>,
    );
    const rows = within(screen.getByRole("table", { name: "Conflicting fields" })).getAllByRole("row").slice(1);
    const remote = (row: HTMLElement) => within(row).getAllByRole("cell")[3].textContent;
    expect(remote(rows[0])).toBe("Low (changed)");
    expect(remote(rows[1])).toBe("Ana");
  });
});
