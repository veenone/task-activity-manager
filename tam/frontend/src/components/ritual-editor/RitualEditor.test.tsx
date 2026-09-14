import { createRef } from "react";
import { beforeAll, beforeEach, describe, expect, it, vi } from "vitest";
import { act, fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import * as api from "../../api";
import { READ_ONLY_SENTENCE, ENTRY_EXISTS } from "../../lib/ritualText";
import { RitualEditor } from "./RitualEditor";
import type { RitualEditorHandle } from "./RitualEditor";

vi.mock("../../api", async () => {
  const actual = await vi.importActual<typeof import("../../api")>("../../api");
  return { ...actual, SaveRitualBody: vi.fn(), StandupEntry: vi.fn(), RitualMacroIssues: vi.fn(), BrowserOpenURL: vi.fn() };
});

// jsdom has no layout; ProseMirror asks for rectangles when it scrolls a
// selection into view.
beforeAll(() => {
  Range.prototype.getBoundingClientRect = () => new DOMRect();
  Range.prototype.getClientRects = () => ({ length: 0, item: () => null, [Symbol.iterator]: [][Symbol.iterator] }) as unknown as DOMRectList;
  document.elementFromPoint = () => null;
});

function doc(over: Partial<api.RitualDocument> = {}): api.RitualDocument {
  return {
    profileId: "p1", boardId: 1, sprintId: 14, ritualType: "planning", title: "Sprint 14 · Planning",
    body: "<h2>Decisions</h2><ul><li>ship it</li></ul>", baseBody: "", pageId: "", version: 0,
    conflictBody: "", conflictVersion: 0, status: "local", updatedAt: "2026-09-14T09:00:00Z", syncedAt: "", ...over,
  };
}

beforeEach(() => {
  vi.clearAllMocks();
  vi.mocked(api.SaveRitualBody).mockImplementation(async (_p, _b, _s, _t, body) => doc({ body, status: "local" }));
});

const pause = (ms: number) => act(() => new Promise((r) => setTimeout(r, ms)));

describe("RitualEditor", () => {
  it("never saves a page just because it was opened", async () => {
    render(<RitualEditor profileId="p1" doc={doc()} saveDelayMs={10} />);
    expect(await screen.findByText("ship it")).toBeInTheDocument();
    await pause(50);
    expect(api.SaveRitualBody).not.toHaveBeenCalled();
  });

  it("saves once after an edit pauses, and hands the saved row back", async () => {
    const ref = createRef<RitualEditorHandle>();
    const onSaved = vi.fn();
    const { container } = render(<RitualEditor ref={ref} profileId="p1" doc={doc()} onSaved={onSaved} saveDelayMs={10} />);
    await screen.findByText("ship it");
    fireEvent.keyDown(container.querySelector(".ProseMirror")!, { key: "a" });
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>hello</p>"); });
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalledTimes(1));
    expect(vi.mocked(api.SaveRitualBody).mock.calls[0]).toEqual(["p1", 1, 14, "planning", "<h2>Decisions</h2><ul><li>ship it</li></ul><p>hello</p>"]);
    await waitFor(() => expect(onSaved).toHaveBeenCalled());
  });

  it("flushes a pending edit immediately when asked", async () => {
    const ref = createRef<RitualEditorHandle>();
    const { container } = render(<RitualEditor ref={ref} profileId="p1" doc={doc()} saveDelayMs={60_000} />);
    await screen.findByText("ship it");
    fireEvent.keyDown(container.querySelector(".ProseMirror")!, { key: "a" });
    act(() => { ref.current!.editor!.commands.insertContentAt(ref.current!.editor!.state.doc.content.size, "<p>now</p>"); });
    await act(() => ref.current!.flush());
    expect(api.SaveRitualBody).toHaveBeenCalledTimes(1);
  });

  it("opens a page it cannot read as read only, with the page link", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: "<p>&bogus;</p>" })} pageUrl="https://c.example.com/pages/viewpage.action?pageId=42" />);
    expect(screen.getByText(READ_ONLY_SENTENCE)).toBeInTheDocument();
    await userEvent.click(screen.getByRole("button", { name: "Open in Confluence" }));
    expect(api.BrowserOpenURL).toHaveBeenCalledWith("https://c.example.com/pages/viewpage.action?pageId=42");
    expect(document.querySelector(".ProseMirror")).toBeNull();
  });

  it("draws content it does not model as a locked block", async () => {
    render(<RitualEditor profileId="p1" doc={doc({ body: '<ac:structured-macro ac:name="panel"><ac:rich-text-body><p>x</p></ac:rich-text-body></ac:structured-macro>' })} />);
    expect(await screen.findByText("Confluence: panel")).toBeInTheDocument();
  });

  it("adds today's standup entry under the log once, and refuses a second", async () => {
    vi.mocked(api.StandupEntry).mockResolvedValue("<h3>Tue 15 Sep 2026</h3><p><strong>Yesterday</strong></p><ul><li></li></ul>");
    const ref = createRef<RitualEditorHandle>();
    render(<RitualEditor ref={ref} profileId="p1" saveDelayMs={10}
      doc={doc({ ritualType: "standup", body: "<h2>Daily log</h2><h3>Mon 14 Sep 2026</h3><p>y</p>" })} />);
    await screen.findByText("Mon 14 Sep 2026");
    await userEvent.click(screen.getByRole("button", { name: "Add today's entry" }));
    await waitFor(() => expect(ref.current!.editor!.state.doc.child(1).textContent).toBe("Tue 15 Sep 2026"));
    await waitFor(() => expect(api.SaveRitualBody).toHaveBeenCalled());
    await userEvent.click(screen.getByRole("button", { name: "Add today's entry" }));
    expect(await screen.findByText(ENTRY_EXISTS)).toBeInTheDocument();
  });

  it("offers Add today's entry only on the standup", async () => {
    render(<RitualEditor profileId="p1" doc={doc()} />);
    await screen.findByText("ship it");
    expect(screen.queryByRole("button", { name: "Add today's entry" })).toBeNull();
  });
});
