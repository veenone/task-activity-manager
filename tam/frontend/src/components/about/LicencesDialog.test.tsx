import { describe, it, expect, vi } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { LicencesDialog } from "./LicencesDialog";
import type { Notices } from "./LicencesDialog";

const notices: Notices = {
  license: "GPL-3.0-only",
  text: "own",
  packages: [
    { name: "github.com/google/uuid", version: "v1.6.0", license: "BSD-3-Clause", kind: "go", text: "bsd" },
    { name: "marked", version: "18.0.13", license: "MIT", kind: "npm", text: "mit" },
    { name: "react", version: "19.2.7", license: "MIT", kind: "npm", text: "" },
  ],
  texts: {
    own: "GNU GENERAL PUBLIC LICENSE Version 3",
    bsd: "Copyright (c) 2009 Google Inc. Redistribution and use",
    mit: "Copyright (c) Christopher Jeffrey. Permission is hereby granted",
  },
};

function packageNames() {
  const table = screen.getByRole("table", { name: /Third-party packages/ });
  return within(table)
    .queryAllByRole("rowheader")
    .map((cell) => cell.textContent);
}

describe("LicencesDialog", () => {
  it("shows TAM's own licence text", async () => {
    const user = userEvent.setup();
    render(<LicencesDialog notices={notices} onClose={vi.fn()} />);
    expect(screen.getByRole("dialog", { name: "Licences" })).toBeInTheDocument();
    await user.click(screen.getByText(/Task Activity Manager is licensed under the GNU GPL v3/));
    expect(screen.getByText("GNU GENERAL PUBLIC LICENSE Version 3")).toBeVisible();
  });

  it("lists every package with its version, licence and kind", () => {
    render(<LicencesDialog notices={notices} onClose={vi.fn()} />);
    expect(packageNames()).toEqual(["github.com/google/uuid", "marked", "react"]);
    const row = screen.getByRole("rowheader", { name: "github.com/google/uuid" }).closest("tr")!;
    expect(within(row).getAllByRole("cell").map((c) => c.textContent)).toEqual(["v1.6.0", "BSD-3-Clause", "Go"]);
  });

  it("narrows the list by name or licence", async () => {
    const user = userEvent.setup();
    render(<LicencesDialog notices={notices} onClose={vi.fn()} />);
    const filter = screen.getByRole("searchbox", { name: "Filter packages" });
    await user.type(filter, "mit");
    expect(packageNames()).toEqual(["marked", "react"]);
    await user.clear(filter);
    await user.type(filter, "UUID");
    expect(packageNames()).toEqual(["github.com/google/uuid"]);
    await user.clear(filter);
    await user.type(filter, "nothing like this");
    expect(packageNames()).toEqual([]);
    expect(screen.getByText("No package matches the filter.")).toBeInTheDocument();
  });

  it("expands a row to show that package's licence text", async () => {
    const user = userEvent.setup();
    render(<LicencesDialog notices={notices} onClose={vi.fn()} />);
    const toggle = screen.getByRole("button", { name: "marked" });
    expect(toggle).toHaveAttribute("aria-expanded", "false");
    expect(screen.queryByText(/Christopher Jeffrey/)).not.toBeInTheDocument();
    await user.click(toggle);
    expect(toggle).toHaveAttribute("aria-expanded", "true");
    expect(screen.getByText(/Christopher Jeffrey/)).toBeInTheDocument();
    await user.click(toggle);
    expect(screen.queryByText(/Christopher Jeffrey/)).not.toBeInTheDocument();
  });

  it("says so when a package shipped no licence file", async () => {
    const user = userEvent.setup();
    render(<LicencesDialog notices={notices} onClose={vi.fn()} />);
    await user.click(screen.getByRole("button", { name: "react" }));
    expect(screen.getByText("This package ships no licence file. Its declared licence is MIT.")).toBeInTheDocument();
  });
});
