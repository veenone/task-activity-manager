import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { QueryClientProvider } from "@tanstack/react-query";
import { DialogProvider, createQueryClient } from "@agile-suite/core";
import * as api from "../api";
import type { Attachment } from "../api";
import { fileSize } from "../lib/format";
import { AttachmentsSection } from "./AttachmentsSection";

vi.mock("../api", async () => {
  const actual = await vi.importActual<typeof import("../api")>("../api");
  return {
    ...actual,
    ListAttachments: vi.fn(),
    PickAttachmentFiles: vi.fn(),
    AttachFiles: vi.fn(),
    SaveAttachment: vi.fn(),
    DiscardPendingChange: vi.fn(),
  };
});

const jiras: Attachment = {
  id: "10010", name: "trace.log", author: "ranand", authorName: "R. Anand",
  created: "2026-11-17T09:00:00.000+0700", size: 2048, mimeType: "text/plain", path: "",
};
const mine: Attachment = {
  id: "", name: "shot.png", author: "", authorName: "", created: "", size: 5, mimeType: "",
  path: "C:\\Users\\me\\shot.png", pending: true, pendingId: 77,
};

function renderSection(open = true) {
  return render(
    <QueryClientProvider client={createQueryClient()}>
      <DialogProvider>
        <AttachmentsSection profileId="p1" issueKey="PLAT-412" open={open} />
      </DialogProvider>
    </QueryClientProvider>,
  );
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.ListAttachments).mockResolvedValue([jiras, mine]);
  vi.mocked(api.PickAttachmentFiles).mockResolvedValue(["C:\\Users\\me\\notes.txt"]);
  vi.mocked(api.AttachFiles).mockResolvedValue();
  vi.mocked(api.SaveAttachment).mockResolvedValue("D:\\saved\\trace.log");
  vi.mocked(api.DiscardPendingChange).mockResolvedValue();
});

describe("AttachmentsSection", () => {
  it("reads nothing while the section is closed", async () => {
    renderSection(false);
    await screen.findByRole("button", { name: "Attach files" });
    expect(api.ListAttachments).not.toHaveBeenCalled();
  });

  it("lists the files Jira holds with who added them, and the pending one as pending", async () => {
    renderSection();
    const held = (await screen.findByText("trace.log")).closest("li") as HTMLElement;
    expect(api.ListAttachments).toHaveBeenCalledWith("p1", "PLAT-412");
    expect(within(held).getByText(fileSize(2048))).toBeInTheDocument();
    expect(within(held).getByText(/^R\. Anand, /)).toBeInTheDocument();
    expect(within(held).getByRole("button", { name: "Save trace.log" })).toBeInTheDocument();

    const pending = screen.getByText("shot.png").closest("li") as HTMLElement;
    expect(within(pending).getByText("pending")).toBeInTheDocument();
    // A pending file is not in Jira, so there is nothing to save from there.
    expect(within(pending).queryByRole("button", { name: /^Save/ })).toBeNull();
  });

  it("journals the files the dialog answered against this issue", async () => {
    const user = userEvent.setup();
    renderSection();
    await screen.findByText("trace.log");
    await user.click(screen.getByRole("button", { name: "Attach files" }));
    await waitFor(() => expect(api.AttachFiles).toHaveBeenCalledWith("p1", "PLAT-412", ["C:\\Users\\me\\notes.txt"]));
    // The list is read again so the new file shows as pending.
    await waitFor(() => expect(api.ListAttachments).toHaveBeenCalledTimes(2));
  });

  it("journals nothing when the dialog is cancelled", async () => {
    const user = userEvent.setup();
    vi.mocked(api.PickAttachmentFiles).mockResolvedValue([]);
    renderSection();
    await screen.findByText("trace.log");
    await user.click(screen.getByRole("button", { name: "Attach files" }));
    await waitFor(() => expect(api.PickAttachmentFiles).toHaveBeenCalled());
    expect(api.AttachFiles).not.toHaveBeenCalled();
  });

  it("shows the refusal when a picked file cannot be attached", async () => {
    const user = userEvent.setup();
    vi.mocked(api.AttachFiles).mockRejectedValue(new Error("C:\\Users\\me\\notes.txt cannot be read"));
    renderSection();
    await screen.findByText("trace.log");
    await user.click(screen.getByRole("button", { name: "Attach files" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("notes.txt cannot be read");
  });

  it("saves a file Jira holds by its id and says where it went", async () => {
    const user = userEvent.setup();
    renderSection();
    await user.click(await screen.findByRole("button", { name: "Save trace.log" }));
    await waitFor(() => expect(api.SaveAttachment).toHaveBeenCalledWith("p1", "PLAT-412", "10010", "trace.log"));
    expect(await screen.findByRole("status")).toHaveTextContent("Saved to D:\\saved\\trace.log");
  });

  it("discards a pending file by its journal row", async () => {
    const user = userEvent.setup();
    renderSection();
    await user.click(await screen.findByRole("button", { name: "Discard shot.png" }));
    await waitFor(() => expect(api.DiscardPendingChange).toHaveBeenCalledWith("p1", 77));
  });
});
