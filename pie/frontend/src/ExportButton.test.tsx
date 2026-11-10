import { useState } from "react";
import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ExportButton } from "./ExportButton";
import { type Notice, NoticeContext, StatusBar } from "./StatusBar";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({ ExportWorkbook: vi.fn() }));

const sheet = { name: "Links", header: ["From"], rows: [["A"]] };

beforeEach(() => vi.resetAllMocks());

// The button reports to the status bar, as App wires them.
function Wired({ sheets }: { sheets: () => (typeof sheet)[] }) {
  const [notice, setNotice] = useState<Notice | null>(null);
  return (
    <NoticeContext.Provider value={setNotice}>
      <div className="pie-filters">
        <ExportButton name="pie-links" sheets={sheets} />
      </div>
      <StatusBar notice={notice} onDismiss={() => setNotice(null)} progress={null} theme="light" />
    </NoticeContext.Provider>
  );
}

it("saves the sheets the view builds on click, and names the file in the status bar", async () => {
  vi.mocked(api.ExportWorkbook).mockResolvedValue("/home/analyst/links.xlsx");
  const sheets = vi.fn(() => [sheet]);
  const user = userEvent.setup();
  const { container } = render(<Wired sheets={sheets} />);
  expect(sheets).not.toHaveBeenCalled();
  await user.click(screen.getByRole("button", { name: "Export to Excel" }));
  expect(api.ExportWorkbook).toHaveBeenCalledWith("pie-links", [sheet]);
  const footer = container.querySelector("footer")!;
  expect(footer).toHaveTextContent("Saved to /home/analyst/links.xlsx");
  // Nothing is written beside the button any more.
  expect(container.querySelector(".pie-filters")).toHaveTextContent(/^Export to Excel$/);
  await user.click(screen.getByRole("button", { name: "Dismiss" }));
  expect(footer).not.toHaveTextContent("Saved to");
});

it("says nothing when the dialog is cancelled, and why when the write fails", async () => {
  vi.mocked(api.ExportWorkbook).mockResolvedValueOnce("").mockRejectedValueOnce("write links.xlsx: access denied");
  const user = userEvent.setup();
  const { container } = render(<Wired sheets={() => [sheet]} />);
  await user.click(screen.getByRole("button", { name: "Export to Excel" }));
  expect(container.querySelector("footer")).not.toHaveTextContent("Saved to");
  await user.click(screen.getByRole("button", { name: "Export to Excel" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("Export failed. write links.xlsx: access denied");
  expect(container.querySelector("footer")).toHaveClass("statusbar-failed");
});
