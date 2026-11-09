import { beforeEach, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ExportButton } from "./ExportButton";
import * as api from "../wailsjs/go/main/App";

vi.mock("../wailsjs/go/main/App", () => ({ ExportWorkbook: vi.fn() }));

const sheet = { name: "Links", header: ["From"], rows: [["A"]] };

beforeEach(() => vi.resetAllMocks());

it("saves the sheets the view builds on click, and names the file", async () => {
  vi.mocked(api.ExportWorkbook).mockResolvedValue("/home/analyst/links.xlsx");
  const sheets = vi.fn(() => [sheet]);
  render(<ExportButton name="pie-links" sheets={sheets} />);
  expect(sheets).not.toHaveBeenCalled();
  await userEvent.setup().click(screen.getByRole("button", { name: "Export to Excel" }));
  expect(api.ExportWorkbook).toHaveBeenCalledWith("pie-links", [sheet]);
  expect(await screen.findByRole("status")).toHaveTextContent("Saved to /home/analyst/links.xlsx");
});

it("says nothing when the dialog is cancelled, and why when the write fails", async () => {
  vi.mocked(api.ExportWorkbook).mockResolvedValueOnce("").mockRejectedValueOnce("write links.xlsx: access denied");
  const user = userEvent.setup();
  render(<ExportButton name="pie-links" sheets={() => [sheet]} />);
  await user.click(screen.getByRole("button", { name: "Export to Excel" }));
  expect(screen.getByRole("status")).toBeEmptyDOMElement();
  await user.click(screen.getByRole("button", { name: "Export to Excel" }));
  expect(await screen.findByRole("alert")).toHaveTextContent("access denied");
});
